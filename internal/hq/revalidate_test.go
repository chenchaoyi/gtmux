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

// splitWakeTail is what lets a probe re-render one field of a line the queue holds only
// as text. Getting it wrong would corrupt a delivered wake, so both shapes are pinned.
func TestSplitWakeTail(t *testing.T) {
	line := hqwake.Line(hqwake.ClassUsageWarn, "sat:0.0 (%74)", "ctx→80% in ~4m")
	head, tail, ok := splitWakeTail(line)
	if !ok || tail != "ctx→80% in ~4m" {
		t.Fatalf("split = (%q, %q, %v)", head, tail, ok)
	}
	if head+hqwake.FieldSep()+tail != line {
		t.Fatal("re-joining the split must reproduce the line exactly")
	}
	// A line with no field at all is not something to rewrite.
	if _, _, ok := splitWakeTail("» ▸ gtmux·usage·warn  sat:0.0 (%74)"); ok {
		t.Error("a field-less line must not be split")
	}
}

// The probe must fail OPEN on everything it cannot establish. Each case below is a step
// that could go wrong at delivery time, and every one of them delivers rather than
// discards — because the alarm it would discard may be real.
func TestUsageWarnProbe_FailsOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no resume records, no usage snapshots: nothing resolves
	cases := map[string]string{
		"no pane id in the head":  hqwake.Line(hqwake.ClassUsageWarn, "sat:0.0", "ctx 86%"),
		"no session on record":    hqwake.Line(hqwake.ClassUsageWarn, "sat:0.0 (%74)", "ctx 86%"),
		"no field to re-render":   "» ▸ gtmux·usage·warn  sat:0.0 (%74)",
		"a class we do not judge": hqwake.Line(hqwake.ClassWaiting, "sat:0.0 (%74)", "needs you"),
	}
	for name, line := range cases {
		keep, out := usageWarnProbe(line)
		if !keep || out != line {
			t.Errorf("%s: probe must deliver unchanged, got keep=%v out=%q", name, keep, out)
		}
	}
}
