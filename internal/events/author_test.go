package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withJournal writes these records as THE journal for this test, so the read path under
// test is the real one: the file is parsed, not a slice handed in.
func withJournal(t *testing.T, recs []Record) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(Path())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, r := range recs {
		b, _ := json.Marshal(r)
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
	}
}

// Two prompt submissions on the same pane are field-for-field identical whether the
// commander typed the words or the supervisor delivered them. Only the audit trail tells
// them apart, and the supervisor's pull view hides the audit trail — so the difference was
// invisible without `--all`, and the failure direction is the dangerous one: taking your
// own words for the commander's is self-authorization (issue #1156).

func prompt(seq int64, pane, head string) Record {
	return Record{Seq: seq, Event: "UserPromptSubmit", Pane: pane, Origin: OriginInstruction, Summary: head}
}

func sent(seq int64, pane, actor, state, payload string) Record {
	return Record{Seq: seq, Event: AuditEventSend, Pane: pane, Actor: actor, Summary: state + ": " + payload}
}

func TestOnlyADeliveredPromptGetsAnAuthor(t *testing.T) {
	typed := "先停一下，版本对不上，查清楚再做别的。"
	relayed := "我是 HQ。一件小事想请你确认一下，不着急。"
	got := AuthorOf([]Record{
		prompt(40844, "%19", typed), // the commander, at the keyboard
		prompt(40862, "%19", relayed),
		sent(40863, "%19", "hq", "landed", relayed), // what makes the one above HQ's
	})
	if got[40862] != "hq" {
		t.Errorf("a relayed prompt was not attributed: %q", got[40862])
	}
	if who, ok := got[40844]; ok {
		t.Errorf("the commander's own line was credited to %q", who)
	}
}

// A refusal never reached the pane, so its words cannot be the prompt that arrived, and
// crediting it would put another party's name on the commander's own line.
func TestARefusedDeliveryClaimsNothing(t *testing.T) {
	line := "这句没进去，草稿里有别人的字"
	got := AuthorOf([]Record{
		prompt(2, "%19", line),
		sent(3, "%19", "hq", "refused-draft", line),
		sent(4, "%19", "hq", "failed", line),
	})
	if who, ok := got[2]; ok {
		t.Errorf("a refused delivery claimed a prompt as %q", who)
	}
}

func TestADeliveryIntoAnotherPaneIsAnotherConversation(t *testing.T) {
	line := "把这个任务接着做完，做完直接合，不用问我"
	got := AuthorOf([]Record{
		prompt(2, "%19", line),
		sent(3, "%20", "hq", "landed", line),
	})
	if len(got) != 0 {
		t.Errorf("a send into %%20 attributed a prompt on %%19: %v", got)
	}
}

// A record written before senders were journalled carries no actor, and an unattributable
// prompt reads as the commander's — the answer that is right when we cannot tell.
func TestAnOlderAuditRecordAttributesNothing(t *testing.T) {
	line := "这是老记录对应的那句话"
	got := AuthorOf([]Record{prompt(2, "%19", line), sent(3, "%19", "", "landed", line)})
	if len(got) != 0 {
		t.Errorf("an actor-less record attributed something: %v", got)
	}
}

// The two sides each hold a bounded head and the harness can append its own blocks, so a
// recorded head the prompt BEGINS WITH is a match.
func TestALongerPromptIsMatchedByItsHead(t *testing.T) {
	head := "把这条改成红档，然后把剩下的都跑一遍，跑完告诉我结果"
	got := AuthorOf([]Record{
		prompt(2, "%19", head+"\n\n<system-reminder>…</system-reminder>"),
		sent(3, "%19", "agent:%31", "landed", head),
	})
	if got[2] != "agent:%31" {
		t.Errorf("a prompt with a harness block appended lost its author: %q", got[2])
	}
}

// A short delivery must not claim a longer prompt that merely starts the same way: a
// wrong name on the commander's own words is the whole defect.
func TestAShortDeliveryDoesNotClaimALongerPrompt(t *testing.T) {
	got := AuthorOf([]Record{
		prompt(2, "%19", "继续"),
		prompt(3, "%19", "继续把剩下的十几个 PR 都合掉，然后发版"),
		sent(4, "%19", "hq", "landed", "继续"),
	})
	if got[2] != "hq" {
		t.Errorf("the exact short delivery lost its author: %q", got[2])
	}
	if who, ok := got[3]; ok {
		t.Errorf("a short delivery claimed a longer prompt as %q", who)
	}
}

func TestTheMoreSpecificDeliveryWins(t *testing.T) {
	short := "把这个任务接着做完"
	long := short + "，做完直接合，不用问我"
	got := AuthorOf([]Record{
		prompt(2, "%19", long),
		sent(3, "%19", "hq", "landed", short),
		sent(4, "%19", "agent:%31", "landed", long),
	})
	if got[2] != "agent:%31" {
		t.Errorf("the shorter head claimed it: %q", got[2])
	}
}

func TestNothingToJoinAgainst(t *testing.T) {
	if got := AuthorOf(nil); got != nil {
		t.Errorf("an empty read produced %v", got)
	}
	if got := AuthorOf([]Record{prompt(2, "%19", "一句没人投递过的话")}); got != nil {
		t.Errorf("a lone prompt produced %v", got)
	}
}

// Whitespace is folded on both sides: the two records normalize their own text
// differently, and a re-wrapped line is the same instruction.
func TestWhitespaceDoesNotBreakTheJoin(t *testing.T) {
	got := AuthorOf([]Record{
		prompt(2, "%19", "先停一下，\n版本对不上，\n查清楚再做别的。"),
		sent(3, "%19", "hq", "landed", "先停一下，  版本对不上， 查清楚再做别的。"),
	})
	if got[2] != "hq" {
		t.Errorf("a re-wrapped prompt lost its author: %q", got[2])
	}
}

// The two records are truncated by different writers to different budgets, and measured on
// the real journal it is the PROMPT's head that is shorter: the hook keeps less of it than
// the audit trail keeps of the payload. A prefix test in one direction only attributed
// nothing at all, silently (caught on seq 55834/55835, 2026-09-21).
func TestThePromptsOwnHeadIsTheShorterOne(t *testing.T) {
	short := "动手吧。这条是我(中控)替司令答的,依据是他刚才那句陈述句:「修完后看看 iss"
	long := short + "ue #1156 是否合理,需要的话我们修复。」他预先授权了「需"
	got := AuthorOf([]Record{prompt(55834, "%18", short), sent(55835, "%18", "hq", "landed", long)})
	if got[55834] != "hq" {
		t.Errorf("the prompt's shorter head lost its author: %q", got[55834])
	}
}

// Agreeing on an opening is not agreeing. Two lines that merely start alike are not the
// same line, and a partial match is what would put the wrong name on the commander's words.
func TestAPartialAgreementIsNotAMatch(t *testing.T) {
	got := AuthorOf([]Record{
		prompt(2, "%19", "把这条改成红档，然后把剩下的都跑一遍"),
		sent(3, "%19", "hq", "landed", "把这条改成红档，但是先别合，等我看过再说"),
	})
	if who, ok := got[2]; ok {
		t.Errorf("two lines that only start alike were joined as %q", who)
	}
}

// "Idle" is not "done" (issue #1160). The reap sweep reads the dispatch ledger, the ledger
// only holds what the supervisor dispatched, and a session the commander drives directly
// never enters it — so a flagship he uses every day still shows the one old task it was
// spawned for, marked done, and half an hour of quiet makes it a reap candidate.
//
// These pin the rule that stops it: suggest only on POSITIVE evidence that every prompt
// since the dispatch came from gtmux, and answer no whenever that cannot be shown.

// before is what a real journal holds from before any dispatch it is asked about: the
// evidence reaches back past the moment in question.
var before = Record{Seq: 1, Ts: 500, Event: "SessionStart", Pane: "%1"}

func drove(t *testing.T, recs []Record, pane string, since int64) Driver {
	t.Helper()
	withJournal(t, recs)
	return WhoDroveSince(Read(0, 9000), pane, since)
}

func TestAPureWorkerIsRecognisedAsOne(t *testing.T) {
	got := drove(t, []Record{
		before,
		{Seq: 2, Ts: 1000, Event: "UserPromptSubmit", Pane: "%31", Summary: "把这个任务做完"},
		{Seq: 3, Ts: 1000, Event: AuditEventSend, Pane: "%31", Actor: "hq", Summary: "landed: 把这个任务做完"},
		{Seq: 4, Ts: 1200, Event: "Stop", Pane: "%31"},
	}, "%31", 900)
	if got != DriverMachine {
		t.Errorf("a dispatch nobody has typed into since was not recognised as a worker: %v", got)
	}
}

func TestAFlagshipTheCommanderDrivesIsNotReapable(t *testing.T) {
	got := drove(t, []Record{
		before,
		{Seq: 2, Ts: 1000, Event: "UserPromptSubmit", Pane: "%19", Summary: "把这个任务做完"},
		{Seq: 3, Ts: 1000, Event: AuditEventSend, Pane: "%19", Actor: "hq", Summary: "landed: 把这个任务做完"},
		// Days later, the commander is still using this session. Nothing delivered it.
		{Seq: 9, Ts: 1800, Event: "UserPromptSubmit", Pane: "%19", Origin: OriginInstruction, Summary: "发版吧"},
	}, "%19", 900)
	if got != DriverPerson {
		t.Errorf("a session the commander has been typing into was not seen as driven: %v", got)
	}
	if !got.Settled() {
		t.Error("someone having typed into it is not an answer that can change")
	}
}

// The case the function this replaced got wrong. Rotation keeps two generations; the
// commander's prompts were in the one it dropped, and what is left starts after the
// dispatch and holds only HQ's deliveries. That answered "a pure worker" — the flagship
// offered up for reclaiming by a different road than #1160's.
func TestAJournalThatNoLongerReachesTheDispatchProvesNothing(t *testing.T) {
	got := drove(t, []Record{
		{Seq: 900, Ts: 5000, Event: "UserPromptSubmit", Pane: "%19", Summary: "把这个任务做完"},
		{Seq: 901, Ts: 5000, Event: AuditEventSend, Pane: "%19", Actor: "hq", Summary: "landed: 把这个任务做完"},
	}, "%19", 1000)
	if got != DriverUncovered {
		t.Errorf("a journal starting after the dispatch was read as evidence: %v", got)
	}
	if !got.Settled() {
		t.Error("rotation only moves forward, so not reaching back is permanent")
	}
}

// No prompt yet is not an answer. It must not settle, or a worker that has not been
// given anything yet could never be suggested once it had.
func TestNoPromptYetIsNotAnAnswer(t *testing.T) {
	got := drove(t, []Record{before, {Seq: 2, Ts: 5000, Event: "Stop", Pane: "%19"}}, "%19", 900)
	if got != DriverUnknown || got.Settled() {
		t.Errorf("a pane with no prompts since read as %v (settled %v)", got, got.Settled())
	}
	if WhoDroveSince([]Record{before}, "", 900) != DriverUnknown ||
		WhoDroveSince([]Record{before}, "%19", 0) != DriverUnknown {
		t.Error("a missing pane or moment produced an answer")
	}
}

// Another pane's traffic says nothing about this one.
func TestAnotherPanesPromptsDoNotDecideThisOne(t *testing.T) {
	recs := []Record{
		before,
		{Seq: 2, Ts: 1000, Event: "UserPromptSubmit", Pane: "%31", Summary: "派给你的活"},
		{Seq: 3, Ts: 1000, Event: AuditEventSend, Pane: "%31", Actor: "hq", Summary: "landed: 派给你的活"},
		{Seq: 4, Ts: 1500, Event: "UserPromptSubmit", Pane: "%19", Origin: OriginInstruction, Summary: "司令在别的船上打字"},
	}
	if got := drove(t, recs, "%31", 900); got != DriverMachine {
		t.Errorf("typing in another pane made this worker look driven: %v", got)
	}
	if got := drove(t, recs, "%19", 900); got != DriverPerson {
		t.Errorf("the pane being typed into was not seen as driven: %v", got)
	}
}
