package hq

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// watchdogWaitTimeout: a pane that has been WAITING (needs the user) this long without
// being resolved escalates to HQ once (the roadmap's needs-you age/timeout). Seconds.
const watchdogWaitTimeout int64 = 10 * 60

func watchdogMarker(pane string) string { return filepath.Join(state.Dir(), "watchdog", pane) }

// watchdogSweep escalates, to a live HQ, any pane stuck WAITING past the timeout — the
// lifecycle watchdog (charter M5). It runs from the single-writer serve slow-tick, is
// suggest-only, and fires ONCE per waiting episode: a presence marker dedups within an
// episode and is cleared when the pane leaves waiting, so a fresh wait re-arms. It never
// escalates about the HQ pane itself.
func watchdogSweep(now int64) {
	hq := hqpane.Find()
	for _, p := range radar.GatherAgents() {
		if p.Status != "waiting" || p.PaneID == hq {
			state.Remove(watchdogMarker(p.PaneID)) // episode over / self → re-arm
			continue
		}
		if hq == "" {
			continue // nowhere to escalate; don't arm (so HQ, once live, can surface it)
		}
		since := waitingSince(p.PaneID)
		fired := state.Exists(watchdogMarker(p.PaneID))
		if !shouldEscalate(p.Status, since, now, watchdogWaitTimeout, fired) {
			continue
		}
		_ = state.Touch(watchdogMarker(p.PaneID))
		mins := (now - since) / 60
		// The line was hand-built in the pre-hq-perception-v2 format for months: the
		// delivery was always draft-guarded, but HQ received a shape its playbook does
		// not teach, and PriorityOf could not read a class out of it — so an escalation
		// queued as a default-priority outcome.
		msg := hqwake.Line(hqwake.ClassStuckWaiting, fmt.Sprintf("%s (%s)", p.Loc, p.PaneID),
			fmt.Sprintf("waited %dm, still needs you", mins))
		hqnudge.Deliver(hq, msg) // draft/copy-mode-guarded like every HQ injection
	}
}

// shouldEscalate is the pure decision: a pane waiting past the timeout that hasn't
// already escalated this episode. Extracted for testability.
func shouldEscalate(status string, sinceWait, now, timeout int64, alreadyFired bool) bool {
	return status == "waiting" && sinceWait > 0 && now-sinceWait >= timeout && !alreadyFired
}

// waitingSince returns when a pane's wait began (its waiting marker mtime), 0 if none.
func waitingSince(pane string) int64 {
	fi, err := os.Stat(state.WaitingPath(pane))
	if err != nil {
		return 0
	}
	return fi.ModTime().Unix()
}
