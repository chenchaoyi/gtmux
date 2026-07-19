package hq

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resource"
)

// markerChanged is the by-TIER dedup core: a value that jitters within the same tier
// must not re-nudge; only a tier change (or a clear-then-recross) does.
func TestMarkerChanged_ByTier(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if !markerChanged("resourcewarn", "amber") {
		t.Fatal("first crossing into amber should nudge")
	}
	if markerChanged("resourcewarn", "amber") {
		t.Fatal("same tier (intra-tier jitter) must NOT re-nudge")
	}
	if !markerChanged("resourcewarn", "red") {
		t.Fatal("escalation amber→red should nudge")
	}
	if markerChanged("resourcewarn", "") {
		t.Fatal("clearing to normal must not nudge")
	}
	if !markerChanged("resourcewarn", "amber") {
		t.Fatal("re-crossing into amber after a clear should nudge again")
	}
}

// limitsTierKey collapses a limits warn to its window identity so a climbing % within
// the same window doesn't re-nudge.
func TestLimitsTierKey(t *testing.T) {
	for _, c := range [][2]string{
		{"week (fable) 93%", "week (fable)"},
		{"week (fable) 94%", "week (fable)"}, // same key despite the % jitter
		{"week (all models) 88%", "week (all models)"},
		{"  ", ""},
		{"", ""},
	} {
		if got := limitsTierKey(c[0]); got != c[1] {
			t.Errorf("limitsTierKey(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestResourceTierKey(t *testing.T) {
	if got := resourceTierKey(resource.Machine{}); got != "" {
		t.Errorf("no warn → empty key, got %q", got)
	}
	m := resource.Machine{Warn: "disk 40Gi", DiskFreeGB: 40}
	if got := resourceTierKey(m); got == "" || got != resource.MachineTier(m).String() {
		t.Errorf("warn set → tier key %q (MachineTier=%q)", got, resource.MachineTier(m).String())
	}
}

// TestFeedFailCountRoundTrip confirms the perception-feed restart-failure counter
// persists across ticks (the state.marker glue behind the pure NextFailureCount).
func TestFeedFailCountRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if readFeedFailCount() != 0 {
		t.Fatalf("fresh fail count = %d, want 0", readFeedFailCount())
	}
	writeFeedFailCount(2)
	if readFeedFailCount() != 2 {
		t.Fatalf("fail count = %d, want 2", readFeedFailCount())
	}
	writeFeedFailCount(0)
	if readFeedFailCount() != 0 {
		t.Fatalf("reset fail count = %d, want 0", readFeedFailCount())
	}
}

// TestFeedDegradedDedup confirms the degraded escalation fires once on the
// transition into degraded and clears on recovery without re-alerting — the same
// by-tier dedup the resource/limits nudges use.
func TestFeedDegradedDedup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Not degraded → key "" → no fire.
	if markerChanged("hqfeeddegraded", "") {
		t.Fatal("healthy feed must not alert")
	}
	// Transition into degraded fires once.
	if !markerChanged("hqfeeddegraded", "down") {
		t.Fatal("first degradation should alert")
	}
	if markerChanged("hqfeeddegraded", "down") {
		t.Fatal("still-degraded must not re-alert")
	}
	// Recovery clears without alerting.
	if markerChanged("hqfeeddegraded", "") {
		t.Fatal("recovery must not alert")
	}
	// A fresh outage after recovery alerts again.
	if !markerChanged("hqfeeddegraded", "down") {
		t.Fatal("re-degradation after recovery should alert again")
	}
}

// resolvedDecide is the pure waiting→clear transition rule the resolved backstop runs.
func TestResolvedDecide(t *testing.T) {
	// A displayed-waiting pane is (re)tracked on its marker kind.
	if v, k := resolvedDecide("waiting", "", "permission", false); v != resolvedTrack || k != "permission" {
		t.Fatalf("waiting → track(permission), got %v %q", v, k)
	}
	// Waiting with no marker kind tracks a placeholder (still records the wait).
	if v, k := resolvedDecide("waiting", "", "", false); v != resolvedTrack || k != "pending" {
		t.Fatalf("waiting w/o marker → track(pending), got %v %q", v, k)
	}
	// A pane we never tracked as waiting produces no resolved.
	if v, _ := resolvedDecide("working", "", "", false); v != resolvedHold {
		t.Fatalf("untracked non-waiting → hold, got %v", v)
	}
	// The core fix: tracked waiting → now working (permission approved, agent resumed)
	// with no wait on screen → EMIT resolved.
	if v, _ := resolvedDecide("working", "permission", "permission", false); v != resolvedEmit {
		t.Fatalf("tracked waiting → working (clear) → emit, got %v", v)
	}
	// Same, going idle (Stop-style clear) → EMIT.
	if v, _ := resolvedDecide("idle", "question", "", false); v != resolvedEmit {
		t.Fatalf("tracked waiting → idle (clear) → emit, got %v", v)
	}
	// Flicker guard: a one-tick liveWorking flip while the approval card is STILL on
	// screen must NOT read as resolved — keep tracking.
	if v, _ := resolvedDecide("working", "permission", "permission", true); v != resolvedHold {
		t.Fatalf("clear with a wait still on screen → hold (flicker guard), got %v", v)
	}
}
