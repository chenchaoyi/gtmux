package codexrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// The fake peer exposes wire requests so a wrong home/thread or an unsafe retry
// decision fails independently of the implementation's internal bookkeeping.
func peer(t *testing.T, wrongHome bool, response string) *bytes.Buffer {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	if wrongHome {
		home = t.TempDir()
	}
	init, err := json.Marshal(map[string]any{"id": 1, "result": map[string]any{"codexHome": home}})
	if err != nil {
		t.Fatal(err)
	}
	requests := &bytes.Buffer{}
	old := connect
	t.Cleanup(func() { connect = old })
	connect = func(context.Context) (connection, error) {
		return connection{reader: strings.NewReader(string(init) + "\n" + response), writer: requests, close: func() {}}, nil
	}
	return requests
}

func testTarget() Target { return Target{HomeID: transcript.CodexHomeID(), ThreadID: "root-a"} }

func requests(t *testing.T, raw *bytes.Buffer) []map[string]json.RawMessage {
	t.Helper()
	var out []map[string]json.RawMessage
	d := json.NewDecoder(raw)
	for {
		var v map[string]json.RawMessage
		err := d.Decode(&v)
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
}

func TestQueueTargetsExactConversation(t *testing.T) {
	raw := peer(t, false, `{"method":"unrelated/notification","params":{}}
{"id":2,"result":{"queuedSubmission":{"id":"q-1","clientUserMessageId":"message-a"}}}`)
	got := Queue(context.Background(), testTarget(), "message-a", "synthetic input")
	if !got.Accepted || !got.Attempted || got.QueueID != "q-1" {
		t.Fatalf("acceptance = %+v", got)
	}
	wire := requests(t, raw)
	if len(wire) != 3 || string(wire[0]["method"]) != `"initialize"` || string(wire[1]["method"]) != `"initialized"` || string(wire[2]["method"]) != `"thread/queue/add"` {
		t.Fatalf("unexpected requests: %v", wire)
	}
	var params struct {
		ThreadID  string `json:"threadId"`
		MessageID string `json:"clientUserMessageId"`
		Input     []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"input"`
	}
	if err := json.Unmarshal(wire[2]["params"], &params); err != nil {
		t.Fatal(err)
	}
	if params.ThreadID != "root-a" || params.MessageID != "message-a" || len(params.Input) != 1 || params.Input[0].Type != "text" || params.Input[0].Text != "synthetic input" {
		t.Fatalf("wrong input target: %+v", params)
	}
}

func TestQueueRefusesDifferentServerHomeBeforeInput(t *testing.T) {
	raw := peer(t, true, "")
	got := Queue(context.Background(), testTarget(), "message-a", "synthetic input")
	if got.Accepted || got.Attempted || got.Reason != "server-home-mismatch" || len(requests(t, raw)) != 1 {
		t.Fatalf("different home allowed input: %+v", got)
	}
}

func TestQueueDoesNotReplayMissingOrUncertainReceipts(t *testing.T) {
	for name, response := range map[string]string{
		"eof":           "",
		"malformed":     `{broken`,
		"wrong-message": `{"id":2,"result":{"queuedSubmission":{"id":"q-1","clientUserMessageId":"other"}}}`,
		"missing-id":    `{"id":2,"result":{"queuedSubmission":{"clientUserMessageId":"message-a"}}}`,
		"server-error":  `{"id":2,"error":{"code":-32603,"message":"synthetic internal failure"}}`,
		"oversize":      `{"method":"x","params":"` + strings.Repeat("x", 2<<20) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := peer(t, false, response)
			got := Queue(context.Background(), testTarget(), "message-a", "synthetic input")
			if got.Accepted || !got.Attempted || got.Unsupported || len(requests(t, raw)) != 3 {
				t.Fatalf("unsafe replay decision: %+v", got)
			}
		})
	}
}

func TestQueueRecognizesPreExecutionRejections(t *testing.T) {
	for _, code := range []int{-32601, -32602} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			peer(t, false, fmt.Sprintf(`{"id":2,"error":{"code":%d}}`, code))
			got := Queue(context.Background(), testTarget(), "message-a", "synthetic input")
			if got.Accepted || got.Attempted || got.Unsupported != (code == -32601) {
				t.Fatalf("rejection = %+v", got)
			}
		})
	}
}

func TestQueueRejectsInvalidScopeBeforeConnecting(t *testing.T) {
	peer(t, false, "")
	connect = func(context.Context) (connection, error) {
		t.Fatal("invalid scope reached transport")
		return connection{}, nil
	}
	got := Queue(context.Background(), Target{HomeID: "foreign-home", ThreadID: "root-a"}, "message-a", "synthetic input")
	if got.Accepted || got.Attempted || got.Reason != "invalid-input-target" {
		t.Fatalf("invalid scope = %+v", got)
	}
}

func TestQueueRefusesChangedBackendAndRevokedTargetBeforeMutation(t *testing.T) {
	t.Run("backend", func(t *testing.T) {
		raw := peer(t, false, "")
		target := testTarget()
		target.BackendPID, target.BackendStart = 200, "expected"
		got := Queue(context.Background(), target, "message-a", "synthetic")
		if got.Attempted || got.Accepted || got.Reason != "backend-changed" || raw.Len() != 0 {
			t.Fatal("changed backend received input", got)
		}
	})
	t.Run("destination", func(t *testing.T) {
		raw := peer(t, false, "")
		got := QueueChecked(context.Background(), testTarget(), "message-a", "synthetic", func() bool { return false })
		if got.Attempted || got.Accepted || got.Reason != "target-changed" || len(requests(t, raw)) != 2 {
			t.Fatal("revoked destination received input", got)
		}
	})
}
