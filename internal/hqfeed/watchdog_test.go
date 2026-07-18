package hqfeed

import "testing"

func TestNeedsRestart(t *testing.T) {
	cases := []struct {
		name string
		h    Health
		want bool
	}{
		{"no HQ → never", Health{HQLive: false, PidAlive: false, HbStale: true}, false},
		{"healthy → no", Health{HQLive: true, PidAlive: true, HbStale: false}, false},
		{"dead pid → yes", Health{HQLive: true, PidAlive: false, HbStale: false}, true},
		{"stale beat → yes", Health{HQLive: true, PidAlive: true, HbStale: true}, true},
	}
	for _, c := range cases {
		if got := NeedsRestart(c.h); got != c.want {
			t.Errorf("%s: NeedsRestart = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFailureCountAndEscalation(t *testing.T) {
	unhealthy := Health{HQLive: true, PidAlive: false, HbStale: true}
	healthy := Health{HQLive: true, PidAlive: true, HbStale: false}

	// A successful self-heal resets the count and never escalates.
	if got := NextFailureCount(1, healthy); got != 0 {
		t.Errorf("healthy resets count, got %d", got)
	}
	// Two consecutive unhealthy ticks reach the escalation threshold.
	c := 0
	c = NextFailureCount(c, unhealthy) // 1 — first failed restart
	if ShouldEscalate(c) {
		t.Errorf("should not escalate after one failure")
	}
	c = NextFailureCount(c, unhealthy) // 2 — second failed restart
	if !ShouldEscalate(c) {
		t.Errorf("should escalate after two consecutive failures")
	}
	// Recovery clears escalation.
	c = NextFailureCount(c, healthy)
	if ShouldEscalate(c) || c != 0 {
		t.Errorf("recovery should reset, got count %d", c)
	}
	// No HQ resets too (feed not needed).
	if got := NextFailureCount(3, Health{HQLive: false}); got != 0 {
		t.Errorf("no HQ resets count, got %d", got)
	}
}

func TestRestartGate(t *testing.T) {
	const now int64 = 1_000_000

	// First attempt of an outage is immediate (attempts 0, nextAllowedAt 0).
	do, next, attempts := RestartGate(0, now, 0)
	if !do || attempts != 1 {
		t.Fatalf("first attempt: do=%v attempts=%d, want true/1", do, attempts)
	}
	if next != now+restartBackoffBaseSecs {
		t.Fatalf("first backoff = %d, want %d", next-now, int64(restartBackoffBaseSecs))
	}

	// Still inside the backoff window → no attempt, state unchanged.
	if do, n, a := RestartGate(1, now+10, next); do || n != next || a != 1 {
		t.Fatalf("inside backoff should refuse and preserve state, got do=%v n=%d a=%d", do, n, a)
	}

	// Backoff doubles each attempt, capped at the max.
	prev := int64(0)
	for a := 1; a < maxRestartAttempts; a++ {
		do, nextAt, na := RestartGate(a, now, 0) // now ≥ nextAllowedAt (0) → eligible
		if !do || na != a+1 {
			t.Fatalf("attempt %d: do=%v na=%d, want true/%d", a, do, na, a+1)
		}
		delay := nextAt - now
		want := int64(restartBackoffBaseSecs) << (a) // backoff after (a+1) attempts
		if want > restartBackoffMaxSecs {
			want = restartBackoffMaxSecs
		}
		if delay != want {
			t.Fatalf("attempt %d backoff = %d, want %d", a, delay, want)
		}
		if delay < prev {
			t.Fatalf("backoff must not shrink: %d then %d", prev, delay)
		}
		prev = delay
	}

	// At/over the cap the gate refuses forever (until the caller resets on recovery).
	if do, _, a := RestartGate(maxRestartAttempts, now+1_000_000, 0); do || a != maxRestartAttempts {
		t.Fatalf("past cap should refuse, got do=%v a=%d", do, a)
	}
}
