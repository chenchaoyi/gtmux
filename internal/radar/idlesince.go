package radar

import (
	"os"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// idleSince keeps redraws out of idle ordering. A known Codex terminal boundary
// can also correct a marker an older reader populated from settings telemetry.
// Correction changes only the cached time, never latest-completed or lifecycle.
func idleSince(pane, loc, cwd, agent string, activityAt int64, scope *transcript.CodexPathScope) int64 {
	path := state.FinishedPath(pane)
	marked := FileMtime(path)
	fallback := activityAt
	if rec, ok := boundResume(loc, agent); ok && (rec.Agent != "codex" || hqpane.SameDir(rec.Cwd, cwd)) {
		last := scopedLastMessageTime(scope, rec.Agent, rec.SessionID)
		if rec.Agent == "codex" {
			// A known rollout without supported activity must not inherit a fresh
			// window repaint. Leave its time unknown if no prior marker exists.
			fallback = last
			if !state.Exists(state.ActivePath(pane)) && !state.Exists(state.WaitingPath(pane)) {
				boundary := transcript.CodexLastTurnBoundary
				if scope != nil {
					boundary = scope.CodexLastTurnBoundary
				}
				kind, at := boundary(rec.SessionID)
				if (kind == "task_complete" || kind == "turn_aborted") && at.Unix() > 0 && last <= at.Unix() {
					fallback = at.Unix()
					if marked != fallback {
						stampIdleTime(path, fallback)
					}
					return fallback
				}
			}
		} else if last > 0 {
			fallback = last
		}
	}
	if marked > 0 {
		return marked
	}
	if fallback > 0 {
		stampIdleTime(path, fallback)
	}
	return fallback
}

func stampIdleTime(path string, at int64) {
	if state.Touch(path) == nil {
		_ = os.Chtimes(path, time.Unix(at, 0), time.Unix(at, 0))
	}
}
