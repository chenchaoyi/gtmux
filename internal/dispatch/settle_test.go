package dispatch

import (
	"strings"
	"testing"
)

// A paste gets a fixed 3 frames (~900ms) to render before the guard calls it a fragment
// and — correctly, by its own rule — refuses to press Enter. That budget did not depend
// on how much was pasted, so a long message could still be arriving when the deadline
// passed: the user saw it land with its tail missing and never submit. "Sometimes",
// because it is a race with the TUI's redraw.

func TestSettleGrowsWithThePayload(t *testing.T) {
	short := settleFrames(3, "1")
	long := settleFrames(3, strings.Repeat("x", 4000))
	if short != 3 {
		t.Errorf("a keystroke-sized paste = %d frames; want the base 3 (short pastes must not get slower)", short)
	}
	if long <= short {
		t.Errorf("4000 chars = %d frames, same as %d for one char — the fixed budget IS the bug", long, short)
	}
}

func TestSettleIsCappedSoADeliveryCannotStall(t *testing.T) {
	huge := settleFrames(3, strings.Repeat("x", 10_000_000))
	if huge != 3+settleMaxExtraFrames {
		t.Errorf("a pathological payload = %d frames; want the cap %d", huge, 3+settleMaxExtraFrames)
	}
}

// The scaling must never make a delivery WAIT WHEN IT DOESN'T HAVE TO: confirmPaste
// returns the moment the draft holds the delivery, so the budget is only the point at
// which we give up.
func TestSettleIsAnUpperBoundNotADelay(t *testing.T) {
	f := &fakeIO{caps: []string{boxDraft("the full message")}}
	if v := confirmPaste(f.io(), Opts{PasteSettle: 3}, "the full message"); v != pasteInDraft {
		t.Fatalf("verdict = %v; want pasteInDraft", v)
	}
	if f.clock != 0 {
		t.Errorf("slept %d frames on an already-settled draft; want 0", f.clock)
	}
}

// A degenerate base must not disable the wait entirely.
func TestSettleBaseIsAtLeastOneFrame(t *testing.T) {
	if got := settleFrames(0, "x"); got < 1 {
		t.Errorf("settleFrames(0, …) = %d; want at least 1 frame", got)
	}
}
