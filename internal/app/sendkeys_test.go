package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// allowedSendKeys whitelists exactly the named control keys POST /api/send may
// inject. Everything else (free-form text, unlisted control keys) must be denied
// by the map so it never reaches tmux as a key name.
func TestAllowedSendKeysWhitelist(t *testing.T) {
	allowed := []string{
		"Enter", "C-c", "Escape", "Tab",
		"Up", "Down", "Left", "Right",
		"BSpace", "C-d", "C-z", "C-l",
	}
	for _, k := range allowed {
		if !allowedSendKeys[k] {
			t.Errorf("key %q should be allowed", k)
		}
	}

	denied := []string{
		"rm -rf", "C-x", "C-a", "F1", "PageUp", "kill",
		"enter", "ESC", "", "C-c; rm",
	}
	for _, k := range denied {
		if allowedSendKeys[k] {
			t.Errorf("key %q must NOT be allowed", k)
		}
	}

	// The allowlist must not have silently grown beyond the audited set.
	if len(allowedSendKeys) != len(allowed) {
		t.Errorf("allowedSendKeys has %d entries, want %d (audit any additions)", len(allowedSendKeys), len(allowed))
	}
}

// sendToPane rejects a missing/empty tmux binary up front (no shelling out). With
// tmux.Bin forced empty we exercise only the pure guard path — never a real tmux.
func TestSendToPaneNoTmuxRejects(t *testing.T) {
	saved := tmux.Bin
	tmux.Bin = ""
	t.Cleanup(func() { tmux.Bin = saved })

	if err := sendToPane("%1", "hello", "", false); err == nil {
		t.Errorf("sendToPane with no tmux should error (pane not found)")
	}
	// A disallowed key with no tmux still errors (the bin guard fires first).
	if err := sendToPane("%1", "", "C-x", false); err == nil {
		t.Errorf("sendToPane with no tmux + bad key should error")
	}
}

// keystrokeText decides send-keys -l (a keystroke — what a numbered menu commits on)
// vs the paste buffer (multi-line). Regression guard for "tapping a number in the
// approval card does nothing" — a bracketed-pasted digit selects no menu option.
func TestKeystrokeText(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"1", true},       // a menu digit → keystroke
		{"hello", true},   // single-line text → keystrokes
		{"", false},       // empty → nothing to send
		{"a\nb", false},   // multi-line → paste buffer
		{"line\n", false}, // trailing newline → multi-line path
	}
	for _, c := range cases {
		if got := keystrokeText(c.in); got != c.want {
			t.Errorf("keystrokeText(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
