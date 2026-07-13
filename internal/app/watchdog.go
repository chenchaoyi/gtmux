package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/hqnudge"
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
	hq := findHQPane()
	for _, p := range gatherAgents() {
		if p.status != "waiting" || p.paneID == hq {
			state.Remove(watchdogMarker(p.paneID)) // episode over / self → re-arm
			continue
		}
		if hq == "" {
			continue // nowhere to escalate; don't arm (so HQ, once live, can surface it)
		}
		since := waitingSince(p.paneID)
		fired := state.Exists(watchdogMarker(p.paneID))
		if !shouldEscalate(p.status, since, now, watchdogWaitTimeout, fired) {
			continue
		}
		_ = state.Touch(watchdogMarker(p.paneID))
		mins := (now - since) / 60
		msg := fmt.Sprintf("[gtmux] stuck·waiting %s (%s) — waited %dm, still needs you", p.loc, p.paneID, mins)
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
