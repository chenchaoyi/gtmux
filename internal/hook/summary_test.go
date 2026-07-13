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
