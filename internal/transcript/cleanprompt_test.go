package transcript

import "testing"

// CleanUserPrompt must drop every class of system-injected content but pass a real
// user prompt through untouched — the fix for a `<task-notification>` fragment being
// read back as a session goal.
func TestCleanUserPrompt(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"real prompt", "cut a new release", "cut a new release", true},
		{"closed task-notification", "<task-notification> <task-id>b50xphl27</task-id> done </task-notification>", "", false},
		// The exact leaked fragment: an UNCLOSED/truncated task-notification.
		{"truncated task-notification", "<task-notification> <task-id>b50xphl27</", "", false},
		{"system-reminder block", "<system-reminder>context is running low</system-reminder>", "", false},
		{"truncated system-reminder", "<system-reminder>context is running", "", false},
		{"task-notification with attrs", `<task-notification id="x"> bg task done </task-notification>`, "", false},
		{"SYSTEM NOTIFICATION line", "[SYSTEM NOTIFICATION 2026-07-13] a background task finished", "", false},
		{"gtmux nudge echoed back", `[gtmux] goal-changed gtmux:0 (%20) — goal:"do the thing"`, "", false},
		// Injected content APPENDED to a real prompt → keep only the real prompt.
		{"real + trailing reminder", "fix the bug\n<system-reminder>note</system-reminder>", "fix the bug", true},
		{"real + trailing nudge", "ship it\n[gtmux] done gtmux:0 (%1) — goal:\"x\"", "ship it", true},
		{"real + leading task-notification", "<task-notification>done</task-notification>\nnow do X", "now do X", true},
		{"empty", "   ", "", false},
	}
	for _, c := range cases {
		got, ok := CleanUserPrompt(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("%s: CleanUserPrompt(%q) = (%q, %v), want (%q, %v)", c.name, c.in, got, ok, c.want, c.wantOK)
		}
	}
}
