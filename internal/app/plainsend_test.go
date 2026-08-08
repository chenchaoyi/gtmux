package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// plainTermPane routes a send between the agent box-confirm pipeline and the direct
// type path. The table pins both directions: only "no agent found + bare shell
// foreground" is plain; every ambiguous shape fails SAFE to the agent pipeline
// (pre-fix behavior for those panes).
func TestPlainTermPane(t *testing.T) {
	cases := []struct {
		driverKey, fg string
		want          bool
		why           string
	}{
		{"", "bash", true, "bare bash pane (the pane-browser plain terminal)"},
		{"", "zsh", true, "bare zsh pane"},
		{"", "-zsh", true, "login-shell name"},
		{"", "fish", true, "fish shell"},
		{"claude", "claude", false, "agent found in the subtree"},
		{"codex", "codex", false, "agent found in the subtree"},
		{"claude", "bash", false, "agent in subtree wins even over a shell foreground"},
		{"", "2.1.220", false, "Claude's version-named process with the subtree scan missed — MUST fail safe to the agent pipeline (see pane-current-command-is-agent-version footgun)"},
		{"", "vim", false, "full-screen non-agent TUI keeps pre-fix behavior"},
		{"", "ssh", false, "remote shell is not provably plain"},
		{"", "", false, "unknown foreground"},
	}
	for _, c := range cases {
		if got := plainTermPane(c.driverKey, c.fg); got != c.want {
			t.Errorf("plainTermPane(%q, %q) = %v, want %v — %s", c.driverKey, c.fg, got, c.want, c.why)
		}
	}
}

// The real frame from the live repro (2026-08-08, a bash pane created via the pane
// browser): the pane USED to show an agent-style input box, so its scrollback holds
// stale box borders; the send's echo sits at the live shell prompt BELOW the stale
// box's bottom border.
const stalBoxBashFrame = `  Storage
    ✓  gtmux disk usage    19 MB            state dir under control
  16 ok · 0 to improve · 0 blocking
ccy@host /tmp $ printf "%s\n" ...
╭──────────────────────────────╮
│ ❯                            │
╰──────────────────────────────╯
ccy@host /tmp $
ccy@host /tmp $ echo stale-box-test`

// Pins WHY plain panes must not ride the box-confirm pipeline: on a plain shell pane
// whose scrollback holds a stale agent input box, the region detector locks onto the
// STALE box, and the text echoed at the live prompt below it is invisible to the
// draft check — so confirmPaste can only ever settle on "fragment" (the false "Not
// sent — the input box didn't confirm the full message", and a C-u that wiped the
// user's shell line). This is the dispatch behavior the routing works around; if the
// detector ever learns to ignore scrollback boxes, this test will flag that the
// routing rationale should be revisited — not that anything is broken.
func TestStaleScrollbackBoxHidesPlainPaneEcho(t *testing.T) {
	_, draft, structured := dispatch.SplitInputRegion(stalBoxBashFrame)
	if !structured {
		t.Fatalf("expected the stale scrollback box to be (mis)read as an input region; got unstructured — the plain-pane routing rationale may be stale")
	}
	if strings.Contains(draft, "stale-box-test") {
		t.Fatalf("expected the echo at the live prompt to be OUTSIDE the detected draft region, got draft=%q", draft)
	}
}

// The direct path's text-shape split: single-line goes as literal keystrokes,
// multi-line rides the paste buffer (newlines must not submit line-by-line).
func TestKeystrokeTextShapes(t *testing.T) {
	if !keystrokeText("gtmux update") {
		t.Errorf("single-line text should go as keystrokes")
	}
	if keystrokeText("line one\nline two") {
		t.Errorf("multi-line text must ride the paste buffer")
	}
	if keystrokeText("") {
		t.Errorf("empty text is not a keystroke send")
	}
}
