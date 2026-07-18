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

// Restart backoff + cap. Without these the watchdog respawns the feed on EVERY
// unhealthy slow-tick (20 s), so a daemon that cannot come up is retried forever. During
// ONE continuous outage the watchdog instead spaces restarts by an exponential backoff
// and STOPS after maxRestartAttempts, falling back to the CRITICAL degradation + polling
// backstop rather than churning a doomed process. All of this resets the moment the feed
// is healthy again (the caller clears the attempt counter), so a later outage restarts
// immediately.
const (
	maxRestartAttempts     = 6   // stop respawning after this many failed restarts per outage
	restartBackoffBaseSecs = 30  // delay before the 1st retry (doubles each further attempt)
	restartBackoffMaxSecs  = 600 // cap the backoff at 10 min
)

// restartBackoffSecs returns the delay to impose AFTER making `attempts` restarts (so
// before attempt N+1). The very first restart of an outage is immediate (attempts 0 → the
// caller starts with nextAllowedAt 0); each subsequent delay doubles from the base, capped.
func restartBackoffSecs(attempts int) int64 {
	if attempts <= 0 {
		return 0
	}
	d := int64(restartBackoffBaseSecs) << (attempts - 1)
	if d > restartBackoffMaxSecs {
		d = restartBackoffMaxSecs
	}
	return d
}

// RestartGate decides whether the watchdog should spawn a feed restart THIS tick, given
// how many restarts this outage has already made (attempts), the tick time (now), and the
// earliest time a restart is permitted (nextAllowedAt, 0 = now). It returns whether to
// spawn plus the values to persist when it does: the new earliest-allowed time (now + the
// widening backoff) and the incremented attempt count. Once attempts reaches the cap it
// refuses (the degradation + backstop cover the outage). Pure — no I/O, so the schedule is
// unit-tested without tmux.
func RestartGate(attempts int, now, nextAllowedAt int64) (attempt bool, newNextAllowedAt int64, newAttempts int) {
	if attempts >= maxRestartAttempts {
		return false, nextAllowedAt, attempts // gave up — stop churning a doomed daemon
	}
	if now < nextAllowedAt {
		return false, nextAllowedAt, attempts // still inside the backoff window
	}
	newAttempts = attempts + 1
	return true, now + restartBackoffSecs(newAttempts), newAttempts
}
