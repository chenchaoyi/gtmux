package codexbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
	"github.com/gorilla/websocket"
)

func TestWTR005CanonicalProjectDecisionsKeepResponseUnchanged(t *testing.T) {
	for _, kind := range []string{"checkout-denied", "literal-deny-alias-cwd", "ancestor-denied", "main-denied", "cwd-child-denied", "unknown-level", "existing-trusted-checkout", "dangling-denied", "dangling-trusted", "relative-denied", "unrelated-owned-denied"} {
		t.Run(kind, func(t *testing.T) {
			adapter, request, body, cwd := trustConfigFixture(t)
			var reply map[string]any
			if err := json.Unmarshal(body, &reply); err != nil {
				t.Fatal(err)
			}
			projects := reply["result"].(map[string]any)["config"].(map[string]any)["projects"].(map[string]any)
			alias := filepath.Join(filepath.Dir(cwd), "policy-alias")
			target, level := cwd, "untrusted"
			switch kind {
			case "ancestor-denied":
				target = filepath.Dir(cwd)
			case "main-denied":
				for root := range projects {
					target = root
				}
			case "cwd-child-denied":
				target = filepath.Join(cwd, "child")
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				var req map[string]any
				json.Unmarshal(request, &req)
				req["params"].(map[string]any)["cwd"] = target
				request, _ = json.Marshal(req)
			case "unknown-level":
				level = "future-decision"
			case "existing-trusted-checkout":
				level = "trusted"
			case "dangling-denied", "dangling-trusted":
				target = filepath.Join(filepath.Dir(cwd), "missing")
				if kind == "dangling-trusted" {
					level = "trusted"
				}
			case "relative-denied":
				alias = "relative-policy"
			case "unrelated-owned-denied":
				target = t.TempDir()
			}
			if kind != "relative-denied" {
				if err := os.Symlink(target, alias); err != nil {
					t.Fatal(err)
				}
			}
			key := alias
			if kind == "literal-deny-alias-cwd" {
				var req map[string]any
				json.Unmarshal(request, &req)
				req["params"].(map[string]any)["cwd"] = alias
				request, _ = json.Marshal(req)
				key = cwd
			}
			projects[key] = map[string]any{"trust_level": level}
			body, _ = json.Marshal(reply)
			adapter.request(request)
			got := adapter.response(body)
			if kind == "unrelated-owned-denied" {
				if bytes.Equal(got, body) {
					t.Fatal("unrelated stable decision disabled eligible inheritance")
				}
			} else if !bytes.Equal(got, body) {
				t.Fatal("canonical refusal/unknown decision was changed", kind)
			}
		})
	}
}

func TestWTR005ActualUnixForwardingKeepsCanonicalDenyAndSameHomeGuards(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{false: "cross-home", true: "same-home-canonical-deny"}[same], func(t *testing.T) {
			main, cwd, _ := registeredTrustFixture(t)
			home := t.TempDir()
			alias := filepath.Join(filepath.Dir(cwd), "denied")
			if err := os.Symlink(cwd, alias); err != nil {
				t.Fatal(err)
			}
			client, down := trustUnixPair(t)
			up, backend := trustUnixPair(t)
			observer := &observer{home: transcript.CodexHomeIDForPath(home), bind: func(string, string) { t.Error("unexpected conversation binding") }}
			ctx, cancel := context.WithCancel(context.Background())
			adapter := newTrustConfig(ctx, observer.homeVerified)
			done := make(chan error, 2)
			go func() { done <- forward(down, up, func(b []byte) { observer.request(b); adapter.request(b) }) }()
			go func() { done <- forward(up, down, observer.response, adapter.response) }()
			defer func() {
				cancel()
				client.Close()
				down.Close()
				up.Close()
				backend.Close()
				for i := 0; i < 2; i++ {
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Error("forwarder leaked")
					}
				}
			}()
			exchange := func(request, reply []byte) []byte {
				t.Helper()
				if err := client.WriteMessage(websocket.TextMessage, request); err != nil {
					t.Fatal(err)
				}
				_, got, err := backend.ReadMessage()
				if err != nil || !bytes.Equal(request, got) {
					t.Fatal("request changed", err)
				}
				if err = backend.WriteMessage(websocket.TextMessage, reply); err != nil {
					t.Fatal(err)
				}
				_, got, err = client.ReadMessage()
				if err != nil {
					t.Fatal(err)
				}
				return got
			}
			initHome := home
			if !same {
				initHome = t.TempDir()
			}
			initialize, _ := json.Marshal(map[string]any{"id": 1, "result": map[string]any{"codexHome": initHome}})
			exchange([]byte(`{"id":1,"method":"initialize"}`), initialize)
			request, _ := json.Marshal(map[string]any{"id": 2, "method": "config/read", "params": map[string]any{"cwd": cwd, "includeLayers": true}})
			body, _ := json.Marshal(map[string]any{"id": 2, "result": map[string]any{"config": map[string]any{"projects": map[string]any{main: map[string]any{"trust_level": "trusted"}, alias: map[string]any{"trust_level": "untrusted"}}, "approval_policy": "on-request"}, "layers": []any{}}})
			if got := exchange(request, body); !bytes.Equal(got, body) {
				t.Fatal("real forwarding promoted a canonically denied checkout")
			}
		})
	}
}

func TestWTR005CanonicalObservationsAreStableAndBounded(t *testing.T) {
	for _, kind := range []string{"same-inode-other-spelling", "changed-identity", "unknown-owner", "project-limit", "cancelled-read"} {
		t.Run(kind, func(t *testing.T) {
			adapter, request, body, cwd := trustConfigFixture(t)
			original := adapter.resolveProject
			if kind == "same-inode-other-spelling" || kind == "project-limit" {
				var reply map[string]any
				json.Unmarshal(body, &reply)
				projects := reply["result"].(map[string]any)["config"].(map[string]any)["projects"].(map[string]any)
				if kind == "project-limit" {
					for i := 0; i < trustProjectLimit; i++ {
						projects[filepath.Join(cwd, fmt.Sprintf("unknown-%d", i))] = map[string]any{"trust_level": "trusted"}
					}
				} else {
					other := t.TempDir()
					projects[other] = map[string]any{"trust_level": "untrusted"}
					seal, ok := ownedTrustDirectory(cwd)
					if !ok {
						t.Fatal("fixture identity")
					}
					adapter.resolveProject = func(ctx context.Context, path string) (trustProjectPath, bool) {
						if path == other {
							return trustProjectPath{Canonical: other, Seal: seal}, true
						}
						return original(ctx, path)
					}
				}
				body, _ = json.Marshal(reply)
			}
			switch kind {
			case "changed-identity":
				count := 0
				adapter.resolveProject = func(ctx context.Context, path string) (trustProjectPath, bool) {
					p, ok := original(ctx, path)
					count++
					if count == 2 {
						p.Seal += "changed"
					}
					return p, ok
				}
			case "unknown-owner":
				adapter.resolveProject = func(context.Context, string) (trustProjectPath, bool) { return trustProjectPath{}, false }
			case "project-limit":
				adapter.resolveProject = func(context.Context, string) (trustProjectPath, bool) {
					t.Error("oversized policy started filesystem resolution")
					return trustProjectPath{}, false
				}
			case "cancelled-read":
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				adapter.ctx = ctx
				entered, release, settled := make(chan struct{}), make(chan struct{}), make(chan struct{})
				adapter.resolveProject = func(ctx context.Context, path string) (trustProjectPath, bool) {
					close(entered)
					<-release
					defer close(settled)
					return original(ctx, path)
				}
				adapter.request(request)
				done := make(chan []byte, 1)
				go func() { done <- adapter.response(body) }()
				<-entered
				cancel()
				select {
				case got := <-done:
					if !bytes.Equal(got, body) {
						t.Error("cancelled resolver authorized")
					}
				case <-time.After(time.Second):
					t.Error("cancelled resolver blocked forwarding")
				}
				// Even after the caller returns, the still-running resolver holds its one
				// proof slot. A new connection-scoped request must not replace it.
				adapter.ctx = context.Background()
				var nextRequest, nextBody map[string]any
				json.Unmarshal(request, &nextRequest)
				json.Unmarshal(body, &nextBody)
				nextRequest["id"] = 8
				nextBody["id"] = 8
				req2, _ := json.Marshal(nextRequest)
				body2, _ := json.Marshal(nextBody)
				adapter.request(req2)
				if !bytes.Equal(adapter.response(body2), body2) {
					t.Error("pending resolver was oversubscribed")
				}
				close(release)
				select {
				case <-settled:
				case <-time.After(time.Second):
					t.Error("resolver did not exit")
				}
				return
			}
			adapter.request(request)
			if !bytes.Equal(adapter.response(body), body) {
				t.Fatal("unknown or changing canonical policy was promoted", kind)
			}
		})
	}
}
