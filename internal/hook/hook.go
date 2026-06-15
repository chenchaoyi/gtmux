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
	"time"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

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

// Run executes one hook invocation. stdin carries Claude Code's hook JSON; it is
// fully drained (an unread pipe can block the caller) and parsed for
// hook_event_name. Always returns 0 — a hook must never fail the agent turn.
func Run(stdin io.Reader) int {
	raw, _ := io.ReadAll(stdin) // drain the pipe regardless of what we do next
	var payload struct {
		HookEventName string `json:"hook_event_name"`
	}
	_ = json.Unmarshal(raw, &payload)
	event := payload.HookEventName

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
	debugf("event=%s pane=%q session=%q active=%v notify=%v", event, pane, session, activePresent, d.notify)

	if !d.notify {
		return 0
	}
	if session != "" && ghostty.IsViewing(session) {
		debugf("suppressed: already viewing session=%q", session)
		return 0
	}

	icon := ""
	if state.Exists(state.IconPath()) {
		icon = state.IconPath()
	}
	group := "gtmux"
	if session != "" {
		group = "gtmux-" + session
	}
	notify.Send(notify.Options{
		Title:    "Claude Code",
		Subtitle: session,
		Message:  i18n.Tr("Agent finished — click to open", "Agent 结束 —— 点击打开"),
		Activate: "com.gtmux.focus",
		Group:    group,
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
