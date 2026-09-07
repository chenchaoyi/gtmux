package radar

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// The scan is a FULL process table, and it had no cache: `AgentDriverKey` takes one per
// PANE, so 28 panes paid 28 full scans for one question about the fleet. Attributed
// 2026-09-03 on a machine with 17 sessions / 28 panes / 1002 processes: ~52 of these a
// minute, giving `gtmux serve` a 6–7% baseline that grew linearly with the fleet.
func TestProcSnapshotIsCachedWithinItsTTL(t *testing.T) {
	resetProcCache()
	calls := 0
	orig := boundedPS
	boundedPS = func(...string) ([]byte, error) {
		calls++
		return []byte("100 1 0:01.00 /usr/bin/claude\n"), nil
	}
	defer func() { boundedPS = orig }()

	// The shape that cost the most: one question about the fleet, asked pane by pane.
	for i := 0; i < 28; i++ {
		if got := snapshotProcs(); len(got) != 1 {
			t.Fatalf("snapshot %d returned %d rows", i, len(got))
		}
	}
	if calls != 1 {
		t.Errorf("28 lookups ran ps %d times, want 1", calls)
	}
}

func TestProcCacheExpires(t *testing.T) {
	resetProcCache()
	calls := 0
	orig := boundedPS
	boundedPS = func(...string) ([]byte, error) {
		calls++
		return []byte("100 1 0:01.00 /usr/bin/claude\n"), nil
	}
	defer func() { boundedPS = orig }()

	snapshotProcs()
	procCacheMu.Lock()
	procCacheAt = time.Now().Add(-procCacheTTL - time.Second) // age it past the TTL
	procCacheMu.Unlock()
	snapshotProcs()
	if calls != 2 {
		t.Errorf("an expired cache did not refresh: %d calls", calls)
	}
}

func TestAFailedScanIsNotCached(t *testing.T) {
	// The failure this guards is a WEDGED ps — the one that froze the menu bar once.
	// Caching its empty result would hold the whole radar in that degraded state for the
	// TTL instead of retrying on the next question.
	resetProcCache()
	calls := 0
	orig := boundedPS
	boundedPS = func(...string) ([]byte, error) {
		calls++
		return nil, errors.New("ps wedged")
	}
	defer func() { boundedPS = orig }()

	snapshotProcs()
	snapshotProcs()
	if calls != 2 {
		t.Errorf("a degraded snapshot was cached: %d calls for 2 lookups", calls)
	}
}

func TestProcCacheIsSafeUnderConcurrentReaders(t *testing.T) {
	resetProcCache()
	orig := boundedPS
	boundedPS = func(...string) ([]byte, error) {
		return []byte("100 1 0:01.00 /usr/bin/claude\n"), nil
	}
	defer func() { boundedPS = orig }()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); snapshotProcs() }()
	}
	wg.Wait()
}

func resetProcCache() {
	procCacheMu.Lock()
	procCacheVal, procCacheAt = nil, time.Time{}
	procCacheMu.Unlock()
}
