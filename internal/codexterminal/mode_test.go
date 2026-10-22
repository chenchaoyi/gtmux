package codexterminal

import "testing"

func TestProcessRecoveryRejectsHeadlessModesEvenWithTerminalStdin(t *testing.T) {
	for _, tc := range []struct {
		args string
		want bool
	}{
		{"/opt/codex", true},
		{"codex --model synthetic-model resume thread-id", true},
		{"codex fork thread-id", true},
		{"codex app-server", false},
		{"codex --config key=value app-server --listen stdio://", false},
		{"codex mcp-server", false},
		{"codex exec task", false},
		{"codex e task", false},
		{"codex explain 'app-server' behavior", false},
		{"other-agent codex", false},
		{"/path with spaces/codex", false},
		{"codex\napp-server", false},
		{"", false},
	} {
		if got := interactiveCommand(tc.args); got != tc.want {
			t.Errorf("command classification = %v for %q, want %v", got, tc.args, tc.want)
		}
	}
}
