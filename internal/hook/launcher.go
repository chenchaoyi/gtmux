package hook

import (
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// paneLauncher reports the executable that actually started the agent in this pane, when
// it is NOT the agent's own name — a wrapper, an alias, an `npx`-style shim.
//
// An agent is not always launched by its own name, and a resume that assumes it was
// brings the conversation back as something else. Measured 2026-08-29: a Codex session
// started through an internal wrapper (`agentwrap`), which supplies the company's
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
func launcherFrom(cmd, agentKey string) string { return agents.LauncherFromCommand(cmd, agentKey) }
func looksLikeVersion(s string) bool           { return agents.LooksLikeVersion(s) }

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
