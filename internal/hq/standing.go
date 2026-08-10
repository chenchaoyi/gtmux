package hq

// The standing-knock cadence (standing-wake-backoff, ledger C17 · C15 · C20③).
//
// Several wake classes repeat until an ACT clears them — self-rotate until the session id
// changes, unread until consumption, resource/usage warns on their tiers. That shape is
// correct: a knock is a debt, not an FYI. What it lacked is the second question a repeat
// has to answer, and which the FIRST knock had already answered for itself:
//
//	Has anything changed since I last said this?
//
// A repeat carrying the same breach against an unchanged world tells the consumer nothing
// the previous one didn't — and it is not free: each delivery costs HQ a full turn. For
// self-rotate that turn burns the very context the knock is warning about, which is how
// one measured night produced 17 knocks in 10 hours while the fleet sat completely still
// and ctx climbed 78% → 96% largely on the answer-the-knock turns themselves. The alarm
// was manufacturing its own evidence.
//
// So a repeat is suppressed while its FINGERPRINT is unchanged, and any drift re-arms it.
// Two rules bound the suppression, because the dangerous direction on an alarm channel is
// silence:
//
//   - a minimum spacing still applies to a CHANGED fingerprint (a fast-flapping world must
//     not produce a fast-knocking alarm), and
//   - a safety floor still fires eventually even when nothing has changed at all, so a
//     standing debt cannot be forgotten entirely.
//
// This is the family mechanism, kept pure and separate from any one class's sensing.

// standingWorld is what a standing class knows about the world at one evaluation.
type standingWorld struct {
	// Breach identifies WHAT is currently over the line — a set, not a rendering. It must
	// be built from the criteria that tripped ("age", "ctx,age"), never from the numbers,
	// because several of those numbers only ever grow: an age figure re-rendered every
	// tick would drift by construction and re-arm the alarm forever, which is precisely
	// the loop being removed. A criterion CROSSING is a change; a criterion getting worse
	// while already crossed is not news.
	Breach string
	// Moved counts observable world events OUTSIDE the knocked role — fleet activity that
	// could change the judgment. It is compared, never interpreted, so a class may count
	// whatever it can cheaply see.
	//
	// It deliberately excludes the knocked role's OWN turns, which is a deviation from the
	// proposal's literal "no HQ-pane turns since the last knock". Counting them re-arms the
	// alarm on the very turn that ANSWERS it — the self-feeding loop the ledger identified.
	// Genuine escalation from the role's own work still re-arms, through Breach: that is
	// what a turns threshold crossing is.
	Moved int64
}

// standingState is what a standing class persists between evaluations.
type standingState struct {
	KnockedAt int64  // when the last knock was DELIVERED (0 = never)
	Breach    string // the breach set that knock announced
	Moved     int64  // the world counter at that moment
}

// standingVerdict is one evaluation's outcome.
type standingVerdict int

const (
	standingClear standingVerdict = iota // nothing over the line — say nothing, re-arm
	standingHold                         // over the line, but this repeat would add nothing
	standingKnock                        // deliver
)

// standingDecide is the pure cadence rule. `repeat` is the minimum spacing between any two
// knocks; `floor` is the interval after which an entirely unchanged breach speaks anyway
// (0 disables the floor — the breach then stays silent until something changes).
//
// Nothing here CLEARS a breach. Only the act the class asked for does, in the caller —
// the same guarantee the consumption watermark makes, applied to a different debt: gtmux
// stops asking when the thing it asked for has happened, not when it has finished asking.
func standingDecide(now int64, w standingWorld, st standingState, repeat, floor int64) (standingVerdict, standingState) {
	if w.Breach == "" {
		// Back under every line. Forget the last knock so a fresh breach gets a fresh
		// FIRST knock rather than inheriting a suppression it never earned.
		return standingClear, standingState{}
	}
	if st.KnockedAt == 0 {
		return standingKnock, standingState{KnockedAt: now, Breach: w.Breach, Moved: w.Moved}
	}
	if now-st.KnockedAt < repeat {
		return standingHold, st // minimum spacing, whatever changed
	}
	changed := w.Breach != st.Breach || w.Moved != st.Moved
	if !changed && (floor <= 0 || now-st.KnockedAt < floor) {
		return standingHold, st
	}
	return standingKnock, standingState{KnockedAt: now, Breach: w.Breach, Moved: w.Moved}
}
