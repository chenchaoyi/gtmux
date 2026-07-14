package hqfeed

// Health is the observable feed state the watchdog decides on (pure inputs so the
// decision is testable without tmux / processes).
type Health struct {
	HQLive   bool // an HQ pane is live — no HQ ⇒ no feed needed, zero cost
	PidAlive bool // the daemon pidfile names a live process
	HbStale  bool // the heartbeat is older than StaleAfter
}

// Healthy reports whether the feed is up and beating.
func (h Health) Healthy() bool { return h.PidAlive && !h.HbStale }

// NeedsRestart reports whether the watchdog should (re)start the daemon this tick:
// an HQ is live and the feed is not healthy. When no HQ is live the feed is not
// needed and NeedsRestart is false (the cost gate).
func NeedsRestart(h Health) bool {
	if !h.HQLive {
		return false
	}
	return !h.Healthy()
}

// escalateAfter is the number of consecutive unhealthy ticks (each following a
// restart attempt) at which mechanical self-heal is declared failed and the outage
// escalates to a CRITICAL degradation. Design §6.4: self-heal fails 2 → escalate.
const escalateAfter = 2

// NextFailureCount folds this tick's observation into the running consecutive-
// failure counter: a healthy feed resets to 0; an unhealthy one increments. The
// app persists the returned count across ticks.
func NextFailureCount(prev int, h Health) int {
	if !h.HQLive || h.Healthy() {
		return 0
	}
	return prev + 1
}

// ShouldEscalate reports whether the consecutive-failure count has reached the
// escalation threshold — mechanical self-heal has failed enough times to surface
// the outage.
func ShouldEscalate(failureCount int) bool { return failureCount >= escalateAfter }
