package hook

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"

	"github.com/chenchaoyi/gtmux/internal/usage"
)

// AUDIT 1.2 (standing-wake-backoff): why did `usage·warn burn` knock THREE times in one
// round (2026-08-05, `%18` 23.6M → 23.7M → 23.9M) when watchUsage dedups on the layer?
//
// Reproduced here, and it is not a dedup bug — it is a MISSING anti-flap layer.
// Evaluate reports the FIRST breached layer, ctx before burn. A session whose ctx sits
// near its threshold therefore alternates ctx → burn → ctx → burn as the value dithers,
// and every alternation is a layer CHANGE, which is precisely what watchUsage treats as
// "new, nudge". The burn breach never went away; only its turn to be reported did.
//
// Contrast with resource·warn, which pays for exactly this with three rules — hysteresis
// on the threshold, N confirming samples, and a restate interval. usage·warn has none of
// them. That, not the dedup key, is the hole; the fix belongs with §5 of this change.
func TestAudit_UsageBurnRepeatsWhenCtxDithers(t *testing.T) {
	l := usage.Layers{CtxWarn: 0.8, SessionOutWarn: 20_000_000, TypeRatePerMinWarn: 30_000}
	const win = int64(1_000_000)
	// The burn breach is CONSTANT and real throughout — 23.6M, over the 20M line.
	const burned = int64(23_600_000)
	// ctx dithers a hair either side of 0.80. A modest rate keeps the burn layer live
	// (post-§5.1 burn alarms on the projection, never on the bare total).
	ctx := []float64{0.81, 0.79, 0.81, 0.79, 0.81}
	const rate = int64(20_000)

	// The measured shape needed the BARE-TOTAL burn layer: with no rate to project ctx,
	// evaluation fell through to `burn <total>`, and back to ctx as soon as ctx crossed
	// again. §5.1 deleted that layer, so the alternation has no second layer to reach —
	// a stopped session past the burn line is simply silent now.
	prior, flips := "", 0
	for _, c := range ctx {
		warn := usage.Evaluate(l, 30*time.Minute, win, c, burned, 0)
		if layerOf(warn) == "burn" {
			t.Errorf("a bare total still alarms (%q) — §5.1 removed that form", warn)
		}
		if warn != "" && layerOf(warn) != layerOf(prior) {
			flips++
		}
		prior = warn
	}
	// The ctx layer itself still re-arms as the value crosses back and forth — the warn
	// clears for a sample and returns. That residue is what the restate gate below is
	// for, and it is why the gate must NOT treat a one-sample clear as a recovery.
	if flips < 2 {
		t.Fatalf("expected the ctx layer to re-arm on dithering (that is the residue), got %d", flips)
	}
	_ = rate

	// THE FIX: the restate gate turns those flips into ONE knock. A layer change is no
	// longer sufficient to speak; the pane must also have been quiet long enough.
	t.Setenv("HOME", t.TempDir())
	knocks, now := 0, int64(10_000_000)
	for range ctx {
		if usageRestateOK("%18", now) {
			knocks++
			_ = state.WriteInt64Marker(usageGatePath("%18"), now)
		}
		now += 60 // a minute between samples, well inside the restate interval
	}
	_ = knocks
	if knocks != 1 {
		t.Fatalf("the measured round must collapse to 1 knock, got %d", knocks)
	}

	// And the direction that must survive: past the interval, a breach speaks again.
	if !usageRestateOK("%18", now+usageRestateMinSec) {
		t.Error("a warning must be restatable once the interval has passed")
	}
}

// C15 form 2, and its fix. `burn <total>` was monotonic — once breached it could never
// de-assert, because output only ever grows. An alarm with no exit condition is not a
// warning, it is a permanent label. It now alarms on the PROJECTION, which can clear.
func TestBurnAlarmsOnRateAndCanClear(t *testing.T) {
	l := usage.Layers{CtxWarn: 0.8, SessionOutWarn: 20_000_000, TypeRatePerMinWarn: 30_000}
	const win, h = int64(1_000_000), 30 * time.Minute

	// Far past the line but the session has STOPPED producing → silent. This is the case
	// the old form could not express, and the one that knocked forever.
	for _, out := range []int64{20_000_000, 23_600_000, 100_000_000} {
		if w := usage.Evaluate(l, h, win, 0.1, out, 0); w != "" {
			t.Errorf("out=%d at rate 0 still warns (%q) — a stopped session cannot be burning", out, w)
		}
	}
	// Past the line AND still burning → warns, and says the rate, which is the part that
	// can change.
	w := usage.Evaluate(l, h, win, 0.1, 23_600_000, 20_000)
	if w == "" || !strings.Contains(w, "/m") {
		t.Errorf("a session past the line and still producing must warn with a rate, got %q", w)
	}
	// Under the line, heading for it fast → the projection warns, as before.
	if w := usage.Evaluate(l, h, win, 0.1, 19_000_000, 100_000); w == "" {
		t.Error("an approaching breach must still project")
	}
	// Under the line and crawling → silent.
	if w := usage.Evaluate(l, h, win, 0.1, 1_000_000, 10); w != "" {
		t.Errorf("a slow session far from the line must be silent, got %q", w)
	}
}
