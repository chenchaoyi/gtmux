package dispatch

// The pending-decision disposition loop (hq-attention-system): set-disposition on a
// live entry, with the awaiting-commander disposition as the plate membership test.
// All mutations are in-place (no duplicate entry). The unwired attention verbs
// (promote / re-prioritize / mark-surfaced / archive) were removed by
// slim-attention-ledger: no caller ever reached them, and `gtmux reap` is what
// keeps the live set small.

// updateTask loads a LIVE task, applies mutate, stamps LastUpdate, and re-saves it.
// A missing task is a no-op returning false.
func updateTask(id string, now int64, mutate func(*Task)) bool {
	t, ok := LoadTask(id)
	if !ok {
		return false
	}
	mutate(&t)
	t.LastUpdate = now
	return AddTask(t) == nil
}

// DispositionAwaitingCommander is the one disposition value gtmux itself knows by name
// (hq-signal-ergonomics §4). Everything else in Disposition stays free text; this one
// is first-class because a VIEW is built on it — the pending-decision standing view,
// the single home of "what is on the commander's plate". Before it, that list lived
// only as prose re-printed in every brief.
const DispositionAwaitingCommander = "awaiting-commander"

// AwaitingCommander reports whether the entry is on the commander's plate. The
// disposition is the ONLY membership test: a hand-set disposition and a MarkAwaiting-
// Commander call must mean the same thing, or the view and the row disagree.
func (t Task) AwaitingCommander() bool { return t.Disposition == DispositionAwaitingCommander }

// PendingSince is the wait clock the standing view orders by: when the entry entered the
// pending set, falling back to when it entered the ledger at all for an entry marked
// before AwaitingSince existed (or by a hand-written disposition).
func (t Task) PendingSince() int64 {
	switch {
	case t.AwaitingSince > 0:
		return t.AwaitingSince
	case t.FirstSeen > 0:
		return t.FirstSeen
	default:
		return t.CreatedAt
	}
}

// SetDisposition records how an item was handled (auto-answered / relayed / todo …),
// and maintains the pending-set clock as a side effect so there is exactly ONE way in
// and out of the plate: setting the awaiting-commander disposition puts an entry there,
// setting any other disposition (including "") takes it off.
func SetDisposition(id, disposition string, now int64) bool {
	return updateTask(id, now, func(t *Task) { applyDisposition(t, disposition, now) })
}

// applyDisposition sets the disposition and keeps AwaitingSince in step. Re-marking an
// already-pending entry does NOT restart its clock — the view orders by oldest-waiting,
// and a re-mark is not a new wait.
func applyDisposition(t *Task, disposition string, now int64) {
	t.Disposition = disposition
	if disposition != DispositionAwaitingCommander {
		t.AwaitingSince = 0
		return
	}
	if t.AwaitingSince == 0 {
		t.AwaitingSince = now
	}
}

// MarkAwaitingCommander puts an entry on the commander's plate.
func MarkAwaitingCommander(id string, now int64) bool {
	return SetDisposition(id, DispositionAwaitingCommander, now)
}

// ClearAwaitingCommander takes an entry off the plate, recording how it left (decided /
// withdrawn / escalated …). An empty disposition clears the field entirely.
func ClearAwaitingCommander(id, disposition string, now int64) bool {
	if disposition == DispositionAwaitingCommander {
		disposition = "" // clearing into the same state would be a no-op, not a resolution
	}
	return SetDisposition(id, disposition, now)
}
