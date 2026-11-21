package codexrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestSeedThreadNameRequiresExactCodexAcknowledgement(t *testing.T) {
	const title = "gtmux HQ · 舰队中控"
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	target := Target{HomeID: transcript.CodexHomeID(), ThreadID: "hq-root", BackendPID: 123, BackendStart: "daemon-birth"}
	previous := connect
	t.Cleanup(func() { connect = previous })
	for _, tc := range []struct {
		label, thread, name, ack, want, why string
		foreignHome, foreignPeer, revoked   bool
	}{
		{label: "accepted", thread: "hq-root", ack: `"result":{}`, want: title},
		{label: "user name", thread: "hq-root", name: "User title", want: "User title"},
		{label: "foreign backend", foreignPeer: true, why: "backend-changed"},
		{label: "foreign home", foreignHome: true, why: "server-home-mismatch"},
		{label: "foreign thread", thread: "different-id", why: "thread-read-unavailable"},
		{label: "role lost", thread: "hq-root", revoked: true, why: "title-target-changed"},
		{label: "unsupported", thread: "hq-root", ack: `"error":{"code":-32601}`, why: "title-unconfirmed"},
		{label: "missing acknowledgement", thread: "hq-root", ack: `"result":null`, why: "title-unconfirmed"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			var writes bytes.Buffer
			called := false
			serverHome, peer := home, target.BackendPID
			if tc.foreignHome {
				serverHome = t.TempDir()
			}
			if tc.foreignPeer {
				peer++
			}
			responses := fmt.Sprintf("{\"id\":1,\"result\":{\"codexHome\":%q}}\n{\"id\":2,\"result\":{\"thread\":{\"id\":%q,\"name\":%q}}}\n", serverHome, tc.thread, tc.name)
			if tc.ack != "" {
				responses += "{\"id\":3," + tc.ack + "}\n"
			}
			connect = func(context.Context) (connection, error) {
				return connection{peerPID: peer, peerStart: target.BackendStart, reader: strings.NewReader(responses), writer: &writes, close: func() {}}, nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, why := SeedThreadName(ctx, target, title, func() bool {
				called = true
				if !strings.Contains(writes.String(), "thread/read") || strings.Contains(writes.String(), "thread/name/set") {
					t.Fatalf("authority guard not between read and write: %s", &writes)
				}
				return !tc.revoked
			})
			if got != tc.want || why != tc.why {
				t.Fatalf("name = %q (%s), want %q (%s)", got, why, tc.want, tc.why)
			}
			var methods []string
			dec := json.NewDecoder(&writes)
			for dec.More() {
				var req struct {
					Method string                     `json:"method"`
					Params map[string]json.RawMessage `json:"params"`
				}
				if err := dec.Decode(&req); err != nil {
					t.Fatal(err)
				}
				methods = append(methods, req.Method)
				if req.Method == "thread/name/set" {
					var name string
					if err := json.Unmarshal(req.Params["name"], &name); err != nil || name != title ||
						string(req.Params["threadId"]) != `"hq-root"` {
						t.Fatalf("unexpected name request: %+v", req)
					}
				}
				if req.Method == "thread/read" && (string(req.Params["threadId"]) != `"hq-root"` || string(req.Params["includeTurns"]) != "false") {
					t.Fatalf("read must address only the thread metadata: %+v", req)
				}
			}
			wantMethods := "initialize,initialized,thread/read"
			switch {
			case tc.foreignPeer:
				wantMethods = ""
			case tc.foreignHome:
				wantMethods = "initialize"
			case tc.ack != "":
				wantMethods += ",thread/name/set"
			}
			if strings.Join(methods, ",") != wantMethods || called != (tc.thread == target.ThreadID) {
				t.Fatalf("unexpected requests %v or authority check %v", methods, called)
			}
		})
	}
}
