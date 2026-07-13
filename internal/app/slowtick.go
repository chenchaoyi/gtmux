// The serve slow-tick evaluator (resource-watch + limits-watch): the SINGLE
// place that samples machine resources + subscription limits and nudges a live
// gtmux HQ on a NEW warning. It runs from the hub's one goroutine (server
// OnSlowTick), so its dedup markers have no read-check-write race — this is the
// fix for the limits·warn 3× bug (its nudge used to live in gatherUsage, which
// /api/usage + the HQ card + the CLI call concurrently).
package app

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/resource"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// slowTickEval is wired to server Deps.OnSlowTick. It evaluates resource +
// subscription-limit warnings and nudges HQ once per new/changed warning.
func slowTickEval() {
	// Draft-guard drain: flush any HQ nudges queued behind a half-typed draft. Cheap-
	// gated on Pending() so a quiet tick doesn't scan/capture the HQ pane.
	if hqnudge.Pending() {
		hqnudge.Drain(findHQPane())
	}
	// Resource: sample + nudge on a NEW machine warn TIER. Dedup keys on the tier
	// (amber/red), NOT the exact warn value — disk-free jittering 40→39→38 GB stays
	// amber and must NOT re-nudge per GB (the by-tier fix).
	rep := currentResource()
	nudgeOnChange("resourcewarn", resourceTierKey(rep.Machine), "[gtmux] resource·warn "+rep.Machine.Warn, orphanTail(rep))
	// Limits: cache-gated refresh (spawns claude at most once per TTL), nudge on
	// a new weekly-window crossing. (Moved here from gatherUsage — the 3× fix.)
	// Dedup keys on the WINDOW identity, so a % climbing within the same window
	// (93→94→95%) doesn't re-nudge.
	lr, _ := limits.Get(limits.LoadConfig(), false, time.Now())
	nudgeOnChange("limitswarn", limitsTierKey(lr.Warn), "[gtmux] limits·warn "+lr.Warn, "")
	// Lifecycle watchdog (charter M5): escalate a pane stuck waiting past the timeout.
	watchdogSweep(time.Now().Unix())
}

// resourceTierKey is the dedup key for a machine warning: the tier (amber/red), or
// "" when fine (which clears the marker).
func resourceTierKey(m resource.Machine) string {
	if m.Warn == "" {
		return ""
	}
	return resource.MachineTier(m).String()
}

// limitsTierKey reduces a limits warn ("week (fable) 93%") to its window identity
// ("week (fable)") so the dedup fires once per window crossing, not per % it climbs.
// "" (no warn) clears the marker.
func limitsTierKey(warn string) string {
	warn = strings.TrimSpace(warn)
	if warn == "" {
		return ""
	}
	if i := strings.LastIndexByte(warn, ' '); i > 0 {
		return strings.TrimSpace(warn[:i])
	}
	return warn
}

// nudgeOnChange sends msg to a live HQ pane only when the dedupKey DIFFERS from the
// key recorded in the marker; a same-key repeat is suppressed. Pass a TIER/LAYER key
// (not the raw jittering value) so intra-tier drift doesn't re-nudge. extra is
// appended to the message (e.g. the reclaim hint), when non-empty.
func nudgeOnChange(marker, dedupKey, msg, extra string) {
	if !markerChanged(marker, dedupKey) {
		return
	}
	pane := findHQPane()
	if pane == "" {
		return
	}
	if extra != "" {
		msg += " — " + extra
	}
	hqnudge.Deliver(pane, msg) // draft-guarded like every other HQ injection
}

// markerChanged reports whether dedupKey differs from the marker's last value, and
// persists the new key when it does. An empty key CLEARS the marker and returns false
// (nothing to nudge). This is the by-TIER dedup core — testable without tmux.
func markerChanged(marker, dedupKey string) bool {
	path := filepath.Join(state.Dir(), marker)
	if dedupKey == state.ReadMarker(path) {
		return false // unchanged tier → never re-nudge
	}
	if dedupKey == "" {
		state.Remove(path)
		return false
	}
	return state.WriteMarker(path, dedupKey) == nil
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
