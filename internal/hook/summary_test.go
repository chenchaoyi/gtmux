package hook

import "testing"

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
