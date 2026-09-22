package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// A conversation has to say whose words these are. On 2026-09-20 a message HQ relayed
// into the commander's pane, 「先停一下，版本对不上，查清楚再做别的。」, was drawn under his own
// avatar, and his history stopped telling him which instructions were his.

// noPane stands in for the tmux lookup: these tests are about the join, not about what a
// live pane is called.
func noPane(string) (string, string) { return "", "" }

func sendRec(pane, actor, state, payload string) events.Record {
	return events.Record{Event: events.AuditEventSend, Pane: pane, Actor: actor, Summary: state + ": " + payload}
}

// indexed writes recs as a journal and looks the pane up through events.Sends' own index,
// so these exercise the one rule the chat really uses to decide what counts as delivered.
func indexed(t *testing.T, recs []events.Record, pane string) map[string]string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(events.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(events.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		b, _ := json.Marshal(r)
		_, _ = f.Write(append(b, '\n'))
	}
	_ = f.Close()
	return (&events.SendIndex{}).Senders(pane, 0)
}

func TestOnlyALandedSendIntoThisPaneAttributesATurn(t *testing.T) {
	recs := []events.Record{
		sendRec("%18", "hq", "landed", "先停一下，版本对不上，查清楚再做别的。"),
		sendRec("%18", "agent:%31", "landed", "把这个任务接着做完"),
		// A refusal never reached the pane, so its words cannot be the turn on screen.
		sendRec("%18", "hq", "refused-draft", "这句没进去"),
		sendRec("%18", "hq", "failed", "这句也没进去"),
		// Another pane's delivery is another conversation.
		sendRec("%20", "hq", "landed", "别的 pane 的话"),
		// A record from before senders were journalled carries no actor.
		sendRec("%18", "", "landed", "老记录"),
		// Not a send at all.
		{Event: "UserPromptSubmit", Pane: "%18", Summary: "landed: 这是事件不是投递"},
	}
	by := indexed(t, recs, "%18")
	if len(by) != 2 {
		t.Fatalf("attributable heads = %d, want 2: %v", len(by), by)
	}
	for _, c := range []struct{ head, want string }{
		{"先停一下，版本对不上，查清楚再做别的。", "hq"},
		{"把这个任务接着做完", "agent:%31"},
	} {
		if got := by[c.head]; got != c.want {
			t.Errorf("%q → %q, want %q", c.head, got, c.want)
		}
	}
	for _, gone := range []string{"这句没进去", "这句也没进去", "别的 pane 的话", "老记录", "这是事件不是投递"} {
		if _, ok := by[gone]; ok {
			t.Errorf("%q should not attribute anything", gone)
		}
	}
}

// The reader's own channels are the reader. Marking them would say "this was not you"
// about the one case where it was.
func TestTheReadersOwnChannelsAreNotSenders(t *testing.T) {
	for _, actor := range []string{"user", "phone:c3bb6e4c", "browser:9f12", "terminal:aa01", "", "something-new"} {
		if s := senderFor(actor, noPane); s != nil {
			t.Errorf("actor %q was marked as someone else: %+v", actor, s)
		}
	}
}

func TestTheSupervisorAndAnotherSessionAreSenders(t *testing.T) {
	resolve := func(pane string) (string, string) {
		if pane == "%31" {
			return "gtmux dev", "Claude Code"
		}
		return "", ""
	}
	hq := senderFor("hq", resolve)
	if hq == nil || hq.Kind != "hq" || hq.Label != "HQ" {
		t.Fatalf("hq = %+v", hq)
	}
	if hq.Pane != "" {
		t.Error("HQ is a role, not a pane the chat should name")
	}

	a := senderFor("agent:%31", resolve)
	if a == nil || a.Kind != "agent" || a.Pane != "%31" {
		t.Fatalf("agent = %+v", a)
	}
	if a.Label != "gtmux dev" || a.Agent != "Claude Code" {
		t.Errorf("a sending session should carry its name and its agent: %+v", a)
	}

	// A pane that has since closed still deserves its id on the message it sent.
	gone := senderFor("agent:%99", resolve)
	if gone == nil || gone.Label != "%99" {
		t.Errorf("a closed pane lost its name entirely: %+v", gone)
	}
}

// The head is matched the way it is written down: the journal holds a bounded head of the
// payload, the transcript holds the whole prompt, and both go through the same fold.
func TestATurnMatchesTheHeadOfWhatWasSent(t *testing.T) {
	long := "把这条改成红档，然后把剩下的都跑一遍，跑完告诉我结果，不要自己合"
	recs := []events.Record{sendRec("%18", "hq", "landed", long)}
	turns := []transcript.Turn{
		{Prompt: long + "\n\n（后面还有一大段补充说明，日志里并没有记下来）"},
		{Prompt: "  先停一下，  版本   对不上  "},
		{Prompt: ""},
	}
	got := stampSendersWith(turns, indexed(t, recs, "%18"), noPane)
	if got[0].From == nil || got[0].From.Kind != "hq" {
		t.Errorf("a turn whose head matches was not attributed: %+v", got[0].From)
	}
	if got[1].From != nil || got[2].From != nil {
		t.Error("a turn with no matching delivery must stay untouched")
	}
}

// Whitespace is folded on both sides: a prompt re-wrapped by the composer is the same
// instruction as the one the journal recorded.
func TestWhitespaceDoesNotBreakTheMatch(t *testing.T) {
	recs := []events.Record{sendRec("%18", "hq", "landed", "先停一下， 版本对不上， 查清楚再做别的。")}
	turns := []transcript.Turn{{Prompt: "先停一下，\n版本对不上，\n查清楚再做别的。"}}
	got := stampSendersWith(turns, indexed(t, recs, "%18"), noPane)
	if got[0].From == nil {
		t.Error("a re-wrapped prompt lost its sender")
	}
}

// The journal is rotated by size, so a turn older than it cannot be attributed. Saying
// nothing is the only honest answer, and it renders as it always did.
func TestNothingToJoinAgainstLeavesTheTurnAlone(t *testing.T) {
	turns := []transcript.Turn{{Prompt: "很久以前说过的一句话"}}
	got := stampSendersWith(turns, map[string]string{}, noPane)
	if got[0].From != nil {
		t.Errorf("an unmatched turn was marked: %+v", got[0].From)
	}
}

// The lookback reaches the oldest turn on screen and stops: a short conversation must not
// read a one-second window, and a stitched history must not read the whole journal.
func TestTheLookbackCoversTheTurnsAndIsBounded(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	at := func(d time.Duration) string { return now.Add(-d).Format(time.RFC3339) }

	if got := sendLookback(nil, now.Unix()); got != sendLookbackFloor {
		t.Errorf("no turns → %d, want the floor %d", got, sendLookbackFloor)
	}
	if got := sendLookback([]transcript.Turn{{Time: at(time.Minute)}}, now.Unix()); got != sendLookbackFloor {
		t.Errorf("a minute-old turn → %d, want the floor %d", got, sendLookbackFloor)
	}
	day := []transcript.Turn{{Time: at(24 * time.Hour)}, {Time: at(time.Hour)}}
	if got := sendLookback(day, now.Unix()); got <= 24*3600 {
		t.Errorf("a day-old turn → %d, want more than a day", got)
	}
	old := []transcript.Turn{{Time: at(400 * 24 * time.Hour)}}
	if got := sendLookback(old, now.Unix()); got != sendLookbackCeiling {
		t.Errorf("a year-old turn → %d, want the ceiling %d", got, sendLookbackCeiling)
	}
	// A turn whose log line carried no clock must not drag the window to the epoch.
	if got := sendLookback([]transcript.Turn{{Time: ""}}, now.Unix()); got != sendLookbackFloor {
		t.Errorf("a clockless turn → %d, want the floor", got)
	}
}

// A short delivery must not claim a long prompt that merely starts the same way. "继续"
// begins any number of unrelated instructions, and a wrong avatar on the commander's own
// words is the defect this whole join exists to remove.
func TestAShortDeliveryDoesNotClaimALongerPrompt(t *testing.T) {
	by := indexed(t, []events.Record{sendRec("%18", "hq", "landed", "继续")}, "%18")
	got := stampSendersWith([]transcript.Turn{
		{Prompt: "继续"},
		{Prompt: "继续把剩下的十几个 PR 都合掉，然后发版"},
	}, by, noPane)
	if got[0].From == nil {
		t.Error("the exact short message lost its sender")
	}
	if got[1].From != nil {
		t.Errorf("a short delivery claimed a longer prompt: %+v", got[1].From)
	}
}

// Two deliveries where one head begins the other: the longer, more specific one wins.
func TestTheMoreSpecificDeliveryWins(t *testing.T) {
	by := indexed(t, []events.Record{
		sendRec("%18", "hq", "landed", "把这个任务接着做完"),
		sendRec("%18", "agent:%31", "landed", "把这个任务接着做完，做完直接合，不用问我"),
	}, "%18")
	got := stampSendersWith([]transcript.Turn{
		{Prompt: "把这个任务接着做完，做完直接合，不用问我"},
	}, by, noPane)
	if got[0].From == nil || got[0].From.Kind != "agent" {
		t.Errorf("the shorter head claimed it: %+v", got[0].From)
	}
}

// The writer and the reader have to agree. A field the journal spells one way and the
// join reads another is the failure this whole change is about, so the round trip runs
// through the real append: AuditSend writes, the journal is read back, and the join has
// to find the sender in it.
func TestTheSenderSurvivesTheJournal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	diag.SetProcess("test", "hq")
	t.Cleanup(func() { diag.SetProcess("", "") })

	now := time.Now().Unix()
	events.AuditSend("%18", "landed", "先停一下，版本对不上，查清楚再做别的。", now)
	events.AuditSend("%18", "refused-draft", "这句没进去，草稿里有别人的字", now)

	turns := stampSenders([]transcript.Turn{
		{Prompt: "先停一下，版本对不上，查清楚再做别的。"},
		{Prompt: "这句没进去，草稿里有别人的字"},
		{Prompt: "这句是我自己敲的"},
	}, "%18", now)

	if turns[0].From == nil || turns[0].From.Kind != "hq" || turns[0].From.Label != "HQ" {
		t.Fatalf("a landed HQ send did not come back through the journal: %+v", turns[0].From)
	}
	if turns[1].From != nil {
		t.Errorf("a refused send claimed a turn: %+v", turns[1].From)
	}
	if turns[2].From != nil {
		t.Errorf("an unsent prompt was attributed: %+v", turns[2].From)
	}
}

// A DISPATCH is a delivery too, and it has to leave the same trace.
//
// The audit trail could account for every hand-typed `gtmux send` and none of the SPAWNED
// deliveries, which is the larger half. A reader asking "who put this prompt in the pane"
// got no answer for the one case gtmux itself caused, and the reap gate (issue #1160)
// could not recognise the very thing it exists to recognise: a worker nobody but gtmux
// has ever typed into.
//
// Source-level, because the property is "every path that delivers also journals it" and a
// new delivery path forgetting is exactly how this happened once.
func TestEveryDeliveryPathJournalsIt(t *testing.T) {
	for _, f := range []string{"send.go", "spawn.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		if !strings.Contains(src, "dispatch.Deliver(") {
			continue // not a delivery path (any more)
		}
		if !strings.Contains(src, "events.AuditSend(") {
			t.Errorf("%s delivers into a pane but journals nothing: a reader cannot say who wrote that prompt", f)
		}
	}
}
