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
