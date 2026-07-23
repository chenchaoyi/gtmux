package dispatchbridge

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/driver"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// pollInterval is how often the deliver-verify loop re-reads the pane/stream.
const pollInterval = 300 * time.Millisecond

// hookEquipped reports whether an agent launch command has the delivery-receipt
// capability (its hooks feed the event stream, so the event-first verify path
// applies). Since P1 the fact is the driver registry's Receipt capability — which
// makes it switchable: `driver.<agent>.receipt: off` (or `driver.enable: off`)
// forces the pure Layer-1 screen-read verification for fault isolation.
func hookEquipped(agentCmd string) bool {
	return driver.For(agentKey(agentCmd)).Receipt != nil
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

// DispatchIO builds the live tmux/events I/O for delivering to a pane.
func DispatchIO(pane string) dispatch.IO {
	return dispatch.IO{
		Capture:    func() string { return tmux.CaptureFull(pane) },
		Paste:      func(text string) error { return tmux.Paste(pane, text) },
		Enter:      func() error { return tmux.SendKey(pane, "Enter") },
		ClearDraft: func() error { return tmux.SendKey(pane, "C-u") },
		InMode:     func() bool { return tmux.InMode(pane) },
		ExitMode:   func() error { return tmux.ExitCopyMode(pane) },
		Events:     func(since int64) []dispatch.Ev { return eventsForPane(pane, since) },
		Now:        func() int64 { return time.Now().Unix() },
		Sleep:      func() { time.Sleep(pollInterval) },
		RecentSend: dispatch.RecentSend,
		RecordSend: dispatch.RecordSend,
	}
}

// DeliverOpts builds the verify options for a pane + agent, applying tuning.
func DeliverOpts(pane, agentCmd string, force bool, tune dispatch.Tuning) dispatch.Opts {
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

// ShellCommands are foreground commands that mean "still a bare shell, no agent yet"
// — used to tell when a launched agent has actually taken over the pane.
var ShellCommands = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "fish": true,
	"-sh": true, "-bash": true, "-zsh": true, "login": true, "tmux": true,
}

// WaitAgentReady is the READY gate of the spawn delivery handshake
// (launched → ready → content-verified → submitted). It waits until a pane's composer
// is input-ready and SETTLED before the caller pastes a goal — process liveness alone
// is necessary but NOT sufficient, because a freshly launched agent's TUI is still
// painting a startup banner / trust gate / MCP-connect spinner, and pasting a long goal
// into that unstable window truncates the goal and swallows the Enter.
//
// It proceeds in two phases under one deadline:
//   - launched: the foreground command is no longer a bare shell (the agent took over);
//   - ready: two CONSECUTIVE identical captures both satisfy prompt.IsComposerReady
//     (prompt row present, no startup/trust gate, no boot banner) — the settle check
//     that guards against catching the composer between two repaints of the banner.
//
// Returns false on timeout so the caller reports failure with evidence and does NOT
// paste into a pane that never became ready. agentCmd is the launch command (its
// basename selects the per-agent readiness signatures; "" uses the default set).
func WaitAgentReady(pane, agentCmd string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	g := readyGate{agent: agentKey(agentCmd)}
	for {
		if g.step(tmux.Display(pane, "#{pane_current_command}"),
			func() string { return tmux.CaptureFull(pane) }) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

// readyGate is the (pure, tmux-free) settle state machine WaitAgentReady drives: it
// reaches `launched` once the foreground command leaves the shell set, then reports
// ready only when two CONSECUTIVE identical captures both satisfy IsComposerReady.
type readyGate struct {
	agent    string
	launched bool
	prev     string
}

// step advances the gate by one sample. cmd is the pane's foreground command; capture
// is called ONLY once launched (so an un-launched poll pays no capture cost). Returns
// true when the composer is ready and settled.
func (g *readyGate) step(cmd string, capture func() string) bool {
	if !g.launched && cmd != "" && !ShellCommands[cmd] {
		g.launched = true
	}
	if !g.launched {
		return false
	}
	c := capture()
	if prompt.IsComposerReady(c, g.agent) && c == g.prev {
		return true // ready AND settled (two identical ready captures)
	}
	g.prev = c
	return false
}

// agentKey reduces a launch command ("claude --model …") to the basename used to key
// the per-agent readiness signatures.
func agentKey(agentCmd string) string {
	f := strings.Fields(agentCmd)
	if len(f) == 0 {
		return ""
	}
	return filepath.Base(f[0])
}
