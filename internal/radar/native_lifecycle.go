package radar

import (
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

var sharedCodexBackend = native.SharedCodexBackend

// A delayed tool-finish callback can postdate its own completion. The timestamp
// alone must not revive an old managed worker after its exact frontend is gone.
// The caller separately verifies shared-backend role and applies follow policy.
func closedFrontendTurn(scope *transcript.CodexPathScope, r native.Record, now int64) (int64, bool) {
	if r.Agent != "codex" || r.State != "working" || r.Event != "Resumed" || !nativeReadableScope(r) ||
		!native.ClosedFrontendObservation(r) {
		return 0, false
	}
	reader := transcript.CodexLastTurnEvent
	if scope != nil {
		reader = scope.CodexLastTurnEvent
	}
	e := reader(r.SessionID)
	if e.Ambiguous || e.At.IsZero() || e.At.After(time.Unix(now+1, 0)) || e.At.Unix() > r.UpdatedAt ||
		(e.Kind != "task_complete" && e.Kind != "turn_aborted") || (r.TurnID != "" && r.TurnID != e.TurnID) {
		return 0, false
	}
	return e.At.Unix(), true
}
