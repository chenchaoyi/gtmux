package radar

import "testing"

// TestAgentFromCommand: a pane's foreground command is often just "node", but the
// full argv reveals the agent. Match by the basename of the executable / path
// tokens, without false-matching a bare filename argument.
func TestAgentFromCommand(t *testing.T) {
	p := builtinProfiles
	cases := []struct {
		cmd, want string
	}{
		{"node /Users/ccy/.nvm/versions/node/v22.22.0/bin/codex", "Codex"},
		{"node /opt/homebrew/lib/node_modules/.bin/claude --resume", "Claude Code"},
		{"codex", "Codex"},                // run directly (first token)
		{"-bash", ""},                     // a shell
		{"vim internal/app/notes.md", ""}, // unrelated editor
		{"cat codex", ""},                 // bare filename arg, not the executable
		{"/Applications/Codex.app/Contents/MacOS/Codex", ""}, // the DESKTOP app (capital C ≠ "codex")
		{"node /home/u/bin/gemini", "Gemini"},
	}
	for _, c := range cases {
		if got := agentFromCommand(c.cmd, p); got != c.want {
			t.Errorf("agentFromCommand(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

// TestAgentInSubtree: an idle Codex shows as `pane → -bash → node …/codex`.
// Walking the pane's process subtree must find it.
func TestAgentInSubtree(t *testing.T) {
	procs := map[int]procInfo{
		100: {ppid: 1, command: "-bash"},                                // the pane shell
		101: {ppid: 100, command: "node /Users/ccy/.nvm/.../bin/codex"}, // codex CLI (child)
		200: {ppid: 1, command: "-bash"},                                // an unrelated shell
	}
	children := map[int][]int{1: {100, 200}, 100: {101}}

	if got := agentInSubtree(100, procs, children, builtinProfiles); got != "Codex" {
		t.Errorf("agentInSubtree(pane shell) = %q, want Codex", got)
	}
	if got := agentInSubtree(200, procs, children, builtinProfiles); got != "" {
		t.Errorf("agentInSubtree(plain shell) = %q, want empty", got)
	}
}
