package hook

import (
	"os"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"strconv"
)

type codexPane struct {
	id, command, cwd, loc string
}

type codexActive struct {
	sessionID string
	since     time.Time
}

func codexBoundSessions(panes []codexPane) map[string]string {
	bound := make(map[string]string)
	for _, p := range panes {
		if p.loc == "" {
			continue
		}
		r, ok := resume.LoadLive(p.loc)
		if ok && r.Agent == "codex" && r.SessionID != "" && resume.CodexCwdMatches(p.id, r, p.cwd) {
			bound[p.id] = r.SessionID
		}
	}
	return bound
}

func codexActiveSessions(panes []codexPane) map[string]codexActive {
	active := make(map[string]codexActive)
	for _, p := range panes {
		if info, err := os.Stat(state.ActivePath(p.id)); err == nil {
			active[p.id] = codexActive{state.ReadMarker(state.ActivePath(p.id)), info.ModTime()}
		}
	}
	return active
}

// codexStopPane never trusts TMUX_PANE alone. A shared Codex app-server may hand
// a Stop hook the first client's inherited pane and omit the session/cwd fields.
// In that case, only one bound, active session with a just-completed rollout can
// claim the event. Ambiguity leaves the event pane-less instead of ending another
// pane's turn.
func codexStopPane(sessionID string, panes []codexPane, bound map[string]string, active map[string]codexActive,
	completed func(string) (string, time.Time), now time.Time) (pane, resolvedSession string) {
	return codexEndPane("task_complete", sessionID, panes, bound, active, completed, now)
}

func codexEndPane(expected, sessionID string, panes []codexPane, bound map[string]string, active map[string]codexActive,
	completed func(string) (string, time.Time), now time.Time) (pane, resolvedSession string) {
	for _, p := range panes {
		if p.command != "codex" {
			continue
		}
		sid := bound[p.id]
		turn, hasTurn := active[p.id]
		if sid == "" || !hasTurn || (turn.sessionID != "" && turn.sessionID != sid) ||
			(sessionID != "" && sid != sessionID) {
			continue
		}
		if sessionID == "" || turn.sessionID == "" {
			kind, at := completed(sid)
			if expected == "" || kind != expected || at.IsZero() || !at.After(turn.since) ||
				at.After(now.Add(time.Second)) || now.Sub(at) > 5*time.Second {
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
var resolveWrappedCodex = codexterminal.Resolve

func codexPanes() []codexPane {
	lines := tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{pane_current_command}\t#{pane_current_path}\t#{session_name}:#{window_index}.#{pane_index}\t#{pane_pid}\t#{pane_tty}")
	var panes []codexPane
	var wrapped []codexterminal.Pane
	for _, line := range lines {
		f := strings.Split(line, "\t")
		if len(f) < 4 {
			continue
		}
		panes = append(panes, codexPane{f[0], f[1], f[2], f[3]})
		if f[1] != "codex" && len(f) == 6 {
			pid, _ := strconv.Atoi(f[4])
			wrapped = append(wrapped, codexterminal.Pane{ID: f[0], Command: f[1], PID: pid, TTY: f[5], Cwd: f[2], Loc: f[3]})
		}
	}
	// The resolvers below retain all existing session/cwd guards. A generic
	// launcher becomes a Codex candidate only through live terminal evidence.
	verified := resolveWrappedCodex(wrapped)
	for i := range panes {
		if _, ok := verified[panes[i].id]; ok {
			panes[i].command = "codex"
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

var codexHookFrontend = resume.CodexFrontendForThread
