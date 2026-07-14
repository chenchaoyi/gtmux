package events

import (
	"testing"
	"time"
)

// TestReadSinceReplaysTailExactlyOnce verifies a consumer resuming from a cursor
// gets every event after it, once, in sequence order, and no gap under normal
// continuous appends.
func TestReadSinceReplaysTailExactlyOnce(t *testing.T) {
	tinyCap(t, 20)
	now := time.Now().Unix()
	for i := 0; i < 5; i++ {
		Append(Record{Ts: now, Event: "Stop", State: "idle", Loc: "a:0.0"})
	}
	// Consumer already saw up to seq 2.
	recs, gap := ReadSince(2)
	if gap {
		t.Errorf("unexpected gap on a contiguous tail")
	}
	if len(recs) != 3 {
		t.Fatalf("ReadSince(2) = %d records, want 3", len(recs))
	}
	for i, r := range recs {
		if r.Seq != int64(i+3) {
			t.Errorf("record %d seq = %d, want %d", i, r.Seq, i+3)
		}
	}
	// Advancing the cursor to the end yields nothing more.
	rest, _ := ReadSince(5)
	if len(rest) != 0 {
		t.Errorf("ReadSince(5) = %d, want 0", len(rest))
	}
}

// TestReadSinceCursorZeroFromStart confirms cursor 0 returns everything with no
// leading-gap false positive.
func TestReadSinceCursorZeroFromStart(t *testing.T) {
	tinyCap(t, 20)
	now := time.Now().Unix()
	for i := 0; i < 3; i++ {
		Append(Record{Ts: now, Event: "Stop", State: "idle", Loc: "a:0.0"})
	}
	recs, gap := ReadSince(0)
	if gap {
		t.Errorf("cursor 0 must not report a leading gap")
	}
	if len(recs) != 3 {
		t.Fatalf("ReadSince(0) = %d, want 3", len(recs))
	}
}

// TestReadSinceDetectsLeadingGap simulates a consumer that was down long enough
// that its cursor points before the oldest retained event — a gap it must reconcile.
func TestReadSinceDetectsLeadingGap(t *testing.T) {
	tinyCap(t, 20)
	now := time.Now().Unix()
	for i := 0; i < 3; i++ {
		Append(Record{Ts: now, Event: "Stop", State: "idle", Loc: "a:0.0"})
	}
	// Retained seqs are 1,2,3; a consumer at cursor 5 (ahead) sees nothing and no gap.
	if recs, gap := ReadSince(5); len(recs) != 0 || gap {
		t.Errorf("ahead-of-log cursor: recs=%d gap=%v, want 0,false", len(recs), gap)
	}
	// Cursor 0 never leading-gaps, even when the first retained seq isn't 1 (older
	// events rotated away): that's retention, and cursor 0 means "from the start".
	if _, gap := readSinceFrom([]Record{{Seq: 2}, {Seq: 3}}, 0); gap {
		t.Errorf("cursor 0 leading gap should be suppressed")
	}
	// A positive cursor whose next expected (cursor+1) is missing IS a leading gap.
	if _, gap := readSinceFrom([]Record{{Seq: 5}, {Seq: 6}}, 3); !gap {
		t.Errorf("expected leading gap: cursor 3, first retained 5")
	}
	// A hole inside the retained tail is an internal gap; the records still return.
	recs, gap := readSinceFrom([]Record{{Seq: 4}, {Seq: 6}}, 3)
	if !gap {
		t.Errorf("expected internal gap: 4 then 6")
	}
	if len(recs) != 2 {
		t.Errorf("gap set still returns its records, got %d", len(recs))
	}
}

// TestLatestSeq confirms the high-water mark.
func TestLatestSeq(t *testing.T) {
	tinyCap(t, 20)
	now := time.Now().Unix()
	if LatestSeq() != 0 {
		t.Errorf("empty LatestSeq = %d, want 0", LatestSeq())
	}
	for i := 0; i < 4; i++ {
		Append(Record{Ts: now, Event: "Stop", State: "idle", Loc: "a:0.0"})
	}
	if LatestSeq() != 4 {
		t.Errorf("LatestSeq = %d, want 4", LatestSeq())
	}
}
