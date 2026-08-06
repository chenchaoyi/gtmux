package hook

import "testing"

// The codex-only, no-re-detach gating for the SYNC-hook detach.
func TestShouldDetachCodexHook(t *testing.T) {
	cases := []struct {
		agent string
		args  []string
		want  bool
	}{
		{"codex", []string{"--agent", "codex", "UserPromptSubmit"}, true},
		{"codex", []string{"--detached", "--agent", "codex", "UserPromptSubmit"}, false}, // worker must not re-detach
		{"claude", []string{"--agent", "claude", "Stop"}, false},                         // non-codex untouched
		{"opencode", []string{"--agent", "opencode", "Stop"}, false},
	}
	for _, c := range cases {
		if got := shouldDetachCodexHook(c.agent, c.args); got != c.want {
			t.Errorf("shouldDetachCodexHook(%q, %v) = %v, want %v", c.agent, c.args, got, c.want)
		}
	}
}
