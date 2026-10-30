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

func TestManagedBackendRequiresUnambiguousDaemonCommand(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    bool
	}{
		{"/opt/codex app-server --listen unix:// --analytics-default-enabled --managed-daemon", true},
		{"codex app-server --managed-daemon --listen unix:///fixture.sock", true},
		{"codex app-server --managed-daemon", true},
		{"codex app-server --listen unix://", false},
		{"codex resume thread", false},
		{"codex explain app-server --managed-daemon", false},
		{"codex app-server --config label=some --managed-daemon", false},
		{"codex app-server --config 'label=some --managed-daemon'", false},
		{"codex app-server --managed-daemon -- task", false},
		{"codex app-server --listen --managed-daemon", false},
		{"other app-server --managed-daemon", false},
		{"codex app-server\n--managed-daemon", false},
		{"", false},
	} {
		if got := managedBackendCommand(tc.command); got != tc.want {
			t.Errorf("managed backend = %v for %q, want %v", got, tc.command, tc.want)
		}
	}
}
