package hook

import (
	"testing"
	"time"

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
	// ctx dithers a hair either side of 0.80. Rate 0 keeps the projections out of it,
	// so nothing here depends on timing — only on which layer gets to speak.
	ctx := []float64{0.81, 0.79, 0.81, 0.79, 0.81}

	prior, knocks := "", 0
	for _, c := range ctx {
		warn := usage.Evaluate(l, 30*time.Minute, win, c, burned, 0)
		if warn != "" && layerOf(warn) != layerOf(prior) {
			knocks++ // watchUsage's rule verbatim: a different layer nudges
			if layerOf(warn) == "burn" {
				t.Logf("knock %d: %q (the SAME 23.6M breach, re-announced)", knocks, warn)
			}
		}
		prior = warn
	}
	if knocks < 3 {
		t.Fatalf("expected the measured repeat shape (≥3 knocks from one steady breach), got %d", knocks)
	}

	// The control: hold ctx still and the same breach speaks exactly once, which is why
	// the dedup looked correct in every test written for it.
	prior, knocks = "", 0
	for i := 0; i < 5; i++ {
		warn := usage.Evaluate(l, 30*time.Minute, win, 0.5, burned, 0)
		if warn != "" && layerOf(warn) != layerOf(prior) {
			knocks++
		}
		prior = warn
	}
	if knocks != 1 {
		t.Fatalf("a steady world must knock once, got %d", knocks)
	}
}

// The second half of C15 form 2, stated as a test: `burn <total>` is monotonic, so once
// breached it can NEVER de-assert. No amount of dedup fixes an alarm with no exit.
func TestAudit_BurnTotalHasNoDeAssertCondition(t *testing.T) {
	l := usage.Layers{CtxWarn: 0.8, SessionOutWarn: 20_000_000, TypeRatePerMinWarn: 30_000}
	for _, out := range []int64{20_000_000, 23_600_000, 100_000_000} {
		if w := usage.Evaluate(l, 30*time.Minute, 1_000_000, 0.1, out, 0); w == "" {
			t.Fatalf("out=%d unexpectedly cleared", out)
		}
	}
	// Output only ever grows within a session, so there is no input to this function
	// that turns the alarm off again — it is on for the session's remaining life.
}
