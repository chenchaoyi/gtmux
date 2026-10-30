package native

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

func TestSharedBackendRequiresStableExactProcessAndPositiveRole(t *testing.T) {
	oldRead, oldRole := readAgentProcess, managedCodexBackend
	t.Cleanup(func() { readAgentProcess, managedCodexBackend = oldRead, oldRole })
	r := Record{Agent: "codex", PID: 200, Comm: "codex", ProcessStart: "original-birth"}
	for _, tc := range []struct {
		name        string
		role, known bool
		after       string
		want        bool
	}{
		{"managed stable", true, true, "original-birth", true},
		{"legacy frontend", false, true, "original-birth", false},
		{"unknown process", true, false, "original-birth", false},
		{"PID reused during role read", true, true, "replacement-birth", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			readAgentProcess = func(pid int) (launchhost.AgentProcess, bool) {
				calls++
				start := r.ProcessStart
				if calls > 1 {
					start = tc.after
				}
				return launchhost.AgentProcess{PID: pid, Comm: "codex", Start: start}, tc.known
			}
			managedCodexBackend = func(pid int) bool { return pid == r.PID && tc.role }
			if got := SharedCodexBackend(r); got != tc.want {
				t.Fatalf("backend = %v, want %v", got, tc.want)
			}
		})
	}
	if SharedCodexBackend(Record{Agent: "codex", PID: 200, Comm: "codex"}) || SharedCodexBackend(Record{Agent: "claude", PID: 200, Comm: "codex", ProcessStart: r.ProcessStart}) {
		t.Fatal("unknown birth or another agent was classified")
	}
}
