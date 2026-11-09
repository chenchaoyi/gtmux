package codexrpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const activeMetadata = `{"id":2,"result":{"thread":{"id":"root-a","canAcceptDirectInput":true,"status":{"type":"active","activeFlags":[]}}}}` + "\n"
const activeTurn = `{"id":3,"result":{"data":[{"id":"turn-a","status":"inProgress","itemsView":"notLoaded","items":[]}]}}` + "\n"

func screenshotInputs() []Input {
	return []Input{{Type: "text", Text: "synthetic screenshot"}, {Type: "localImage", Path: "/synthetic/uploads/image.png"}}
}

func TestSSI001ActiveScreenshotUsesExactTurnAndMetadataOnly(t *testing.T) {
	raw := peer(t, false, activeMetadata+activeTurn+`{"id":4,"result":{"turnId":"turn-a"}}`)
	checks := 0
	got := ImageInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), func() bool { checks++; return true })
	if !got.Accepted || !got.Attempted || got.ActiveTurnID != "turn-a" || got.QueueID != "" || checks != 1 {
		t.Fatal(got, checks)
	}
	wire := requests(t, raw)
	want := []string{"initialize", "initialized", "thread/read", "thread/turns/list", "turn/steer"}
	if len(wire) != len(want) {
		t.Fatal(wire)
	}
	for i, name := range want {
		if string(wire[i]["method"]) != `"`+name+`"` {
			t.Fatal(wire)
		}
	}
	var meta map[string]any
	json.Unmarshal(wire[2]["params"], &meta)
	if meta["includeTurns"] != false || meta["threadId"] != "root-a" {
		t.Fatal(meta)
	}
	var turns map[string]any
	json.Unmarshal(wire[3]["params"], &turns)
	if turns["itemsView"] != "notLoaded" || turns["limit"] != float64(1) || turns["sortDirection"] != "desc" {
		t.Fatal(turns)
	}
	var input struct {
		ThreadID string  `json:"threadId"`
		Turn     string  `json:"expectedTurnId"`
		Message  string  `json:"clientUserMessageId"`
		Input    []Input `json:"input"`
	}
	json.Unmarshal(wire[4]["params"], &input)
	if input.ThreadID != "root-a" || input.Turn != "turn-a" || input.Message != "message-a" || len(input.Input) != 2 || input.Input[1] != screenshotInputs()[1] {
		t.Fatal(input)
	}
}

func TestSSI001OnlyExplicitUnsupportedOrIdleUsesLegacyQueue(t *testing.T) {
	for name, response := range map[string]string{
		"unsupported-metadata":  `{"id":2,"error":{"code":-32601}}` + "\n" + `{"id":3,"result":{"queuedSubmission":{"id":"q-a","clientUserMessageId":"message-a"}}}`,
		"unsupported-notLoaded": activeMetadata + `{"id":3,"error":{"code":-32602}}` + "\n" + `{"id":4,"result":{"queuedSubmission":{"id":"q-a","clientUserMessageId":"message-a"}}}`,
		"unsupported-steer":     activeMetadata + activeTurn + `{"id":4,"error":{"code":-32601}}` + "\n" + `{"id":5,"result":{"queuedSubmission":{"id":"q-a","clientUserMessageId":"message-a"}}}`,
		"idle":                  `{"id":2,"result":{"thread":{"id":"root-a","canAcceptDirectInput":true,"status":{"type":"idle"}}}}` + "\n" + `{"id":3,"result":{"queuedSubmission":{"id":"q-a","clientUserMessageId":"message-a"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := peer(t, false, response)
			got := ImageInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), func() bool { return true })
			if !got.Accepted || got.ActiveTurnID != "" || got.QueueID != "q-a" {
				t.Fatal(got)
			}
			wire := requests(t, raw)
			queues := 0
			for _, r := range wire {
				if string(r["method"]) == `"thread/queue/add"` {
					queues++
				}
			}
			if queues != 1 {
				t.Fatal(wire)
			}
		})
	}
}

func TestSSI001RefusalsAndUncertaintyNeverMutateOrReplay(t *testing.T) {
	for name, response := range map[string]string{
		"wrong-thread":         strings.Replace(activeMetadata, `"root-a"`, `"other"`, 1),
		"approval":             strings.Replace(activeMetadata, `"activeFlags":[]`, `"activeFlags":["waitingOnApproval"]`, 1),
		"unknown-flag":         strings.Replace(activeMetadata, `"activeFlags":[]`, `"activeFlags":["futureFlag"]`, 1),
		"metadata-error":       `{"id":2,"error":{"code":-32603}}`,
		"turn-completed":       activeMetadata + strings.Replace(activeTurn, "inProgress", "completed", 1),
		"turn-items-not-empty": activeMetadata + strings.Replace(activeTurn, `"items":[]`, `"items":[{"type":"userMessage"}]`, 1),
		"turns-fail":           activeMetadata + `{"id":3,"error":{"code":-32603}}`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := peer(t, false, response)
			got := ImageInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), func() bool { return true })
			if got.Accepted || got.Attempted {
				t.Fatal(got)
			}
			for _, r := range requests(t, raw) {
				if string(r["method"]) == `"turn/steer"` || string(r["method"]) == `"thread/queue/add"` {
					t.Fatal("unexpected mutation", r)
				}
			}
		})
	}
	for name, end := range map[string]string{"missing": "", "wrong-turn": `{"id":4,"result":{"turnId":"other"}}`, "generic": `{"id":4,"error":{"code":-32603}}`, "changed-turn": `{"id":4,"error":{"code":-32600}}`} {
		t.Run(name, func(t *testing.T) {
			raw := peer(t, false, activeMetadata+activeTurn+end)
			got := ImageInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), func() bool { return true })
			if got.Accepted || !got.Attempted {
				t.Fatal(got)
			}
			wire := requests(t, raw)
			if len(wire) != 5 || string(wire[4]["method"]) != `"turn/steer"` {
				t.Fatal("unsafe fallback", wire)
			}
		})
	}
}

func TestSSI001LastMomentHoldStopsActiveInput(t *testing.T) {
	raw := peer(t, false, activeMetadata+activeTurn)
	got := ImageInputs(context.Background(), testTarget(), "message-a", screenshotInputs(), func() bool { return false })
	if got.Accepted || got.Attempted || len(requests(t, raw)) != 4 {
		t.Fatal(got)
	}
}
