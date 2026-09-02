package dispatch

import "testing"

func boxCap(draft string) string {
	return "transcript\n╭──────────────╮\n│ ❯ " + draft + "  │\n╰──────────────╯"
}

func pane(capture string, inMode bool) IO {
	return IO{
		CaptureColor: func() string { return capture },
		InMode:       func() bool { return inMode },
		Sleep:        func() {},
	}
}

// The guard always knew WHY it was saying no and threw the answer away, so every surface
// that noticed the consequence had to guess at the cause. Measured 2026-09-02: HQ's pane
// sat in copy-mode for 1h40m with 18 wakes queued correctly behind this guard, and the
// three surfaces that noticed offered "restart the agent", "check the input box" and
// "reconcile by pull" — none of them the one keypress that fixed it.
func TestReachNamesWhyItRefused(t *testing.T) {
	cases := []struct {
		name   string
		io     IO
		want   ReachReason
		wantOK bool
	}{
		{"copy-mode outranks everything — keys are eaten as navigation there", pane(boxCap(""), true), ReachCopyMode, false},
		{"someone is typing", pane(boxCap("half a thought"), false), ReachDraft, false},
		{"no input region located", pane("just scrollback, no box", false), ReachUnreadable, false},
		{"no eyes at all", IO{InMode: func() bool { return false }}, ReachUnreadable, false},
		{"an empty, readable box", pane(boxCap(""), false), "", true},
	}
	for _, c := range cases {
		got, ok := Reach(c.io)
		if got != c.want || ok != c.wantOK {
			t.Errorf("%s: Reach = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.wantOK)
		}
	}
}

// Every existing caller reads the verdict through BoxEmpty; it must stay exactly the
// answer Reach gives, or the guard and its explanation drift apart.
func TestBoxEmptyIsReachWithoutTheReason(t *testing.T) {
	for _, c := range []IO{pane(boxCap(""), false), pane(boxCap("x"), false), pane(boxCap(""), true), pane("nothing", false)} {
		_, ok := Reach(c)
		if BoxEmpty(c) != ok {
			t.Errorf("BoxEmpty disagreed with Reach")
		}
	}
}

// Two agreeing frames, still: one frame can catch a render mid-flight, and the reason
// must come from the frame that actually refused.
func TestReachStillNeedsTwoFrames(t *testing.T) {
	frames := []string{boxCap(""), boxCap("appeared on the second look")}
	i := 0
	x := IO{
		CaptureColor: func() string { f := frames[i]; i++; return f },
		InMode:       func() bool { return false },
		Sleep:        func() {},
	}
	if r, ok := Reach(x); ok || r != ReachDraft {
		t.Errorf("Reach = (%q, %v), want the second frame's draft to refuse", r, ok)
	}
}
