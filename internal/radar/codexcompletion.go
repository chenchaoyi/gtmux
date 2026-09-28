package radar

import (
	"os"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// codexTurnCompleted is a narrow fallback when a Codex Stop hook misses its
// pane. The rollout belongs to the pane's bound conversation, and completion
// must be newer than the current turn/wait markers. A later task_started keeps
// an old completion from ending the next turn.
func codexTurnCompleted(pane, loc, cwd string, boundary func(string) (string, time.Time)) (time.Time, bool) {
	rec, ok := resume.Load(loc)
	if !ok || rec.Agent != "codex" || rec.SessionID == "" || !hqpane.SameDir(rec.Cwd, cwd) {
		return time.Time{}, false
	}
	active, err := os.Stat(state.ActivePath(pane))
	if err != nil {
		return time.Time{}, false
	}
	// Codex can omit the session id on UserPromptSubmit. The resulting plain
	// marker still belongs to this turn if the bound rollout completed after it.
	// A marker naming a different session must never be cleared here.
	if sid := state.ReadMarker(state.ActivePath(pane)); sid != "" && sid != rec.SessionID {
		return time.Time{}, false
	}
	kind, at := boundary(rec.SessionID)
	if kind != "task_complete" || at.IsZero() || at.After(time.Now().Add(time.Second)) || !at.After(active.ModTime()) {
		return time.Time{}, false
	}
	if wait, err := os.Stat(state.WaitingPath(pane)); err == nil && !at.After(wait.ModTime()) {
		return time.Time{}, false
	}
	return at, true
}
