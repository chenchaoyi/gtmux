package app

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// pollInterval is how often the deliver-verify loop re-reads the pane/stream.
const pollInterval = 300 * time.Millisecond

// hookAgents are the agents that install gtmux hooks (so their submissions land on
// the session-events stream). For these, dispatch verify prefers the deterministic
// event; others fall to the hardened screen-read. Keyed by launch-command basename.
var hookAgents = map[string]bool{
	"claude": true, "codex": true, "gemini": true, "cursor": true,
	"cursor-agent": true, "opencode": true, "copilot": true, "kiro": true,
}

// hookEquipped reports whether an agent launch command is one whose hooks feed the
// event stream (so the primary verify path applies).
func hookEquipped(agentCmd string) bool {
	f := strings.Fields(agentCmd)
	if len(f) == 0 {
		return false
	}
	return hookAgents[filepath.Base(f[0])]
}

// eventsForPane maps recent session-events for a pane (Ts >= sinceTs) into the
// reduced form dispatch verify consumes.
func eventsForPane(pane string, sinceTs int64) []dispatch.Ev {
	now := time.Now().Unix()
	win := now - sinceTs + 2
	if win < 1 {
		win = 1
	}
	var out []dispatch.Ev
	for _, r := range events.Read(win, now) {
		if r.Pane != pane || r.Ts < sinceTs {
			continue
		}
		var kind string
		switch r.Event {
		case "UserPromptSubmit":
			kind = dispatch.EvSubmit
		case "Stop":
			kind = dispatch.EvStop
		case "PreCompact":
			kind = dispatch.EvCompact
		default:
			continue
		}
		out = append(out, dispatch.Ev{Kind: kind, Head: r.Summary, Ts: r.Ts})
	}
	return out
}

// dispatchIO builds the live tmux/events I/O for delivering to a pane.
func dispatchIO(pane string) dispatch.IO {
	return dispatch.IO{
		Capture:    func() string { return tmux.CaptureFull(pane) },
		Paste:      func(text string) error { return tmux.Paste(pane, text) },
		Enter:      func() error { return tmux.SendKey(pane, "Enter") },
		ClearDraft: func() error { return tmux.SendKey(pane, "C-u") },
		Events:     func(since int64) []dispatch.Ev { return eventsForPane(pane, since) },
		Now:        func() int64 { return time.Now().Unix() },
		Sleep:      func() { time.Sleep(pollInterval) },
		RecentSend: dispatch.RecentSend,
		RecordSend: dispatch.RecordSend,
	}
}

// deliverOpts builds the verify options for a pane + agent, applying tuning.
func deliverOpts(pane, agentCmd string, force bool, tune dispatch.Tuning) dispatch.Opts {
	return dispatch.Opts{
		Pane:           pane,
		HookEquipped:   hookEquipped(agentCmd),
		Force:          force,
		ResendWindow:   tune.ResendWindow,
		DeliverTimeout: tune.DeliverTimeout,
		HookGrace:      tune.HookGrace,
		PasteRetries:   2,
		EnterRetries:   3,
	}
}

// shellCommands are foreground commands that mean "still a bare shell, no agent yet"
// — used to tell when a launched agent has actually taken over the pane.
var shellCommands = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "fish": true,
	"-sh": true, "-bash": true, "-zsh": true, "login": true, "tmux": true,
}

// waitAgentReady polls a pane until its foreground command is no longer a bare
// shell (the launched agent has taken over) or the timeout lapses. Returns whether
// the agent came up.
func waitAgentReady(pane string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		cmd := tmux.Display(pane, "#{pane_current_command}")
		if cmd != "" && !shellCommands[cmd] {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}
