package dispatchbridge

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
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
		Capture:      func() string { return tmux.CaptureFull(pane) },
		CaptureColor: func() string { return tmux.CaptureFullColor(pane) },
		Paste:        func(text string) error { return tmux.Paste(pane, text) },
		Enter:        func() error { return tmux.SendKey(pane, "Enter") },
		ClearDraft:   func() error { return tmux.SendKey(pane, "C-u") },
		InMode:       func() bool { return tmux.InMode(pane) },
		ExitMode:     func() error { return tmux.ExitCopyMode(pane) },
		Events:       func(since int64) []dispatch.Ev { return eventsForPane(pane, since) },
		Now:          func() int64 { return time.Now().Unix() },
		Sleep:        func() { time.Sleep(pollInterval) },
		RecentSend:   dispatch.RecentSend,
		RecordSend:   dispatch.RecordSend,
		ForgetSend:   dispatch.ForgetSend,
	}
}

// DeliverOpts builds the verify options for a pane + agent, applying tuning.
func DeliverOpts(pane, agentCmd string, force bool, tune dispatch.Tuning) dispatch.Opts {
	return dispatch.Opts{
		Pane:         pane,
		HookEquipped: hookEquipped(agentCmd),
		Force:        force,
		// The operator's `--force` means "send it anyway", which covers BOTH refusals:
		// the re-send interlock and the draft guard. The phone's force (it always carries
		// a sendID) never reaches here — serve builds its own Opts — so a remote send
		// cannot waive draft protection.
		ClobberDraft: force,
		// The draft guard applies only where a draft MEANS something: a pane a known agent
		// drives. Routing fails safe to this pipeline for any non-shell pane (vim, ssh, a
		// TUI app), and those have no composer for the guard to read.
		HasComposer:    knownAgent(agentCmd),
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
//
// When the agent's driver provides the session-start signal (Ready), the settle
// requirement may SHORT-CIRCUIT: once the event proves the session came up, ONE
// input-ready capture suffices — a slow-settling boot (MCP noise churning the
// screen) no longer runs out the clock waiting for two identical frames. The
// event's ABSENCE changes nothing (I2): the full two-frame gate applies unchanged.
func WaitAgentReady(pane, agentCmd string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	g := readyGate{agent: agentKey(agentCmd), sessionUp: driverReady(pane, agentKey(agentCmd))}
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

// ReadyBlocker names what kept a pane from becoming ready — read AFTER WaitAgentReady
// returned false, so a caller's failure report can say WHICH line said no instead of
// only "not ready within the timeout". That distinction is the whole difference between
// "the agent is slow, retry" and "this machine carries a banner the gate can never
// satisfy": the latter was misread as the former for months, because the timeout message
// named nothing and the evidence was 200 lines of scrollback.
//
// It reads the pane ONCE (cheap, and only on the failure path) and returns that frame
// alongside the verdict, so a caller's evidence quotes the very capture the verdict was
// read from rather than a second, possibly different one.
//
// Two of the conditions are invisible to the pure screen predicate and are decided here:
// the agent never launched at all, and a composer that IS ready-looking on this frame —
// which means the gate died on the settle requirement, i.e. the screen kept repainting.
func ReadyBlocker(pane, agentCmd string) (blocker, capture string) {
	cmd := tmux.Display(pane, "#{pane_current_command}")
	if cmd == "" {
		return "the pane is gone", ""
	}
	if ShellCommands[cmd] {
		return "the agent never started — the pane is still a bare shell (" + cmd + ")", tmux.CaptureFull(pane)
	}
	capture = tmux.CaptureFull(pane)
	return readyBlockerOf(capture, agentKey(agentCmd)), capture
}

// readyBlockerOf is ReadyBlocker's pure half (the tmux-free part the tests drive), in
// the same shape as readyGate.step.
func readyBlockerOf(capture, agent string) string {
	if r := prompt.NotReadyReason(capture, agent); r != "" {
		return r
	}
	return "the composer never settled — the screen kept repainting through the timeout"
}

// driverReady wires the driver's session-start signal for the ready gate; nil when
// the capability is absent (hook-less agent) or switched off (`driver.<agent>.ready`).
// The launch moment is NOW (WaitAgentReady runs right after the launch keystroke),
// with a 1s margin so a same-second event is not missed.
func driverReady(pane, agent string) func() bool {
	d := driver.For(agent)
	if d.Ready == nil {
		return nil
	}
	since := time.Now().Unix() - 1
	return func() bool { return d.Ready(pane, since) }
}

// readyGate is the (pure, tmux-free) settle state machine WaitAgentReady drives: it
// reaches `launched` once the foreground command leaves the shell set, then reports
// ready when two CONSECUTIVE identical captures both satisfy IsComposerReady — or,
// when the driver's session-start signal has fired, on the FIRST ready capture (the
// event deterministically proves the session came up, so the settle wait is
// redundant; the gate/banner/prompt checks still apply to that one capture).
type readyGate struct {
	agent     string
	launched  bool
	prev      string
	sessionUp func() bool // driver session-start signal; nil → no short-circuit
}

// step advances the gate by one sample. cmd is the pane's foreground command; capture
// is called ONLY once launched (so an un-launched poll pays no capture cost). Returns
// true when the composer is ready and settled (or ready once, session-start proven —
// the signal is polled only on a ready-but-unsettled frame, so it costs nothing on
// the settled path, and the poll that returns true is also the one that returns).
func (g *readyGate) step(cmd string, capture func() string) bool {
	if !g.launched && cmd != "" && !ShellCommands[cmd] {
		g.launched = true
	}
	if !g.launched {
		return false
	}
	c := capture()
	if prompt.IsComposerReady(c, g.agent) &&
		(c == g.prev || (g.sessionUp != nil && g.sessionUp())) {
		return true
	}
	g.prev = c
	return false
}

// knownAgent reports whether a launch/foreground command names an agent in the registry —
// the fact that decides whether a pane has a composer whose draft is worth protecting.
func knownAgent(agentCmd string) bool {
	_, ok := agents.For(agentKey(agentCmd))
	return ok
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
