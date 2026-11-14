package codexrpc

import (
	"context"
	"strings"
	"testing"
)

func TestHRW001AutomaticWakeUsesFreshIdleWithoutQueueOrSteer(t *testing.T) {
	idle := `{"id":2,"result":{"thread":{"id":"root-a","canAcceptDirectInput":true,"status":{"type":"idle"}}}}` + "\n"
	for _, tc := range []struct {
		name, meta, tail    string
		accepted, attempted bool
	}{
		{"idle", idle, `{"id":3,"result":{"turn":{"id":"ack-turn","status":"inProgress"}}}`, true, true},
		// Real protocol shape allows this return after an external writer wins the
		// sample-to-start race. This test exposes that limit; it does not prove an OS race.
		{"externally-active-after-sample", idle, `{"id":3,"result":{"turn":{"id":"existing-external-turn","status":"inProgress"}}}`, true, true},
		{"busy", activeMetadata + activeTurn, "", false, false},
		{"not-loaded", strings.Replace(idle, `"idle"`, `"notLoaded"`, 1), "", false, false},
		{"unknown-state", strings.Replace(idle, `"idle"`, `"future-state"`, 1), "", false, false},
		{"approval", strings.Replace(idle, `"type":"idle"`, `"type":"idle","activeFlags":["waitingOnApproval"]`, 1), "", false, false},
		{"no-direct", strings.Replace(idle, `"canAcceptDirectInput":true,`, "", 1), "", false, false},
		{"unsupported-metadata", `{"id":2,"error":{"code":-32601}}`, "", false, false},
		{"mismatched-target", strings.Replace(idle, "root-a", "foreign", 1), "", false, false},
		{"lost-start-receipt", idle, "", false, true},
		{"start-unsupported", idle, `{"id":3,"error":{"code":-32601}}`, false, false},
		{"start-error", idle, `{"id":3,"error":{"code":-32603}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := peer(t, false, tc.meta+tc.tail)
			claims := 0
			got := IdleWakeInputs(context.Background(), testTarget(), "wake-id", []Input{{Type: "text", Text: "synthetic wake"}}, func() bool { claims++; return true })
			if got.Accepted != tc.accepted || got.Attempted != tc.attempted || got.StartedTurnID != "" || got.ActiveTurnID != "" || got.QueueID != "" {
				t.Fatal(got)
			}
			starts := 0
			for _, r := range requests(t, raw) {
				m := string(r["method"])
				if m == `"turn/start"` {
					starts++
				}
				if m == `"turn/steer"` || strings.Contains(m, "queue") || strings.Contains(m, "resume") {
					t.Fatal("automatic wake used future/active input", r)
				}
			}
			expected := 0
			if strings.HasPrefix(tc.meta, idle) {
				expected = 1
			}
			if starts != expected || claims != expected {
				t.Fatal("unexpected mutation/claim count", starts, claims, expected)
			}
			if tc.accepted && got.AcknowledgedTurnID == "" {
				t.Fatal("missing exact RPC receipt", got)
			}
			if tc.name == "externally-active-after-sample" && got.AcknowledgedTurnID != "existing-external-turn" {
				t.Fatal("invented causal start", got)
			}
		})
	}
	raw := peer(t, false, idle)
	got := IdleWakeInputs(context.Background(), testTarget(), "wake-id", []Input{{Type: "text", Text: "synthetic wake"}}, func() bool { return false })
	if got.Attempted || got.Accepted || len(requests(t, raw)) != 3 {
		t.Fatal("changed target claimed input", got)
	}
}
