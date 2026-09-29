package radar

import (
	"os"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/prompt"
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

// codexCompletedTurnContradictsFrame rejects a frame/CPU-only "working" hint
// when the pane's own Codex rollout still ends at task_complete. A quota banner
// or other idle TUI repaint changes the screen without starting a turn; treating
// that repaint as a turn also emits a false "done" alert when it settles.
func codexCompletedTurnContradictsFrame(pane, loc, cwd string, boundary func(string) (string, time.Time)) bool {
	if state.Exists(state.ActivePath(pane)) || state.Exists(state.WaitingPath(pane)) {
		return false
	}
	rec, ok := resume.Load(loc)
	if !ok || rec.Agent != "codex" || rec.SessionID == "" || !hqpane.SameDir(rec.Cwd, cwd) {
		return false
	}
	kind, at := boundary(rec.SessionID)
	return kind == "task_complete" && !at.IsZero() && !at.After(time.Now().Add(time.Second))
}

// codexIdleComposerContradictsFrame covers panes with no current Codex resume
// binding yet. A new Codex TUI can sit at its empty composer before its first
// turn/hook, while a previous agent's resume record still names this location.
// Quota/warning banners repaint that idle screen and make frame sampling say
// "working" for a few seconds. Suppress only that screen-only hint; a hook turn
// marker, approval, or visible Codex work still takes precedence.
func codexIdleComposerContradictsFrame(pane, frame string) bool {
	if state.Exists(state.ActivePath(pane)) || state.Exists(state.WaitingPath(pane)) ||
		!prompt.IsComposerReady(frame, "codex") {
		return false
	}
	for _, line := range strings.Split(frame, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "esc to interrupt") ||
			strings.HasPrefix(line, "• Working (") ||
			strings.HasPrefix(line, "• Running ") ||
			strings.HasPrefix(line, "Messages to be submitted after next tool call") {
			return false
		}
	}
	return true
}
