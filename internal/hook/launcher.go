package hook

import (
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// paneLauncher reports the executable that actually started the agent in this pane, when
// it is NOT the agent's own name — a wrapper, an alias, an `npx`-style shim.
//
// An agent is not always launched by its own name, and a resume that assumes it was
// brings the conversation back as something else. Measured 2026-08-29: a Codex session
// started through an internal wrapper (`opencrab`), which supplies the company's
// configuration; a resume built from the registry alone would have run a bare `codex`,
// without any of it. No registry can enumerate the wrappers a real environment puts in
// front of an agent, so the launch itself is what gets recorded.
//
// Returns "" whenever it cannot tell, and "" is the honest answer: Command then falls
// back to the registry form, which is exactly today's behaviour.
func paneLauncher(pane, agentKey string) string {
	if pane == "" || tmux.Bin == "" {
		return ""
	}
	return launcherFrom(tmux.Display(pane, "#{pane_current_command}"), agentKey)
}

// launcherFrom is the decision itself, kept free of tmux so the rules are testable as
// rules — and so a test can prove the rules are actually APPLIED, which a test of the
// helpers alone cannot (removing the version guard left every helper test green).
func launcherFrom(cmd, agentKey string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	base := filepath.Base(cmd)
	// A shell is the pane's own process, not a launcher — recording "bash" would resume
	// the conversation by running bash.
	if isShellComm(base) {
		return ""
	}
	// The agent under its own name needs no record: that IS the registry form.
	if known, ok := agents.CommandKeys()[base]; ok && known == agentKey {
		return ""
	}
	// A Claude pane reports its VERSION as the command ("2.1.250"). That is not a
	// command anyone can run, and recording it would turn a resumable conversation into
	// a broken one — the mirror image of the bug this exists to fix.
	if looksLikeVersion(base) {
		return ""
	}
	return base
}

// looksLikeVersion reports whether a command name is really a version string — what tmux
// reports for a Claude pane (`pane_current_command` = "2.1.250"). Running it would fail;
// recording it as a launcher would turn a resumable conversation into a broken command.
func looksLikeVersion(s string) bool {
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

// launcherFor decides what to record as this pane's launcher for a given event.
//
// The pane's foreground command names the launcher only while the agent itself holds
// the terminal. Read mid-turn, with a tool in the foreground, it could name `node` or
// `git` — and restore would then type a command that cannot run, which is the same
// class of damage this feature exists to undo. So it is read only at the events where
// nothing else can be running: the prompt the user just submitted, and the session
// coming up. Every other event carries the last reading forward rather than
// overwriting a known launcher with a guess.
func launcherFor(loc, pane, agentKey, event string) string {
	switch event {
	case "UserPromptSubmit", "SessionStart", "Resumed":
		// Authoritative, including the empty answer: an agent restarted under its own
		// name in a pane that once ran a wrapper must stop resuming through it.
		return paneLauncher(pane, agentKey)
	}
	prev, _ := resume.Load(loc)
	return prev.Launcher
}
