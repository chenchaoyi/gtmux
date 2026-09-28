package hook

import (
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

type codexPane struct {
	id, command, cwd, loc string
}

func codexBoundSessions(panes []codexPane) map[string]string {
	bound := make(map[string]string)
	for _, p := range panes {
		if p.loc == "" {
			continue
		}
		r, ok := resume.Load(p.loc)
		if ok && r.Agent == "codex" && r.SessionID != "" && hqpane.SameDir(r.Cwd, p.cwd) {
			bound[p.id] = r.SessionID
		}
	}
	return bound
}

func codexActiveSessions(panes []codexPane) map[string]string {
	active := make(map[string]string)
	for _, p := range panes {
		if sid := state.ReadMarker(state.ActivePath(p.id)); sid != "" {
			active[p.id] = sid
		}
	}
	return active
}

// codexStopPane never trusts TMUX_PANE alone. A shared Codex app-server may hand
// a Stop hook the first client's inherited pane and omit the session/cwd fields.
// In that case, only one bound, active session with a just-completed rollout can
// claim the event. Ambiguity leaves the event pane-less instead of ending another
// pane's turn.
func codexStopPane(sessionID string, panes []codexPane, bound, active map[string]string,
	completed func(string) (string, time.Time), now time.Time) (pane, resolvedSession string) {
	for _, p := range panes {
		if p.command != "codex" {
			continue
		}
		sid := bound[p.id]
		if sid == "" || active[p.id] != sid || (sessionID != "" && sid != sessionID) {
			continue
		}
		if sessionID == "" {
			kind, at := completed(sid)
			if kind != "task_complete" || at.IsZero() || at.After(now.Add(time.Second)) || now.Sub(at) > 5*time.Second {
				continue
			}
		}
		if pane != "" {
			return "", ""
		}
		pane, resolvedSession = p.id, sid
	}
	return pane, resolvedSession
}

// codexPanes collects the live clients in one tmux call. A hook's inherited
// TMUX_PANE can describe the app-server's first client rather than this session.
func codexPanes() []codexPane {
	lines := tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{pane_current_command}\t#{pane_current_path}\t#{session_name}:#{window_index}.#{pane_index}")
	var panes []codexPane
	for _, line := range lines {
		f := strings.SplitN(line, "\t", 4)
		if len(f) == 4 {
			panes = append(panes, codexPane{f[0], f[1], f[2], f[3]})
		}
	}
	return panes
}

// codexPaneForCwd accepts the inherited pane only when its live cwd agrees with
// the hook payload. Otherwise exactly one running Codex pane may claim that cwd.
// Ambiguity or a missing pane goes to the native path instead of corrupting an
// unrelated pane's state and delivery receipts.
func codexPaneForCwd(inherited, cwd, sessionID string, panes []codexPane, boundSessions map[string]string) string {
	if cwd == "" {
		return inherited
	}
	// A Codex app-server can inherit a different client's TMUX_PANE. The session id
	// from a hooks-system payload is stronger evidence than that inherited value, but
	// only when exactly one live pane's saved binding agrees with both session and cwd.
	if sessionID != "" {
		var candidate string
		for _, p := range panes {
			if p.command != "codex" || !hqpane.SameDir(p.cwd, cwd) || boundSessions[p.id] != sessionID {
				continue
			}
			if candidate != "" {
				return ""
			}
			candidate = p.id
		}
		if candidate != "" {
			return candidate
		}
	}
	// A same-directory peer makes the inherited pane ambiguous too. Accepting it
	// merely because its cwd agrees recreates the cross-pane corruption this resolver
	// exists to prevent.
	var cwdMatches []string
	for _, p := range panes {
		if p.command == "codex" && hqpane.SameDir(p.cwd, cwd) {
			cwdMatches = append(cwdMatches, p.id)
		}
	}
	if len(cwdMatches) > 1 {
		return ""
	}
	for _, p := range panes {
		if p.id == inherited && p.command == "codex" && hqpane.SameDir(p.cwd, cwd) {
			return inherited
		}
	}
	if len(cwdMatches) == 1 {
		return cwdMatches[0]
	}
	return ""
}

// codexWaitingPane resolves a permission request that carries no cwd. A shared
// app-server can inherit another client's TMUX_PANE, so that value alone cannot
// identify the asking pane. A visible menu is not hook ownership proof either:
// another pane's earlier request might be visible before this one has drawn.
// Only a unique session binding may claim the hook. Radar later senses the live
// menu directly in its own pane when the hook carries no identity.
func codexWaitingPane(sessionID string, panes []codexPane, bound map[string]string) string {
	if sessionID == "" {
		return ""
	}
	var candidate string
	for _, p := range panes {
		if p.command != "codex" || bound[p.id] != sessionID {
			continue
		}
		if candidate != "" {
			return ""
		}
		candidate = p.id
	}
	return candidate
}
