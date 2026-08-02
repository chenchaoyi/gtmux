package radar

import "testing"

// A Claude Code pane renames its foreground process to its version (e.g. "2.1.220"),
// so pane_current_command misses "claude". The driver key MUST still resolve via the
// process subtree, where the real argv "claude --resume …" lives — otherwise the send
// path treats a hook-equipped agent as hook-less and skips the receipt verification.
func TestDriverKeyFromSubtree_ClaudeRenamedProcess(t *testing.T) {
	profiles := LoadProfiles()
	// pane pid 100 (bash) → 200 (the claude process; comm reads "2.1.220" but its
	// snapshotProcs argv is the real "claude --resume …") → 300 (an MCP node child).
	procs := map[int]procInfo{
		100: {ppid: 1, command: "-/bin/bash"},
		200: {ppid: 100, command: "claude --resume 5ae5923b-f831"},
		300: {ppid: 200, command: "npm exec @playwright/mcp@latest"},
	}
	if got := driverKeyFromSubtree(100, procs, profiles); got != "claude" {
		t.Fatalf("driverKeyFromSubtree = %q, want \"claude\"", got)
	}
}

func TestDriverKeyFromSubtree_PlainShellIsEmpty(t *testing.T) {
	procs := map[int]procInfo{
		100: {ppid: 1, command: "-/bin/bash"},
		200: {ppid: 100, command: "vim notes.md"},
	}
	if got := driverKeyFromSubtree(100, procs, LoadProfiles()); got != "" {
		t.Fatalf("driverKeyFromSubtree = %q, want \"\" for a non-agent pane", got)
	}
}
