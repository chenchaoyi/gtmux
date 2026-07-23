package hook

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
)

func TestClassifyReply(t *testing.T) {
	cases := []struct {
		name  string
		reply string
		want  string
	}{
		{"plain question", "I can install it now. 放行就装?", "asking"},
		{"ascii question", "Should I proceed with the deploy?", "asking"},
		{"statement", "Done — the build passed and I pushed the branch.", "report"},
		{"question in code only", "Here is the fix:\n```\nif ok? {}\n```\nApplied it.", "report"},
		{"trailing emphasis", "Ready to merge — go ahead?**", "asking"},
		{"heading not counted", "Summary\n# Next steps?", "report"},
		// The dogfood bug: a question to the user followed by a status footer + sign-off.
		// The final prose line isn't a question, but the trailing block still asks.
		{"question then footer signoff",
			"这里有个不一致要你确认?\n\n## Token usage\n- session 5%\n- week 65%\n\n待命。", "asking"},
		{"question then short signoff", "公司网直连是不是指另一张网?\n说\"开工\"我就起分支。待命。", "asking"},
		// Over-fire bound: a question far ABOVE the trailing block stays a report.
		{"question far above tail",
			"Do you want A or B?\n1\n2\n3\n4\n5\n6\n7\nAll done, shipped.", "report"},
	}
	for _, c := range cases {
		if got := classifyReply(c.reply); got != c.want {
			t.Errorf("%s: classifyReply=%q want %q", c.name, got, c.want)
		}
	}
}

func TestReplyTail_KeepsEnd(t *testing.T) {
	long := ""
	for i := 0; i < 500; i++ {
		long += "x "
	}
	long += "END"
	got := replyTail(long)
	if len(got) == 0 || got[len(got)-3:] != "END" {
		t.Fatalf("reply tail should keep the end, got tail %q", got[max(0, len(got)-6):])
	}
}

func TestEventSummary_PromptHead(t *testing.T) {
	sum, class := eventSummary("UserPromptSubmit", "  Please  refactor   the verifier  ", "", "", "claude")
	if sum != "Please refactor the verifier" {
		t.Fatalf("prompt head = %q", sum)
	}
	if class != "" {
		t.Fatalf("submit has no class, got %q", class)
	}
}

// A submitted WAKE BATCH records its trailing id as the Summary — the wake
// channel's delivery receipt (hqnudge confirms the batch from this event) — while
// the batch text itself stays dropped (it must never become a goal).
func TestEventSummary_WakeBatchRecordsItsID(t *testing.T) {
	sum, class := eventSummary("UserPromptSubmit",
		`» gtmux·done  gtmux:0.0 (%14) │ goal:"x" · #a3f1c2`, "", "", "claude")
	if sum != "#a3f1c2" || class != "" {
		t.Fatalf("wake batch summary = %q/%q, want the batch id", sum, class)
	}
	// A wake line WITHOUT an id (a stray echo, not a delivered batch) stays silent.
	if sum, _ := eventSummary("UserPromptSubmit", "» gtmux·done  %14", "", "", "claude"); sum != "" {
		t.Fatalf("an id-less wake echo must record nothing, got %q", sum)
	}
}

// A UserPromptSubmit carrying a system-injected block (task-notification, our own
// nudge) must yield NO summary — so it never becomes a goal or a goal-changed nudge.
func TestEventSummary_DropsInjectedPrompt(t *testing.T) {
	for _, in := range []string{
		"<task-notification> <task-id>b50xphl27</",
		"<system-reminder>context low</system-reminder>",
		"[SYSTEM NOTIFICATION] background task done",
		`[gtmux] goal-changed gtmux:0 (%20) — goal:"x"`,
	} {
		if sum, _ := eventSummary("UserPromptSubmit", in, "", "", "claude"); sum != "" {
			t.Errorf("injected prompt %q should yield no summary, got %q", in, sum)
		}
	}
}

// TestEventSummary_SharesTheDeliverNeedlePipeline pins the single-track guarantee
// (openspec agent-drivers P1): the Summary the hook records for a UserPromptSubmit
// IS dispatch.NormalizeNeedle of the raw prompt — the same function Deliver derives
// its matching needle with — so the two sides can never drift apart again (the
// dual-track normalization was a real "NOT delivered" misjudgment source).
func TestEventSummary_SharesTheDeliverNeedlePipeline(t *testing.T) {
	payloads := []string{
		"cut a new release and update the cask",
		"  Please  refactor   the verifier  ",
		"fix the flaky test\n<system-reminder>context low</system-reminder>",
		"» gtmux·done  gtmux:0.0 (%14) │ goal:\"x\"\n继续 P2，按 tasks.md 逐项落地",
		strings.Repeat("多字节长指令 ", 30),
	}
	for _, p := range payloads {
		sum, _ := eventSummary("UserPromptSubmit", p, "", "", "claude")
		if sum == "" {
			t.Errorf("payload should record a summary: %q", p)
			continue
		}
		if want := dispatch.NormalizeNeedle(p); sum != want {
			t.Errorf("hook summary %q != deliver needle %q for %q", sum, want, p)
		}
	}
}

// goalOf decides what a submission MEANS for the goal-changed wake. The slash case is
// the one the event summary cannot express: no prose, but the user acted.
func TestGoalOf(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"typed prose", "cut a new release", "cut a new release"},
		{"slash command", "<command-name>/compact</command-name>", "(slash-command) /compact"},
		{"slash, unreadable name", "<command-name>", "(slash-command)"},
		{"harness injection", "<system-reminder>context low</system-reminder>", ""},
		{"our own wake line", `» gtmux·done  gtmux:0.0 (%14) │ goal:"x"`, ""},
		{"legacy nudge echo", `[gtmux] goal-changed gtmux:0 (%20) — goal:"x"`, ""},
		{"empty", "  ", ""},
	}
	for _, c := range cases {
		if got := goalOf(c.in); got != c.want {
			t.Errorf("%s: goalOf(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// The goal-changed payload is the FULL prompt, not the event summary's 40-rune head:
// the head exists for dispatch matching, and the wake clamps for display itself.
func TestGoalOf_KeepsTheFullPromptForFingerprinting(t *testing.T) {
	long := strings.Repeat("ship ", 20)
	if got := goalOf(long); got != strings.TrimSpace(long) {
		t.Errorf("goalOf must not truncate; got %q", got)
	}
}

// goalOf is the ONE classifier behind both the goal-changed wake and the record's
// `origin` stamp (hq-attention-stream) — so the tier a pull-side HQ reads and the knock
// a live HQ gets can never disagree about what counts as a user act. This pins that
// pairing: whatever wakes must also be visible by pull, and whatever is silent must not
// inflate the stream.
func TestGoalOf_DecidesBothTheWakeAndTheTier(t *testing.T) {
	for _, c := range []struct {
		name, raw string
		wantSeen  bool
	}{
		{"typed prose", "先别动那个分支", true},
		{"slash command", "<command-name>/compact</command-name>", true},
		{"harness injection", "<system-reminder>context low</system-reminder>", false},
		{"our own wake line", `» gtmux·done  gtmux:0.0 (%14) │ goal:"x"`, false},
	} {
		goal := goalOf(c.raw)
		origin := ""
		if goal != "" {
			origin = events.OriginInstruction // the stamp hook.go applies
		}
		gotSeen := events.SeverityRank(events.Severity(events.Record{
			Event: "UserPromptSubmit", Origin: origin,
		})) > events.SeverityRank(events.SevRoutine)
		if gotSeen != c.wantSeen {
			t.Errorf("%s: visible above routine = %v, want %v (goal=%q)",
				c.name, gotSeen, c.wantSeen, goal)
		}
		if (goal != "") != c.wantSeen {
			t.Errorf("%s: the wake and the tier disagree — goal=%q but visible=%v",
				c.name, goal, gotSeen)
		}
	}
}

func TestClassify_PreCompactIsStateNeutralLifecycle(t *testing.T) {
	// PreCompact must be a lifecycle event (so it reaches the event stream) that
	// changes NO marker (decide has an empty decision for it).
	if got := classify("claude", "PreCompact", "").Lifecycle; got != "PreCompact" {
		t.Fatalf("PreCompact lifecycle = %q, want PreCompact", got)
	}
	d := decide("PreCompact", true)
	if d.setActive || d.clearActive || d.setWaiting || d.clearWaiting || d.setFinished || d.notify {
		t.Fatalf("PreCompact must not touch any marker: %+v", d)
	}
}
