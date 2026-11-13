package codexrpc

import (
	"context"
	"strings"
	"testing"
)

func TestSCW005UnknownFlagsRefuseWithoutInventingHumanApproval(t *testing.T) {
	for _, entry := range []struct {
		name, flags, reason string
	}{
		{"approval", `["waitingOnApproval"]`, "input-awaiting-confirmation"},
		{"user-input", `["waitingOnUserInput"]`, "input-awaiting-confirmation"},
		{"future", `["futureFlag"]`, "input-state-unavailable"},
		{"mixed", `["waitingOnApproval","futureFlag"]`, "input-state-unavailable"},
	} {
		for _, mode := range []string{"image", "user", "coordination"} {
			t.Run(entry.name+"/"+mode, func(t *testing.T) {
				meta := strings.Replace(activeMetadata, `"activeFlags":[]`, `"activeFlags":`+entry.flags, 1)
				raw := peer(t, false, meta)
				checks := 0
				check := func() bool { checks++; return true }
				var result Outcome
				switch mode {
				case "image":
					result = ImageInputs(context.Background(), testTarget(), "synthetic-scw", screenshotInputs(), check)
				case "user":
					result = UserInputs(context.Background(), testTarget(), "synthetic-scw", screenshotInputs(), check)
				case "coordination":
					result = CoordinationNotice(context.Background(), testTarget(), "synthetic-scw", "synthetic notice", func(string) bool { return check() })
				}
				if result.Accepted || result.Attempted || result.Reason != entry.reason || checks != 0 {
					t.Fatalf("outcome=%+v checks=%d want reason=%s", result, checks, entry.reason)
				}
				for _, req := range requests(t, raw) {
					if method := string(req["method"]); method == `"turn/steer"` || method == `"turn/start"` || method == `"thread/queue/add"` {
						t.Fatalf("refusal mutated input: %s", method)
					}
				}
			})
		}
	}
}
