package agents

import (
	"path/filepath"
	"strings"
)

// ShellProcess names launch infrastructure, not a resumable agent recipe.
func ShellProcess(comm string) bool {
	switch strings.TrimPrefix(comm, "-") {
	case "sh", "bash", "zsh", "dash", "fish", "gtmux", "login", "env", "tmux":
		return true
	}
	return false
}

// LauncherFromCommand records the existing generic launcher convention, excluding
// the agent itself, shells and version strings. It is never control authority.
func LauncherFromCommand(cmd, agentKey string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	base := filepath.Base(cmd)
	// A shell is the pane's own process, not a launcher — recording "bash" would resume
	// the conversation by running bash.
	if ShellProcess(base) {
		return ""
	}
	// The agent under its own name needs no record: that IS the registry form.
	if known, ok := CommandKeys()[base]; ok && known == agentKey {
		return ""
	}
	// A Claude pane reports its VERSION as the command ("2.1.250"). That is not a
	// command anyone can run, and recording it would turn a resumable conversation into
	// a broken one — the mirror image of the bug this exists to fix.
	if LooksLikeVersion(base) {
		return ""
	}
	return base
}

// LooksLikeVersion reports whether a command name is really a version string — what tmux
// reports for a Claude pane (`pane_current_command` = "2.1.250"). Running it would fail;
// recording it as a launcher would turn a resumable conversation into a broken command.
func LooksLikeVersion(s string) bool {
	if s == "" {
		return false
	}
	dots := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '.':
			dots++
		default:
			return false
		}
	}
	return dots > 0
}
