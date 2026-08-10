package hq

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hqwake"
)

// The probe's contract, in the direction that matters most: it may DROP only on positive
// evidence that the premise is gone, and must pass through everything it does not own.
func TestResourceWarnProbe_PassesThroughWhatItDoesNotOwn(t *testing.T) {
	for _, line := range []string{
		hqwake.Line(hqwake.ClassSelfRotate, "ctx 80%", "over: ctx"),
		hqwake.Line(hqwake.ClassStuckWaiting, "sat:0.0 (%7)", "waited 11m"),
		hqwake.Line(hqwake.ClassUsageWarn, "%18", "ctx 86%"),
		"a bare line from nowhere",
	} {
		keep, out := probeForTest(line)
		if !keep || out != line {
			t.Errorf("probe touched a line it does not own: %q → keep=%v out=%q", line, keep, out)
		}
	}
}

// A queued wake is decided at enqueue and delivered later; the revalidation seam is what
// closes that gap. With no probe registered the chain is an identity — the pre-change
// behavior, and the right default: an unclaimed class must never be silently droppable.
func TestRevalidatorChainDefaultsToIdentity(t *testing.T) {
	line := hqwake.Line(hqwake.ClassWaiting, "sat:0.0 (%7)", "needs you")
	keep, out := probeForTest(line)
	if !keep || out != line {
		t.Fatalf("an unowned line must pass through unchanged, got keep=%v out=%q", keep, out)
	}
}
