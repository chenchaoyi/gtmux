package hook

import "testing"

// The chain measured on 2026-08-18 when pane %13 went blind: the hook fires from
// Claude's background session host, which has neither $TMUX_PANE nor a tty, and
// the pane's own bash sits four hops up.
func bgHostChain() []ancestor {
	return []ancestor{
		{pid: 90001, tty: ""},        // gtmux hook
		{pid: 78152, tty: ""},        // the session process (--session-id …)
		{pid: 77301, tty: ""},        // --bg-pty-host
		{pid: 77257, tty: ""},        // claude daemon run
		{pid: 34175, tty: "ttys014"}, // the pane's interactive claude
		{pid: 19982, tty: "ttys014"}, // the pane's bash — tmux's pane_pid
	}
}

func TestResolvePaneByPanePID(t *testing.T) {
	panes := []identityPane{
		{id: "%12", pid: 111, tty: "/dev/ttys009"},
		{id: "%13", pid: 19982, tty: "/dev/ttys014"},
	}
	if got := resolvePane(bgHostChain(), panes); got != "%13" {
		t.Fatalf("pane_pid match: got %q, want %%13", got)
	}
}

// A pane whose pane_pid is gone from the chain (an extra layer of hosting, a
// re-exec) is still identifiable by the pty tmux allocated for it.
func TestResolvePaneByTTYWhenNoPIDMatches(t *testing.T) {
	panes := []identityPane{
		{id: "%12", pid: 111, tty: "/dev/ttys009"},
		{id: "%13", pid: 55555, tty: "/dev/ttys014"}, // pid not in the chain
	}
	if got := resolvePane(bgHostChain(), panes); got != "%13" {
		t.Fatalf("pane_tty match: got %q, want %%13", got)
	}
}

// The whole point of the fallback is that it must not steal a genuinely native
// session — an agent in a plain terminal has that terminal's tty, never a pane's.
func TestResolvePaneLeavesANativeSessionAlone(t *testing.T) {
	chain := []ancestor{{pid: 4242, tty: "ttys077"}, {pid: 4200, tty: "ttys077"}}
	panes := []identityPane{
		{id: "%12", pid: 111, tty: "/dev/ttys009"},
		{id: "%13", pid: 19982, tty: "/dev/ttys014"},
	}
	if got := resolvePane(chain, panes); got != "" {
		t.Fatalf("native session claimed pane %q, want no match", got)
	}
}

// Ambiguity is not resolved by picking one: writing one agent's events onto
// another agent's row is worse than the native fallback.
func TestResolvePaneRefusesAmbiguity(t *testing.T) {
	chain := []ancestor{{pid: 19982, tty: "ttys014"}}
	byPID := []identityPane{{id: "%13", pid: 19982}, {id: "%99", pid: 19982}}
	if got := resolvePane(chain, byPID); got != "" {
		t.Fatalf("ambiguous pane_pid resolved to %q, want no match", got)
	}
	byTTY := []identityPane{{id: "%13", tty: "/dev/ttys014"}, {id: "%99", tty: "/dev/ttys014"}}
	if got := resolvePane(chain, byTTY); got != "" {
		t.Fatalf("ambiguous pane_tty resolved to %q, want no match", got)
	}
}

// pane_pid is the stronger signal and is consulted for the WHOLE chain before the
// tty is considered at all — a tty shared with a neighbouring pane must not win
// over an exact process match further up.
func TestResolvePanePrefersPIDOverTTY(t *testing.T) {
	chain := []ancestor{{pid: 34175, tty: "ttys014"}, {pid: 19982, tty: "ttys014"}}
	panes := []identityPane{
		{id: "%98", pid: 0, tty: "/dev/ttys014"}, // tty match only
		{id: "%13", pid: 19982, tty: ""},         // exact process match
	}
	if got := resolvePane(chain, panes); got != "%13" {
		t.Fatalf("got %q, want the pane_pid match %%13", got)
	}
}

// A process with no controlling terminal reports "??" (macOS) or "-"; neither may
// ever match a pane whose tty field is missing for its own reasons.
func TestNoTTYNeverMatches(t *testing.T) {
	for _, none := range []string{"??", "-", "?", "", "  "} {
		if normalizeTTY(none) != "" {
			t.Fatalf("normalizeTTY(%q) should be empty", none)
		}
		if sameTTY(none, none) {
			t.Fatalf("sameTTY(%q, %q) must be false", none, none)
		}
	}
	chain := []ancestor{{pid: 1, tty: "??"}}
	panes := []identityPane{{id: "%13", pid: 0, tty: ""}}
	if got := resolvePane(chain, panes); got != "" {
		t.Fatalf("tty-less chain matched %q", got)
	}
}

// `ps` prints ttys014 where tmux prints /dev/ttys014 — the same terminal.
func TestTTYSpellingsMatch(t *testing.T) {
	if !sameTTY("/dev/ttys014", "ttys014") {
		t.Fatal("tmux and ps spellings of one tty must match")
	}
}

// A pid of 0 in the pane table (tmux gave us nothing) must not match an ancestor
// whose pid we also failed to read.
func TestZeroPIDsDoNotMatch(t *testing.T) {
	chain := []ancestor{{pid: 0, tty: ""}}
	panes := []identityPane{{id: "%13", pid: 0, tty: ""}}
	if got := resolvePane(chain, panes); got != "" {
		t.Fatalf("zero pids matched %q", got)
	}
}
