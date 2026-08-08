package terminal

import (
	"errors"
	"strings"
	"testing"
)

// iterm2 must satisfy Terminal and be reachable via the registry/detection.
var _ Terminal = iterm2{}

var errScript = errors.New("osascript failed")

// A tmux client inside iTerm2 3.6+ is NOT parented to iTerm.app: its real
// ancestor is the session-restoration daemon iTermServer (under
// ~/Library/Application Support/iTerm2/), verified live — so ancestry-based
// detection (focus from the menu-bar app, native sensing) depends on THAT
// command line matching, not the app bundle path.
func TestITerm2ServerAncestry(t *testing.T) {
	const server = "/Users/x/Library/Application Support/iTerm2/iTermServer-3.6.11 /Users/x/Library/Application Support/iTerm2/iterm2-daemon-1.socket"
	if got := terminalFromCommand(server); got != "iterm2" {
		t.Fatalf("terminalFromCommand(iTermServer daemon) = %q, want iterm2 — ancestry detection for iTerm2-hosted tmux clients walks through iTermServer, not iTerm.app", got)
	}
}

func TestITerm2Registered(t *testing.T) {
	t.Setenv("GTMUX_TERMINAL", "iterm2")
	if got := Active().Name(); got != "iTerm2" {
		t.Errorf("Active().Name() with GTMUX_TERMINAL=iterm2 = %q, want iTerm2", got)
	}
}

// SpawnTabs(dryRun) is the part testable without iTerm2 running: the generated
// AppleScript must target "iTerm" (the scripting name — NOT "iTerm2", which
// resolves to the bundle but loads no scripting dictionary) and attach each
// session.
func TestITerm2SpawnTabsScript(t *testing.T) {
	script, err := iterm2{}.SpawnTabs([]string{"work", "my proj"}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`tell application "iTerm"`, "attach -t 'work'", "attach -t 'my proj'", "create tab with default profile"} {
		if !strings.Contains(script, want) {
			t.Errorf("SpawnTabs script missing %q\n---\n%s", want, script)
		}
	}
	if strings.Contains(script, `"iTerm2"`) {
		t.Errorf("SpawnTabs must NOT target \"iTerm2\" (no scripting dictionary)\n---\n%s", script)
	}
}

// interceptOSA replaces the shared osa runner for a test: records the script,
// answers with a canned reply.
func interceptOSA(t *testing.T, reply string, err error) *string {
	t.Helper()
	var got string
	old := osa
	osa = func(script string) (string, error) { got = script; return reply, err }
	t.Cleanup(func() { osa = old })
	return &got
}

// FocusTab's script must target "iTerm", match the tab-session by the exact
// tmux title OR its prefix (iTerm2 3.6 names an attached tmux session
// "#S — #W (tmux)" — the " (tmux)" suffix rides on the window part, verified
// live), and select tab AND window (cross-window focus, verified live on two
// iTerm2 windows).
func TestITerm2FocusTabScript(t *testing.T) {
	got := interceptOSA(t, "ok", nil)
	res, err := iterm2{}.FocusTab("my sess")
	if err != nil || res != "ok" {
		t.Fatalf("FocusTab = %q, %v; want ok, nil", res, err)
	}
	for _, want := range []string{
		`tell application "iTerm"`,
		`nm is "my sess" or nm starts with "my sess — "`,
		"select t",
		"select w",
		"activate",
	} {
		if !strings.Contains(*got, want) {
			t.Errorf("FocusTab script missing %q\n---\n%s", want, *got)
		}
	}
	if strings.Contains(*got, `application "iTerm2"`) {
		t.Errorf("FocusTab must NOT target \"iTerm2\" (no scripting dictionary)\n---\n%s", *got)
	}
}

// IsViewing asks iTerm directly (its AX window title is empty): frontmost gate
// first (not frontmost → the script answers ""), then the current session's
// name must be the session or a "#S — …" prefix of it. Reply shapes are the
// live iTerm2 3.6.11 values.
func TestITerm2IsViewing(t *testing.T) {
	cases := []struct {
		name, reply string
		err         error
		want        bool
	}{
		{"live 3.6.11 shape", "mysess — gtmux (tmux)", nil, true},
		{"no (tmux) suffix", "mysess — vim", nil, true},
		{"bare session name", "mysess", nil, true},
		{"not frontmost", "", nil, false},
		{"another session's tab", "other — vim", nil, false},
		{"prefix is not a word match", "mysess2 — vim", nil, false},
		{"script error", "", errScript, false},
	}
	for _, c := range cases {
		interceptOSA(t, c.reply, c.err)
		if got := (iterm2{}).IsViewing("mysess"); got != c.want {
			t.Errorf("%s: IsViewing = %v, want %v", c.name, got, c.want)
		}
	}
}

// TabOrder maps each tab's current-session name to a tmux session, dropping
// non-tmux tabs (a plain shell tab is titled by its running command — "sleep"
// in the live run — with no " — " separator) and absorbing the " (tmux)"
// suffix, which rides on the window part of "#S — #W (tmux)".
func TestITerm2TabOrder(t *testing.T) {
	interceptOSA(t, "sleep\nalpha — gtmux (tmux)\nbeta — vim\n", nil)
	got := (iterm2{}).TabOrder()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("TabOrder = %v, want [alpha beta]", got)
	}
	interceptOSA(t, "", errScript)
	if got := (iterm2{}).TabOrder(); got != nil {
		t.Fatalf("TabOrder on script error = %v, want nil", got)
	}
}
