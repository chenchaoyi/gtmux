package events

import "sort"

// ReadSince returns the retained events with Seq greater than cursor, ordered by
// sequence, together with whether a GAP was detected — the perception feed's
// zero-loss catch-up (hq-attention-system). A consumer persists its last-consumed
// seq as the cursor; on reconnect it replays exactly the un-consumed tail, spanning
// a rotation, and never re-emits a consumed event.
//
// gap is true when events were missed between the cursor and what is retained:
// either the first retained event after the cursor is not exactly cursor+1, or
// there is a hole inside the retained tail. Missed events happen when the consumer
// was down long enough that older events rotated away, or (rarely) a phantom seq
// from a crash mid-assignment. In every case the right response is to reconcile
// from a full snapshot rather than proceed blind — this only SIGNALS the gap.
//
// A cursor of 0 means "from the start of what is retained" and never reports a
// leading gap (there is no prior position to be missing events relative to).
// Legacy records without a seq (Seq == 0) are excluded once the cursor is positive
// — they predate the sequence and cannot be positioned against it.
func ReadSince(cursor int64) (recs []Record, gap bool) {
	return readSinceFrom(Read(0, 0), cursor) // everything retained, both generations
}

// readSinceFrom is the pure core of ReadSince over an in-memory record set — the
// filter/sort/gap-detect logic, testable without disk.
func readSinceFrom(all []Record, cursor int64) (recs []Record, gap bool) {
	for _, r := range all {
		if r.Seq > cursor {
			recs = append(recs, r)
		}
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Seq < recs[j].Seq })
	if len(recs) == 0 {
		return recs, false
	}
	// Leading gap: the first retained event isn't the very next one we expected.
	if cursor > 0 && recs[0].Seq != cursor+1 {
		gap = true
	}
	// Internal gap: a hole inside the retained tail.
	for i := 1; i < len(recs); i++ {
		if recs[i].Seq != recs[i-1].Seq+1 {
			gap = true
			break
		}
	}
	return recs, gap
}

// LatestSeq returns the highest sequence number currently retained (0 when the
// log is empty or holds only legacy records). Used for startup reconciliation and
// health/status.
func LatestSeq() int64 {
	var max int64
	for _, r := range Read(0, 0) {
		if r.Seq > max {
			max = r.Seq
		}
	}
	return max
}
