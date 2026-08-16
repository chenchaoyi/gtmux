package hqfeed

import (
	"os"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// Run is the daemon loop: catch up from the cursor, reconcile, then stream every
// new journal event into the spool while beating a heartbeat. It returns when stop
// is closed. `nowFn` supplies unix seconds (injectable for tests). It is a no-op
// second instance is prevented by the caller (AcquireSingleton) — Run itself just
// assumes it owns the feed.
func Run(nowFn func() int64, stop <-chan struct{}) {
	if nowFn == nil {
		nowFn = func() int64 { return time.Now().Unix() }
	}

	// --- startup reconciliation -------------------------------------------------
	cursor := ReadCursor()
	recs, gap := events.ReadSince(cursor)
	for _, r := range recs {
		SpoolAppend(r)
		if r.Seq > cursor {
			cursor = r.Seq
		}
	}
	WriteCursor(cursor)
	announceStartup(gap, nowFn())
	Beat(nowFn())

	// --- heartbeat loop ---------------------------------------------------------
	var mu sync.Mutex // guards cursor across the stream callback + nothing else races it here
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(HeartbeatInterval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				Beat(nowFn())
			}
		}
	}()

	// --- stream new events ------------------------------------------------------
	// A small replay window closes the race between the catch-up above and the tail
	// opening at end; the seq>cursor dedup drops the overlap so nothing is lost or
	// double-spooled.
	events.Follow(replayWindowSecs, nowFn(), func(r events.Record) {
		mu.Lock()
		defer mu.Unlock()
		if r.Seq <= cursor {
			return
		}
		SpoolAppend(r)
		cursor = r.Seq
		WriteCursor(cursor)
	}, stop)

	wg.Wait()
}

// announceStartup journals the daemon's startup control records: a CRITICAL
// `feed-degraded` when the catch-up found a cursor gap (events were missed —
// rotated away while down, or a phantom seq), and the `reconcile` that tells HQ
// to rebuild from one full digest snapshot. JOURNAL-borne, not spool-only (#647:
// a record written only to the spool reaches no reader — the playbook tells HQ
// it need not tail the feed, and `gtmux events` reads the journal). The daemon's
// own tail spools each on its normal pass, so one append yields one spool copy.
func announceStartup(gap bool, now int64) {
	if gap {
		events.Append(events.Record{Ts: now, Event: ControlFeedDegraded,
			Summary:  "perception feed detected a gap on startup — reconciling from a full snapshot",
			Severity: events.SevImportant})
	}
	events.Append(events.Record{Ts: now, Event: ControlReconcile,
		Summary:  "perception feed (re)started — pull a full `gtmux digest` snapshot to reconcile",
		Severity: events.SevNotable})
}

// AcquireSingleton claims the pidfile for this process, returning false when a live
// daemon already holds it (so the caller exits without starting a competitor). A
// stale pidfile (dead pid) is taken over.
func AcquireSingleton() bool {
	if Running() {
		return false
	}
	return WritePid(os.Getpid()) == nil
}

// ReleaseSingleton removes the pidfile if it still names this process (best-effort).
func ReleaseSingleton() {
	if ReadPid() == os.Getpid() {
		_ = os.Remove(PidPath())
	}
}
