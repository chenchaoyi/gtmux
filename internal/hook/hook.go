// Package hook implements `gtmux hook`: the state producer + notifier that
// Claude Code runs on Stop / Notification / UserPromptSubmit. It transitions the
// on-disk markers in internal/state by event TIMING (never message keywords —
// keyword detection proved fragile and was removed) and fires a desktop
// notification, suppressed when you're already watching that session's tab.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// hookAgent describes how one coding agent's hook/notify events map onto gtmux's
// CANONICAL event vocabulary — which is just Claude's event names, since Claude
// is the reference that decide() is written against. Add an agent here (+ its
// installer in install-hooks) to give it ⏸ needs-input / ✓ done / notifications.
type hookAgent struct {
	display string            // notification subtitle, e.g. "Claude Code"
	events  map[string]string // the agent's raw event name → canonical event name
}

// hookAgents is keyed by `gtmux hook --agent <key>` (default "claude"). Claude is
// an identity map; e.g. Codex would map "turn-ended" → "Stop".
var hookAgents = map[string]hookAgent{
	"claude": {
		display: "Claude Code",
		events: map[string]string{
			"UserPromptSubmit": "UserPromptSubmit",
			"Stop":             "Stop",
			"Notification":     "Notification",
		},
	},
}

// canonicalEvent translates an agent's raw event to the canonical event decide()
// understands ("" when the agent or event is unknown → no-op), plus the agent's
// display name for the notification.
func canonicalEvent(agentKey, rawEvent string) (event, display string) {
	ag, ok := hookAgents[agentKey]
	if !ok {
		return "", ""
	}
	return ag.events[rawEvent], ag.display
}

// decision is what a hook event implies, independent of the filesystem. Keeping
// it pure makes the (event, active-marker?) → mutations mapping unit-testable.
type decision struct {
	setActive       bool // touch active/<pane>
	clearActive     bool // rm active/<pane>
	clearWaiting    bool // rm waiting/<pane>
	setWaiting      bool // touch waiting/<pane>
	setLastFinished bool // write <pane> to last-finished
	notify          bool // fire a desktop notification
}

// decide maps a hook event (and whether an active turn is in progress for the
// pane) to its side effects. This is the heart of the hook and the contract the
// workspace depends on:
//
//   - UserPromptSubmit → start a turn (active on, waiting off); state-only, no notify.
//   - Stop             → end a turn (active+waiting off), record last-finished, notify.
//   - Notification     → record last-finished, notify; mark waiting ONLY mid-turn
//     (active present). A Notification while idle is Claude's idle nudge, not a
//     real "blocked on you", so it must NOT set waiting.
func decide(event string, activePresent bool) decision {
	switch event {
	case "UserPromptSubmit":
		return decision{setActive: true, clearWaiting: true}
	case "Stop":
		return decision{clearActive: true, clearWaiting: true, setLastFinished: true, notify: true}
	case "Notification":
		return decision{setWaiting: activePresent, setLastFinished: true, notify: true}
	default:
		return decision{}
	}
}

// applyState performs a decision's filesystem mutations for pane.
func applyState(d decision, pane string) {
	if d.setActive {
		_ = state.Touch(state.ActivePath(pane))
	}
	if d.clearActive {
		state.Remove(state.ActivePath(pane))
	}
	if d.clearWaiting {
		state.Remove(state.WaitingPath(pane))
	}
	if d.setWaiting {
		_ = state.Touch(state.WaitingPath(pane))
	}
	if d.setLastFinished {
		_ = state.WriteLastFinished(pane)
	}
}

// Run executes one hook invocation. args come after `gtmux hook`:
//   - `--agent <key>` selects the agent (default "claude" — Claude's settings.json
//     calls `gtmux hook` with no args, so the default keeps it working unchanged).
//   - a positional token is the raw event name (e.g. Codex passes "turn-ended");
//     otherwise the event is read from stdin's JSON `hook_event_name` (Claude).
//
// stdin is always drained (an unread pipe can block the caller). Always returns 0
// — a hook must never fail the agent's turn.
func Run(stdin io.Reader, args []string) int {
	raw, _ := io.ReadAll(stdin) // drain the pipe regardless of what we do next

	agentKey := "claude"
	rawEvent := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 < len(args) {
				agentKey = args[i+1]
				i++
			}
		default:
			if rawEvent == "" && !strings.HasPrefix(args[i], "-") {
				rawEvent = args[i]
			}
		}
	}
	if rawEvent == "" {
		var payload struct {
			HookEventName string `json:"hook_event_name"`
		}
		_ = json.Unmarshal(raw, &payload)
		rawEvent = payload.HookEventName
	}
	event, display := canonicalEvent(agentKey, rawEvent)
	if event == "" {
		return 0 // unknown agent or unmapped event → no-op
	}

	// The pane id ($TMUX_PANE, e.g. %12) is the state key. Outside tmux we can't
	// key state or name the session — degrade to a generic, state-less notify.
	pane := os.Getenv("TMUX_PANE")
	session := ""
	if pane != "" {
		session = tmux.Display(pane, "#{session_name}")
	}

	activePresent := pane != "" && state.Exists(state.ActivePath(pane))
	d := decide(event, activePresent)
	if pane != "" {
		applyState(d, pane)
	}
	debugf("agent=%s raw=%q event=%s pane=%q session=%q active=%v notify=%v",
		agentKey, rawEvent, event, pane, session, activePresent, d.notify)

	if !d.notify {
		return 0
	}
	if session != "" && terminal.Active().IsViewing(session) {
		debugf("suppressed: already viewing session=%q", session)
		return 0
	}

	icon := ""
	if state.Exists(state.IconPath()) {
		icon = state.IconPath()
	}
	// Differentiate copy/sound: "finished" (calm) vs "needs your input" (urgent).
	// The session name is the bold title; the agent name is the subtitle.
	kind := "done"
	body := i18n.Tr("Finished — tap to jump", "已完成 —— 点按跳转")
	if event == "Notification" {
		kind = "input"
		body = i18n.Tr("Needs your input — tap to jump", "需要你的输入 —— 点按跳转")
	}
	title := session
	if title == "" {
		title = display
	}
	notify.Send(notify.Options{
		Kind:     kind,
		Title:    title,
		Subtitle: display,
		Message:  body,
		Pane:     pane,
		Session:  session,
		IconPath: icon,
	})
	return 0
}

// debugf appends a timestamped trace line when GTMUX_HOOK_DEBUG is set, so
// "why did/didn't it fire" stays diagnosable without rebuilding.
func debugf(format string, a ...any) {
	if os.Getenv("GTMUX_HOOK_DEBUG") == "" {
		return
	}
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(state.Dir(), "hook.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, a...)...)
}
