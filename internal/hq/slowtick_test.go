package hq

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqsurface"
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

// journalDegradation is the carrier fix for the two watchdog records (hq-action-journal):
// journal-borne so the pull side actually sees them, important so they surface, and NOT
// audit — a degradation is new information, so it must count toward the consumption debt.
func TestJournalDegradationIsPullVisibleDebt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	journalDegradation(hqsurface.ControlWakeDegraded, "⚠ HQ wake channel not landing", 500)

	recs, _ := events.ReadSince(0)
	if len(recs) != 1 {
		t.Fatalf("got %d journal records, want 1", len(recs))
	}
	r := recs[0]
	if r.Event != hqsurface.ControlWakeDegraded || r.Severity != events.SevImportant {
		t.Fatalf("record = %+v, want an important wake-degraded", r)
	}
	if !events.IsControl(r) || events.IsAudit(r) {
		t.Fatal("a degradation is a control record and never audit trail")
	}
	// The whole point: it constitutes debt, so the completeness net delivers it even
	// when the wake channel itself is the casualty.
	if tally := unreadScan(0, testHQPane); tally.N != 1 {
		t.Fatalf("unread tally = %d, want the degradation counted as debt", tally.N)
	}
}
