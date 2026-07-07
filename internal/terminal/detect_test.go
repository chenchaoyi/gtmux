package terminal

import "testing"

func TestFromTermEnv(t *testing.T) {
	cases := []struct{ prog, want string }{
		{"ghostty", "ghostty"},
		{"iTerm.app", "iterm2"},
		{"Apple_Terminal", "appleterminal"},
		{"WezTerm", "wezterm"},
		{"WarpTerminal", "warp"},
		{"", ""},
		{"SomethingElse", ""},
	}
	for _, c := range cases {
		t.Setenv("TERM_PROGRAM", c.prog)
		t.Setenv("KITTY_WINDOW_ID", "")
		t.Setenv("TERM", "")
		if got := fromTermEnv(); got != c.want {
			t.Errorf("fromTermEnv(TERM_PROGRAM=%q) = %q, want %q", c.prog, got, c.want)
		}
	}
	// kitty is detected via its own env vars, not TERM_PROGRAM.
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "3")
	if got := fromTermEnv(); got != "kitty" {
		t.Errorf("fromTermEnv(KITTY_WINDOW_ID set) = %q, want kitty", got)
	}
}

func TestTerminalFromCommand(t *testing.T) {
	cases := []struct{ cmd, want string }{
		{"/Applications/Ghostty.app/Contents/MacOS/ghostty", "ghostty"},
		{"/Applications/iTerm.app/Contents/MacOS/iTerm2", "iterm2"},
		{"/System/Applications/Utilities/Terminal.app/Contents/MacOS/Terminal", "appleterminal"},
		{"/Applications/kitty.app/Contents/MacOS/kitty", "kitty"},
		{"/opt/homebrew/bin/wezterm-gui", "wezterm"},
		{"/Applications/Warp.app/Contents/MacOS/stable", "warp"},
		{"-bash", ""},
		{"/usr/bin/login -fp ccy", ""},
	}
	for _, c := range cases {
		if got := terminalFromCommand(c.cmd); got != c.want {
			t.Errorf("terminalFromCommand(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

// resolveName honors GTMUX_TERMINAL first, short-circuiting detection.
func TestResolveNameOverride(t *testing.T) {
	t.Setenv("GTMUX_TERMINAL", "iterm2")
	if got := resolveName(); got != "iterm2" {
		t.Errorf("resolveName() with override = %q, want iterm2", got)
	}
}

// resolveNameForSession also honors the explicit override, ahead of any
// per-session client lookup (the deterministic, tmux-free part of the ordering).
func TestResolveNameForSessionOverride(t *testing.T) {
	t.Setenv("GTMUX_TERMINAL", "ghostty")
	if got := resolveNameForSession("whatever"); got != "ghostty" {
		t.Errorf("resolveNameForSession override = %q, want ghostty", got)
	}
}

// An empty session name can't resolve a client → no panic, falls through to "".
func TestTerminalFromSessionClientsEmpty(t *testing.T) {
	if got := terminalFromSessionClients(""); got != "" {
		t.Errorf("terminalFromSessionClients(\"\") = %q, want \"\"", got)
	}
}
