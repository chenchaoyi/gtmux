package codexrpc

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestRBL001CoordinationOnlySteersExactActiveTurn(t *testing.T) {
	for _, tc := range []struct {
		name, status, direct, flags, metaErr, turnErr, receipt string
		wantAttempt, wantAccept                                bool
	}{
		{"active", "active", "true", "[]", "", "", `{"id":4,"result":{"turnId":"turn-a"}}`, true, true},
		{"idle", "idle", "true", "[]", "", "", "", false, false},
		{"unloaded", "notLoaded", "true", "[]", "", "", "", false, false},
		{"approval", "active", "true", `["waitingOnApproval"]`, "", "", "", false, false},
		{"no-direct", "active", "false", "[]", "", "", "", false, false},
		{"metadata-unsupported", "active", "true", "[]", `{"id":2,"error":{"code":-32601}}`, "", "", false, false},
		{"turn-unsupported", "active", "true", "[]", "", `{"id":3,"error":{"code":-32601}}`, "", false, false},
		{"steer-unsupported", "active", "true", "[]", "", "", `{"id":4,"error":{"code":-32601}}`, true, false},
		{"steer-unknown", "active", "true", "[]", "", "", "", true, false},
		{"wrong-turn-receipt", "active", "true", "[]", "", "", `{"id":4,"result":{"turnId":"other"}}`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := fmt.Sprintf(`{"id":2,"result":{"thread":{"id":"root-a","status":{"type":%q,"activeFlags":%s},"canAcceptDirectInput":%s}}}`, tc.status, tc.flags, tc.direct)
			if tc.metaErr != "" {
				meta = tc.metaErr
			}
			turn := `{"id":3,"result":{"data":[{"id":"turn-a","status":"inProgress","itemsView":"notLoaded","items":[]}]}}`
			if tc.turnErr != "" {
				turn = tc.turnErr
			}
			raw := peer(t, false, meta+"\n"+turn+"\n"+tc.receipt)
			claims := 0
			out := CoordinationNotice(context.Background(), testTarget(), "notice-a", "short inbox hint", func(turn string) bool {
				claims++
				if turn != "turn-a" {
					t.Fatal("wrong claimed turn", turn)
				}
				return true
			})
			if out.Attempted != tc.wantAttempt || out.Accepted != tc.wantAccept || (tc.wantAccept && out.ActiveTurnID != "turn-a") {
				t.Fatalf("outcome %+v", out)
			}
			mutations := 0
			for _, req := range requests(t, raw) {
				method := string(req["method"])
				if method == `"thread/queue/add"` || method == `"turn/start"` || strings.Contains(method, "resume") {
					t.Fatal("coordination entered persistent/idle transport", method)
				}
				if method == `"turn/steer"` {
					mutations++
					if !strings.Contains(string(req["params"]), `"expectedTurnId":"turn-a"`) {
						t.Fatal("lost expected turn")
					}
				}
			}
			want := 0
			if tc.wantAttempt {
				want = 1
			}
			if mutations != want || claims != want {
				t.Fatal("mutation/claim count", mutations, claims, want)
			}
		})
	}
}
func TestRBL001CoordinationGuardPreventsMutation(t *testing.T) {
	raw := peer(t, false, `{"id":2,"result":{"thread":{"id":"root-a","status":{"type":"active","activeFlags":[]},"canAcceptDirectInput":true}}}
{"id":3,"result":{"data":[{"id":"turn-a","status":"inProgress","itemsView":"notLoaded","items":[]}]}}`)
	out := CoordinationNotice(context.Background(), testTarget(), "notice-a", "hint", func(string) bool { return false })
	if out.Attempted || out.Accepted {
		t.Fatal(out)
	}
	for _, r := range requests(t, raw) {
		if strings.Contains(string(r["method"]), "steer") || strings.Contains(string(r["method"]), "queue") {
			t.Fatal("rejected guard wrote input")
		}
	}
}
