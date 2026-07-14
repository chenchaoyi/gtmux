// The serve slow-tick evaluator (resource-watch + limits-watch): the SINGLE
// place that samples machine resources + subscription limits and nudges a live
// gtmux HQ on a NEW warning. It runs from the hub's one goroutine (server
// OnSlowTick), so its dedup markers have no read-check-write race — this is the
// fix for the limits·warn 3× bug (its nudge used to live in gatherUsage, which
// /api/usage + the HQ card + the CLI call concurrently).
package app

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqfeed"
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
	// Perception-feed watchdog (hq-attention-system): keep the silent feed daemon
	// alive while an HQ is live; mechanically self-heal, escalate CRITICAL only after
	// self-heal fails twice.
	feedWatchdog(time.Now().Unix())
}

// feedFailCountPath stores the consecutive restart-failure counter (text int).
func feedFailCountPath() string { return filepath.Join(state.Dir(), "hq-feed", "restart-fails") }

func readFeedFailCount() int {
	n, _ := strconv.Atoi(state.ReadMarker(feedFailCountPath()))
	return n
}

func writeFeedFailCount(n int) { _ = state.WriteMarker(feedFailCountPath(), strconv.Itoa(n)) }

// feedWatchdog is the no-LLM perception-feed supervisor (design §1.2.2 / §6.4). It
// runs from the single-writer serve slow-tick, so its markers have no race. Only
// while an HQ pane is live: it ensures the daemon is up and beating, mechanically
// restarts a dead/stale one (SILENTLY), and — only after two consecutive failed
// restarts — surfaces a CRITICAL degradation (a feed-degraded control record + one
// visible HQ nudge), deduped so recovery doesn't re-alert. This is the ONE place
// the feed watchdog is allowed to be visible: a perception outage must not stay
// silent (the commander's #1 requirement).
func feedWatchdog(now int64) {
	hqLive := findHQPane() != ""
	h := hqfeed.Health{HQLive: hqLive, PidAlive: hqfeed.Running(), HbStale: hqfeed.Stale(now)}
	if hqfeed.NeedsRestart(h) {
		_ = spawnFeedDaemon() // detached; the singleton guard makes a redundant spawn safe
	}
	next := hqfeed.NextFailureCount(readFeedFailCount(), h)
	writeFeedFailCount(next)

	// Escalate once on the transition into degraded; clear on recovery (empty key)
	// without re-alerting. markerChanged is the by-tier dedup core.
	key := ""
	if hqfeed.ShouldEscalate(next) {
		key = "down"
	}
	if markerChanged("hqfeeddegraded", key) {
		hqfeed.EmitControl(hqfeed.ControlFeedDegraded,
			"⚠ perception feed down — mechanical self-heal failed; on the 5-min polling backstop",
			events.SevImportant, now)
		if pane := findHQPane(); pane != "" {
			hqnudge.Deliver(pane, "[gtmux] ⚠ perception feed degraded — self-heal failed, on polling backstop. Re-attach `gtmux hq-feed --tail`.")
		}
	}
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
