package agents

import "testing"

func TestInteractiveCommandScrollbackDefaults(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"codex", "codex --no-alt-screen"},
		{"  /opt/bin/codex --model chosen", "  /opt/bin/codex --no-alt-screen --model chosen"},
		{"codex resume", "codex --no-alt-screen resume"},
		{"'codex' resume", "'codex' --no-alt-screen resume"},
		{"codex --no-alt-screen --approve-for-me", "codex --no-alt-screen --approve-for-me"},
		{"codex -c 'tui.alternate_screen=always'", "codex -c 'tui.alternate_screen=always'"},
		{"codex --config 'tui.alternate_screen = \"never\"'", "codex --config 'tui.alternate_screen = \"never\"'"},
		{"agentwrap resume", "agentwrap resume"},
		{"claude", "claude"},
		{"codex exec", "codex exec"},
		{"codex --model chosen app-server", "codex --model chosen app-server"},
		{"", ""},
	} {
		if got := InteractiveCommand(tc.in); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}
