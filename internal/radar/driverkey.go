package radar

import (
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// AgentDriverKey resolves a pane's driver key ("claude"/"codex"/…) — the identity
// the dispatch layer keys hook-equipped off (and thus whether the receipt-backed
// verification applies to a send). It deliberately does NOT trust
// pane_current_command: Claude Code renames its process to its VERSION there (e.g.
// "2.1.220") and several agents run as bare "node", so the foreground command name
// misses the real agent. Instead it walks the pane's process subtree exactly like
// GatherAgents does — the same path that lets the radar identify these panes at all.
// Returns "" when no known agent runs under the pane (a plain shell, or ps failed).
func AgentDriverKey(paneID string) string {
	if paneID == "" {
		return ""
	}
	pidStr := strings.TrimSpace(tmux.Display(paneID, "#{pane_pid}"))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return ""
	}
	return driverKeyFromSubtree(pid, procSnapshot(), LoadProfiles())
}

// driverKeyFromSubtree is the pure core of AgentDriverKey: it walks pid's subtree in
// procs for a known agent and maps the profile's display name back to its driver key
// (the profile's first command). "" when nothing under pid is a known agent.
func driverKeyFromSubtree(pid int, procs map[int]procInfo, profiles []agentProfile) string {
	children := map[int][]int{}
	for p, info := range procs {
		children[info.ppid] = append(children[info.ppid], p)
	}
	name := agentInSubtree(pid, procs, children, profiles)
	if name == "" {
		return ""
	}
	for i := range profiles {
		if profiles[i].Name == name && len(profiles[i].Commands) > 0 {
			return profiles[i].Commands[0]
		}
	}
	return ""
}
