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
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// agentDisplay lists the coding agents gtmux recognizes at hook time, mapping
// `gtmux hook --agent <key>` to a notification display name. An agent absent here
// is a no-op (preserves "unknown agent does nothing"). This is just the
// known-agent gate + display name; per-agent EVENT SEMANTICS live in classify.go
// (each agent's own table, else the generic table). Install support for each is
// in install-hooks.
var agentDisplay = map[string]string{
	"claude":       "Claude Code",
	"codex":        "Codex",
	"gemini":       "Gemini",
	"cursor":       "Cursor",
	"opencode":     "opencode",
	"copilot":      "Copilot",
	"hermes-agent": "Hermes",
	"kiro":         "Kiro",
}

// extractEvent returns the raw event from a positional hook arg: the token
// itself, or — when it's a JSON object (e.g. Codex's notify payload) — its
// "type" field.
func extractEvent(token string) string {
	if t := strings.TrimSpace(token); strings.HasPrefix(t, "{") {
		var p struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(t), &p) == nil && p.Type != "" {
			return p.Type
		}
	}
	return token
}

// extractResumeFields tolerantly pulls a resumable session id and cwd from a hook
// payload whose key names vary by agent (Claude uses session_id/cwd; others use
// conversation_id, working_directory, …). Best-effort: returns "" for anything missing.
func extractResumeFields(raw []byte) (sid, cwd string) {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return "", ""
	}
	for _, k := range []string{"session_id", "sessionId", "conversation_id", "conversationId"} {
		if s, ok := m[k].(string); ok && s != "" {
			sid = s
			break
		}
	}
	for _, k := range []string{"cwd", "working_directory", "project_dir", "projectDir"} {
		if s, ok := m[k].(string); ok && s != "" {
			cwd = s
			break
		}
	}
	return sid, cwd
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
//   - Waiting           → the classifier confirmed an approval/plan/question is
//     pending (a side-effecting tool, ExitPlanMode, AskUserQuestion, …), so mark
//     waiting unconditionally and notify; the turn is still in progress.
//   - Resumed           → that pending wait was just answered (the plan/question
//     tool finished, the approval was responded to), so clear waiting silently;
//     the turn is still in progress, so active is untouched and we don't notify.
func decide(event string, activePresent bool) decision {
	switch event {
	case "UserPromptSubmit":
		return decision{setActive: true, clearWaiting: true}
	case "Stop":
		return decision{clearActive: true, clearWaiting: true, setLastFinished: true, notify: true}
	case "Notification":
		return decision{setWaiting: activePresent, setLastFinished: true, notify: true}
	case "Waiting":
		return decision{setWaiting: true, notify: true}
	case "Resumed":
		// A pending plan/question/approval was answered → the agent is working
		// again. Clear the wait silently; the turn is still in progress, so don't
		// touch active or notify.
		return decision{clearWaiting: true}
	case "SessionStart", "SessionEnd":
		// A session (re)starting (startup/resume/clear/compact) or ending voids this
		// pane's turn state. Clear active + waiting so a marker orphaned by a prior
		// session — or by a pane id reused across a tmux restart — can't linger as a
		// phantom "working"/"needs you". No notify; the next UserPromptSubmit re-arms.
		return decision{clearActive: true, clearWaiting: true}
	default:
		return decision{}
	}
}

// waitBody is the notification body for a "needs you" event, tailored to the
// kind of wait (permission / plan / question), else a generic prompt.
func waitBody(k Kind) string {
	switch k {
	case KindPlan:
		return i18n.Tr("Plan ready — tap to review", "计划已就绪，点按查看")
	case KindQuestion:
		return i18n.Tr("A question for you — tap to answer", "有个问题等你，点按回答")
	case KindPermission:
		return i18n.Tr("Needs your approval — tap to jump", "需要你批准，点按跳转")
	default:
		return i18n.Tr("Needs your input — tap to jump", "需要你的输入，点按跳转")
	}
}

// staleStop reports whether a Stop is from a SUPERSEDED session — a newer agent
// session now owns the pane — and must be ignored. It fires only when BOTH the
// recorded active session and the event's session are known, so the pre-session
// behavior (and agents that send no session id) is unchanged.
func staleStop(event, activeSession, eventSession string) bool {
	return event == "Stop" && activeSession != "" && eventSession != "" && activeSession != eventSession
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
				rawEvent = extractEvent(args[i])
			}
		}
	}
	var payload struct {
		HookEventName    string `json:"hook_event_name"`
		SessionID        string `json:"session_id"`        // the agent's session id (Claude); "" otherwise
		ToolName         string `json:"tool_name"`         // the tool a PreToolUse refers to (Claude)
		Cwd              string `json:"cwd"`               // the agent's working dir (Claude)
		NotificationType string `json:"notification_type"` // Claude Notification kind (permission_prompt / idle_prompt / …)
	}
	_ = json.Unmarshal(raw, &payload)
	if rawEvent == "" {
		rawEvent = payload.HookEventName
	}
	agentSession := payload.SessionID
	resumeCwd := payload.Cwd
	if agentSession == "" || resumeCwd == "" {
		// Other agents key the id/cwd differently — fall back to a tolerant scan.
		sid, cwd := extractResumeFields(raw)
		if agentSession == "" {
			agentSession = sid
		}
		if resumeCwd == "" {
			resumeCwd = cwd
		}
	}

	display, known := agentDisplay[agentKey]
	if !known {
		return 0 // unknown agent → no-op
	}
	// Classify the raw event into a canonical lifecycle + (for Waiting) a kind.
	class := classify(agentKey, rawEvent, payload.ToolName)
	event := class.Lifecycle
	waitKind := class.Kind
	// Claude's Notification carries a notification_type telling us exactly what it
	// is — so we don't guess from timing. Route the "needs you" kinds to Waiting and
	// let everything else (idle nudge, auth success, completion) stay telemetry. When
	// the field is absent (older Claude), fall back to the legacy mid-turn heuristic
	// (decide() only raises waiting when a turn is active).
	if agentKey == "claude" && rawEvent == "Notification" {
		switch payload.NotificationType {
		case "permission_prompt", "agent_needs_input":
			event, waitKind = "Notification", KindPermission
		case "elicitation_dialog":
			event, waitKind = "Notification", KindQuestion
		case "":
			event, waitKind = "Notification", KindPermission // no type → legacy heuristic
		default:
			event = "" // idle_prompt / auth_success / agent_completed / … → not a wait
		}
	}
	if event == "" {
		debugf("telemetry no-op: agent=%s raw=%q tool=%q", agentKey, rawEvent, payload.ToolName)
		return 0
	}

	// The pane id ($TMUX_PANE, e.g. %12) is the state key. Outside tmux we can't
	// key state or name the session — degrade to a generic, state-less notify.
	pane := os.Getenv("TMUX_PANE")
	session := ""
	if pane != "" {
		session = tmux.Display(pane, "#{session_name}")
	}

	// Capture a resumable session for `restore` (#4): if we know the agent's
	// session id and a stable tmux locator, persist {agent, id, cwd} so a reboot
	// can relaunch the conversation. Keyed by session:window.pane — the same
	// coordinates tmux-resurrect restores by — so the record matches post-reboot.
	if pane != "" && resume.Resumable(agentKey) {
		if loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}"); loc != "" {
			cwd := resumeCwd
			if cwd == "" {
				cwd = tmux.Display(pane, "#{pane_current_path}")
			}
			sid := agentSession
			if sid == "" && agentKey == "codex" {
				// Codex's notify carries no conversation id — derive it from the
				// on-disk rollout whose session_meta.cwd is this pane's dir. Without
				// this, restore couldn't resume Codex and its chat mode stayed empty.
				sid, _ = transcript.CodexSessionForCwd(cwd)
			}
			if sid != "" {
				_ = resume.Save(loc, resume.Record{
					Agent: agentKey, SessionID: sid, Cwd: cwd, UpdatedAt: time.Now().Unix(),
				})
			}
		}
	}

	// A Stop from a SUPERSEDED agent session (a newer session now owns this pane,
	// e.g. after /clear or a relaunch) must not clear the current session's state
	// or notify — it's a late, out-of-order hook (cmux issue #5908).
	if pane != "" && staleStop(event, state.ReadMarker(state.ActivePath(pane)), agentSession) {
		debugf("ignored stale Stop: agent=%s event-session=%q active-session=%q",
			agentKey, agentSession, state.ReadMarker(state.ActivePath(pane)))
		return 0
	}

	activePresent := pane != "" && state.Exists(state.ActivePath(pane))
	priorWaitKind := ""
	if pane != "" {
		priorWaitKind = state.ReadMarker(state.WaitingPath(pane))
	}
	d := decide(event, activePresent)
	if pane != "" {
		applyState(d, pane)
		// Record the agent session on the active marker so a later superseded Stop
		// can be told apart (#5908). No session id → plain marker, unchanged.
		if d.setActive && agentSession != "" {
			_ = state.WriteMarker(state.ActivePath(pane), agentSession)
		}
		// The waiting marker carries the KIND (permission/plan/question) so the
		// radar + notification can say what's actually needed.
		if d.setWaiting {
			_ = state.WriteMarker(state.WaitingPath(pane), string(waitKind))
		}
	}
	// Don't re-notify a Waiting already flagged with the same kind — a generic
	// agent's PreToolUse can re-fire while you're still deciding.
	if event == "Waiting" && waitKind != "" && priorWaitKind == string(waitKind) {
		d.notify = false
	}
	debugf("agent=%s raw=%q event=%s kind=%q pane=%q session=%q agent-session=%q active=%v notify=%v",
		agentKey, rawEvent, event, waitKind, pane, session, agentSession, activePresent, d.notify)

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
	body := i18n.Tr("Finished — tap to jump", "已完成，点按跳转")
	if event == "Notification" || event == "Waiting" {
		kind = "input"
		body = waitBody(waitKind)
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
