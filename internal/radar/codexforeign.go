package radar

import (
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

var foreignCodexFrontend = resume.CodexFrontendForPane

// A named marker for another thread is not evidence that this bound thread
// started work. Correct only an explicitly ended, visibly ready current thread;
// keep unknown/legacy ownership and genuine work or approval conservative.
// This is a display correction, never a marker deletion or completion event.
func codexForeignActiveEndedTurn(pane, loc, cwd, frame string, boundary func(string) (string, time.Time)) (string, time.Time) {
	other := state.ReadMarker(state.ActivePath(pane))
	if other == "" || state.Exists(state.WaitingPath(pane)) || !prompt.IsComposerReady(frame, "codex") || codexScreenWorking(frame) {
		return "", time.Time{}
	}
	rec, ok := foreignCodexFrontend(pane)
	// A legacy saved resume row alone is not exclusive current-thread proof.
	// The production reader rechecks the frontend's kernel/TTY/pane identity.
	if !ok || rec.Pane != pane || rec.Loc != loc || rec.HomeID != transcript.CodexHomeID() ||
		rec.SessionID == "" || rec.SessionID == other || !hqpane.SameDir(rec.Cwd, cwd) {
		return "", time.Time{}
	}
	kind, at := boundary(rec.SessionID)
	if (kind == "task_complete" || kind == "turn_aborted") && !at.IsZero() && !at.After(time.Now().Add(time.Second)) {
		return kind, at
	}
	return "", time.Time{}
}
