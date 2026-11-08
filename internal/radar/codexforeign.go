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

// Keep the legacy boundary adapter for callers that lack turn IDs. Such callers
// can only contradict a foreign marker, never an own marker with uncertain turns.
func codexForeignActiveEndedTurn(pane, loc, cwd, frame string, boundary func(string) (string, time.Time)) (string, time.Time) {
	return codexActiveEndedTurn(pane, loc, cwd, frame, func(sid string) transcript.CodexTurnEvent {
		kind, at := boundary(sid)
		return transcript.CodexTurnEvent{Kind: kind, At: at}
	})
}

// A named foreign marker, or a delayed marker for an explicitly ended own turn,
// cannot outweigh a verified current frontend and ready nonworking frame. One
// shared check reads the binding and bounded rollout metadata once. This only
// corrects display: markers and completion receipts are never written here.
func codexActiveEndedTurn(pane, loc, cwd, frame string, boundary func(string) transcript.CodexTurnEvent) (string, time.Time) {
	sid := state.ReadMarker(state.ActivePath(pane))
	if sid == "" || state.Exists(state.WaitingPath(pane)) ||
		!prompt.IsComposerReady(frame, "codex") || prompt.WaitingOptions(frame) != nil || codexScreenWorking(frame) {
		return "", time.Time{}
	}
	rec, ok := foreignCodexFrontend(pane)
	// A legacy saved resume row alone is not exclusive current-thread proof.
	// The production reader rechecks the frontend's kernel/TTY/pane identity.
	if !ok || rec.Pane != pane || rec.Loc != loc || rec.HomeID != transcript.CodexHomeID() ||
		rec.SessionID == "" || !hqpane.SameDir(rec.Cwd, cwd) {
		return "", time.Time{}
	}
	turn := state.ReadMarker(state.TurnIDPath(pane))
	event := boundary(rec.SessionID)
	if event.Ambiguous || (event.Kind != "task_complete" && event.Kind != "turn_aborted") ||
		event.At.IsZero() || event.At.After(time.Now().Add(time.Second)) {
		return "", time.Time{}
	}
	// Own markers require exact current turn proof, regardless of marker age.
	if rec.SessionID == sid && (turn == "" || event.TurnID != turn) {
		return "", time.Time{}
	}
	// A prompt accepted during the read wins over the older observation.
	if state.ReadMarker(state.TurnIDPath(pane)) != turn || state.ReadMarker(state.ActivePath(pane)) != sid || state.Exists(state.WaitingPath(pane)) {
		return "", time.Time{}
	}
	return event.Kind, event.At
}
