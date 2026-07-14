package hqfeed

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// setup points HOME at a temp dir with a normal events cap so Append assigns seqs.
func setup(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "gtmux"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	setup(t)
	if ReadCursor() != 0 {
		t.Fatalf("fresh cursor = %d, want 0", ReadCursor())
	}
	WriteCursor(42)
	if ReadCursor() != 42 {
		t.Fatalf("cursor = %d, want 42", ReadCursor())
	}
}

func TestHeartbeatAndStale(t *testing.T) {
	setup(t)
	now := int64(1_000_000)
	if !Stale(now) {
		t.Errorf("no heartbeat should read stale")
	}
	Beat(now)
	if Stale(now + 30) {
		t.Errorf("30s-old heartbeat should not be stale (< 90s)")
	}
	if !Stale(now + 91) {
		t.Errorf("91s-old heartbeat should be stale")
	}
}

func TestSpoolRotation(t *testing.T) {
	setup(t)
	now := int64(1_000_000)
	big := ""
	for len(big) < 200*1024 {
		big += "0123456789abcdef"
	}
	// Write enough to exceed the 8 MB cap and rotate at least once.
	for i := 0; i < 60; i++ {
		SpoolAppend(events.Record{Ts: now, Event: "Stop", Summary: big, Seq: int64(i + 1)})
	}
	if _, err := os.Stat(spoolRotatedPath()); err != nil {
		t.Fatalf("expected a rotated spool generation: %v", err)
	}
	// Total on-disk stays bounded (active + one rotated ≈ 2×cap).
	var total int64
	for _, p := range []string{SpoolPath(), spoolRotatedPath()} {
		if fi, err := os.Stat(p); err == nil {
			total += fi.Size()
		}
	}
	if total > 3*spoolCapBytes {
		t.Errorf("spool total %d exceeds bound", total)
	}
	// The reader spans both generations.
	if len(ReadSpool(0, now)) == 0 {
		t.Errorf("ReadSpool returned nothing")
	}
}

// TestRunCatchUpAndReconcile drives the daemon loop once over a pre-seeded journal
// and asserts it spools the caught-up events, advances the cursor, emits a
// reconcile control record, and beats.
func TestRunCatchUpAndReconcile(t *testing.T) {
	setup(t)
	now := int64(2_000_000)
	// Seed the journal with 3 events (Append assigns seq 1..3).
	for i := 0; i < 3; i++ {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "a:0.0"})
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { Run(func() int64 { return now }, stop); close(done) }()
	// Give the loop a moment to catch up, then stop.
	waitFor(t, func() bool { return ReadCursor() >= 3 }, time.Second)
	close(stop)
	<-done

	if ReadCursor() != 3 {
		t.Errorf("cursor = %d, want 3", ReadCursor())
	}
	spool := ReadSpool(0, now)
	var stops, reconcile int
	for _, r := range spool {
		switch r.Event {
		case "Stop":
			stops++
		case ControlReconcile:
			reconcile++
		}
	}
	if stops != 3 {
		t.Errorf("spooled Stop events = %d, want 3", stops)
	}
	if reconcile != 1 {
		t.Errorf("reconcile control records = %d, want 1", reconcile)
	}
	if HeartbeatAt() != now {
		t.Errorf("heartbeat = %d, want %d", HeartbeatAt(), now)
	}
}

// TestRunNoGapNoDegraded guards against a false-positive degradation: a normal
// contiguous catch-up (cursor 0 over seq 1..3) must NOT emit a feed-degraded record.
// (Gap DETECTION itself is unit-tested at the pure level in internal/events.)
func TestRunNoGapNoDegraded(t *testing.T) {
	setup(t)
	now := int64(3_000_000)
	for i := 0; i < 3; i++ {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "a:0.0"})
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { Run(func() int64 { return now }, stop); close(done) }()
	waitFor(t, func() bool { return ReadCursor() >= 3 }, time.Second)
	close(stop)
	<-done
	for _, r := range ReadSpool(0, now) {
		if r.Event == ControlFeedDegraded {
			t.Errorf("no gap should emit no degraded record, got %+v", r)
		}
	}
}

func TestSingleton(t *testing.T) {
	setup(t)
	if !AcquireSingleton() {
		t.Fatalf("first acquire should succeed")
	}
	if ReadPid() != os.Getpid() {
		t.Fatalf("pidfile = %d, want %d", ReadPid(), os.Getpid())
	}
	// A live pid (ourselves) blocks a second acquire.
	if AcquireSingleton() {
		t.Errorf("second acquire should fail while a live daemon holds the pidfile")
	}
	ReleaseSingleton()
	if ReadPid() != 0 {
		t.Errorf("pidfile should be gone after release")
	}
}

func waitFor(t *testing.T, cond func() bool, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", d)
}
