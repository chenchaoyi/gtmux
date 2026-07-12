// The serve slow-tick evaluator (resource-watch + limits-watch): the SINGLE
// place that samples machine resources + subscription limits and nudges a live
// gtmux HQ on a NEW warning. It runs from the hub's one goroutine (server
// OnSlowTick), so its dedup markers have no read-check-write race — this is the
// fix for the limits·warn 3× bug (its nudge used to live in gatherUsage, which
// /api/usage + the HQ card + the CLI call concurrently).
package app

import (
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/resource"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// slowTickEval is wired to server Deps.OnSlowTick. It evaluates resource +
// subscription-limit warnings and nudges HQ once per new/changed warning.
func slowTickEval() {
	// Resource: sample + nudge on a NEW machine warning.
	rep := currentResource()
	nudgeOnChange("resourcewarn", rep.Machine.Warn, "[gtmux] resource·warn "+rep.Machine.Warn, orphanTail(rep))
	// Limits: cache-gated refresh (spawns claude at most once per TTL), nudge on
	// a new weekly-window warning. (Moved here from gatherUsage — the 3× fix.)
	lr, _ := limits.Get(limits.LoadConfig(), false, time.Now())
	nudgeOnChange("limitswarn", lr.Warn, "[gtmux] limits·warn "+lr.Warn, "")
}

// nudgeOnChange sends msg to a live HQ pane only when `value` DIFFERS from the
// last value recorded in the marker; a same-value repeat is suppressed. An empty
// value clears the marker (no nudge). extra is appended to the message (e.g. the
// reclaim hint), when non-empty.
func nudgeOnChange(marker, value, msg, extra string) {
	path := filepath.Join(state.Dir(), marker)
	prior := state.ReadMarker(path)
	if value == prior {
		return // unchanged → never re-nudge (this is the dedup)
	}
	if value == "" {
		state.Remove(path)
		return
	}
	_ = state.WriteMarker(path, value)
	pane := findHQPane()
	if pane == "" {
		return
	}
	if extra != "" {
		msg += " — " + extra
	}
	_ = tmux.SendText(pane, msg, true)
}

// orphanTail summarizes the top reclaim candidate for the resource nudge, so the
// warning is actionable ("… · reclaim: iOS Simulator runtime 100MB").
func orphanTail(rep resource.Report) string {
	if len(rep.Orphans) == 0 {
		return ""
	}
	o := rep.Orphans[0]
	return "reclaim: " + o.Comm
}
