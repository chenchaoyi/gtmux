package hq

import "testing"

func TestShouldSelfCheck(t *testing.T) {
	const h = 60 * 60
	now := int64(1_000_000)

	// Rate limited: < 1h since last check → never, whatever the conditions.
	if fire, _ := shouldSelfCheck(now, now-30*60, false, 999, true, true); fire {
		t.Error("must not fire within the 1h rate limit")
	}

	// Threshold trip fires immediately (past the rate limit).
	if fire, reason := shouldSelfCheck(now, now-2*h, false, selfCheckLedgerCap+1, false, false); !fire || reason != "threshold" {
		t.Errorf("open-ledger over cap should fire threshold, got %v/%q", fire, reason)
	}
	if fire, reason := shouldSelfCheck(now, now-2*h, false, 0, true, false); !fire || reason != "threshold" {
		t.Errorf("journal-over-ceiling should fire threshold, got %v/%q", fire, reason)
	}
	if fire, reason := shouldSelfCheck(now, now-2*h, false, 0, false, true); !fire || reason != "threshold" {
		t.Errorf("cursor gap should fire threshold, got %v/%q", fire, reason)
	}

	// Daily floor: ≥ 24h since last check fires even with recent attention + small ledger.
	if fire, reason := shouldSelfCheck(now, now-25*h, true, 3, false, false); !fire || reason != "daily" {
		t.Errorf("24h floor should fire daily, got %v/%q", fire, reason)
	}

	// Idle (resting user): no recent attention AND ≥ 12h since last check.
	if fire, reason := shouldSelfCheck(now, now-13*h, false, 3, false, false); !fire || reason != "idle" {
		t.Errorf("idle trigger should fire, got %v/%q", fire, reason)
	}
	// But NOT idle when attention was recent.
	if fire, _ := shouldSelfCheck(now, now-13*h, true, 3, false, false); fire {
		t.Error("recent attention should suppress the idle trigger")
	}
	// And NOT idle before the 12h floor (2h since last, nothing else tripping).
	if fire, _ := shouldSelfCheck(now, now-2*h, false, 3, false, false); fire {
		t.Error("idle trigger needs ≥ 12h since last self-check")
	}
}

func TestSelfCheckMarkerRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if readSelfCheckAt() != 0 {
		t.Fatalf("fresh self-check marker = %d, want 0", readSelfCheckAt())
	}
	writeSelfCheckAt(123456)
	if readSelfCheckAt() != 123456 {
		t.Fatalf("self-check marker = %d, want 123456", readSelfCheckAt())
	}
}
