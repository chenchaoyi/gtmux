package codexrpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMSD001ExplicitUserInputSelectsExactActiveOrLoadedIdleTurn(t *testing.T) {
	for _, tc := range []struct {
		name, metadata, tail, method string
		turn, started                string
	}{
		{"active", activeMetadata + activeTurn, `{"id":4,"result":{"turnId":"turn-a"}}`, "turn/steer", "turn-a", ""},
		{"idle", `{"id":2,"result":{"thread":{"id":"root-a","canAcceptDirectInput":true,"status":{"type":"idle"}}}}` + "\n", `{"id":3,"result":{"turn":{"id":"new-turn","status":"inProgress"}}}`, "turn/start", "", "new-turn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := peer(t, false, tc.metadata+tc.tail)
			checks := 0
			got := UserInputs(context.Background(), testTarget(), "message-a", []Input{{Type: "text", Text: "synthetic prompt"}}, func() bool { checks++; return true })
			if !got.Accepted || !got.Attempted || got.QueueID != "" || got.ActiveTurnID != tc.turn || got.StartedTurnID != tc.started || checks != 1 {
				t.Fatal(got, checks)
			}
			wire := requests(t, raw)
			last := wire[len(wire)-1]
			if string(last["method"]) != `"`+tc.method+`"` {
				t.Fatal(wire)
			}
			var p map[string]any
			json.Unmarshal(last["params"], &p)
			if p["threadId"] != "root-a" || p["clientUserMessageId"] != "message-a" || len(p["input"].([]any)) != 1 {
				t.Fatal(p)
			}
			want := 3
			if tc.turn != "" {
				want = 4
				if p["expectedTurnId"] != tc.turn {
					t.Fatal(p)
				}
			}
			if len(p) != want {
				t.Fatal("unexpected model/permission/cwd or queue mutation", p)
			}
		})
	}
}

func TestMSD001LegacyQueueRemainsExplicitAndOnlyUnknownMethodCanFallback(t *testing.T) {
	queue := `{"result":{"queuedSubmission":{"id":"q-a","clientUserMessageId":"message-a"}}}`
	for _, tc := range []struct{ name, response string }{
		{"metadata-unsupported", `{"id":2,"error":{"code":-32601}}` + "\n" + strings.Replace(queue, `{"result"`, `{"id":3,"result"`, 1)},
		{"start-unsupported", `{"id":2,"result":{"thread":{"id":"root-a","canAcceptDirectInput":true,"status":{"type":"idle"}}}}` + "\n" + `{"id":3,"error":{"code":-32601}}` + "\n" + strings.Replace(queue, `{"result"`, `{"id":4,"result"`, 1)},
		{"steer-unsupported", activeMetadata + activeTurn + `{"id":4,"error":{"code":-32601}}` + "\n" + strings.Replace(queue, `{"result"`, `{"id":5,"result"`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := peer(t, false, tc.response)
			got := UserInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), func() bool { return true })
			if !got.Accepted || got.QueueID != "q-a" || got.ActiveTurnID != "" || got.StartedTurnID != "" {
				t.Fatal(got)
			}
			wire := requests(t, raw)
			if string(wire[len(wire)-1]["method"]) != `"thread/queue/add"` {
				t.Fatal(wire)
			}
		})
	}
	// This is the historical phone operation: it still queues by design for CLI/HQ.
	raw := peer(t, false, `{"id":2,"result":{"queuedSubmission":{"id":"q-a","clientUserMessageId":"message-a"}}}`)
	got := QueueInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), nil)
	if !got.Accepted || got.QueueID != "q-a" || len(requests(t, raw)) != 3 {
		t.Fatal(got)
	}
}

func TestMSD001UserInputRefusesUnsafeMetadataAndNeverReplaysAmbiguousWrite(t *testing.T) {
	for name, response := range map[string]string{
		"wrong-home": "", "wrong-thread": strings.Replace(activeMetadata, "root-a", "wrong", 1),
		"not-loaded":   strings.Replace(activeMetadata, `"active"`, `"notLoaded"`, 1),
		"approval":     strings.Replace(activeMetadata, `"activeFlags":[]`, `"activeFlags":["waitingOnApproval"]`, 1),
		"turn-changed": activeMetadata + strings.Replace(activeTurn, "inProgress", "completed", 1),
	} {
		t.Run(name, func(t *testing.T) {
			raw := peer(t, name == "wrong-home", response)
			got := UserInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), nil)
			if got.Accepted || got.Attempted {
				t.Fatal(got)
			}
			for _, r := range requests(t, raw) {
				m := string(r["method"])
				if m == `"turn/start"` || m == `"turn/steer"` || m == `"thread/queue/add"` || strings.Contains(m, "resume") {
					t.Fatal(r)
				}
			}
		})
	}
	for _, start := range []bool{false, true} {
		for _, tail := range []string{"", `{"error":{"code":-32603}}`, `{"error":{"code":-32600}}`, `{"result":{"turnId":"wrong"}}`} {
			meta, id := activeMetadata+activeTurn, "4"
			if start {
				meta = `{"id":2,"result":{"thread":{"id":"root-a","canAcceptDirectInput":true,"status":{"type":"idle"}}}}` + "\n"
				id = "3"
			}
			if tail != "" {
				tail = strings.Replace(tail, "{", `{"id":`+id+`,`, 1)
			}
			raw := peer(t, false, meta+tail)
			got := UserInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), func() bool { return true })
			if got.Accepted || !got.Attempted {
				t.Fatal(got)
			}
			for _, r := range requests(t, raw) {
				if string(r["method"]) == `"thread/queue/add"` {
					t.Fatal("uncertain direct input replayed", r)
				}
			}
		}
	}
	raw := peer(t, false, activeMetadata+activeTurn)
	got := UserInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), func() bool { return false })
	if got.Accepted || got.Attempted || len(requests(t, raw)) != 4 {
		t.Fatal(got)
	}
}
