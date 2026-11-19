package codexrpc

import (
	"context"
	"testing"
)

func TestSPL022WireChecksCancellationAroundFinalGuard(t *testing.T) {
	for _, phase := range []string{"before", "after"} {
		t.Run(phase, func(t *testing.T) {
			raw := peer(t, false, "")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wantChecks := 1
			if phase == "before" {
				cancel()
				wantChecks = 0
			}
			calls := 0
			out := QueueInputs(ctx, testTarget(), "fixture", []Input{{Type: "text", Text: "synthetic"}}, func() bool {
				calls++
				cancel()
				return true
			})
			wire := requests(t, raw)
			if out.Attempted || out.Accepted || out.Reason != "input-deadline" || calls != wantChecks || len(wire) != 2 {
				t.Fatalf("code=%+v, checks=%d, wire=%v", out, calls, wire)
			}
		})
	}
}
