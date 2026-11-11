package codexbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func trustConfigFixture(t *testing.T) (*trustConfig, []byte, []byte, string) {
	t.Helper()
	main, checkout, _ := registeredTrustFixture(t)
	adapter := newTrustConfig(context.Background(), func() bool { return true })
	request, _ := json.Marshal(map[string]any{"id": 7, "method": "config/read", "params": map[string]any{"cwd": checkout, "includeLayers": true}})
	response, _ := json.Marshal(map[string]any{"id": 7, "result": map[string]any{"config": map[string]any{"model": "synthetic-model", "projects": map[string]any{main: map[string]any{"trust_level": "trusted"}}, "approval_policy": "on-request"}, "layers": []any{}, "origins": map[string]any{}}})
	return adapter, request, response, checkout
}

func TestWTR002AliasMirrorsOnlyExistingEffectiveRootTrustAndKeepsOtherFields(t *testing.T) {
	a, req, body, cwd := trustConfigFixture(t)
	a.request(req)
	out := a.response(body)
	if bytes.Equal(out, body) {
		t.Fatal("existing trusted worktree fallback was omitted")
	}
	var before, after map[string]any
	json.Unmarshal(body, &before)
	json.Unmarshal(out, &after)
	projects := after["result"].(map[string]any)["config"].(map[string]any)["projects"].(map[string]any)
	exact, ok := projects[cwd].(map[string]any)
	if !ok || len(exact) != 1 || exact["trust_level"] != "trusted" {
		t.Fatal(projects)
	}
	delete(projects, cwd)
	b, _ := json.Marshal(before)
	c, _ := json.Marshal(after)
	if !bytes.Equal(b, c) {
		t.Fatal("unrelated config, permissions or envelope changed")
	}
	if !bytes.Equal(a.response(body), body) {
		t.Fatal("replayed response was rewritten")
	}
}

func TestWTR002RefusalsLeaveOriginalFrameBytesUnchanged(t *testing.T) {
	for _, kind := range []string{"no-init", "unmatched", "duplicate-id", "expired", "error", "missing-layers", "project-layer", "disabled-layer", "unknown-layer", "no-root-trust", "explicit-cwd", "untrusted-parent", "wrong-root", "changed-proof", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			a, req, body, cwd := trustConfigFixture(t)
			var reply map[string]any
			json.Unmarshal(body, &reply)
			r := reply["result"].(map[string]any)
			c := r["config"].(map[string]any)
			projects := c["projects"].(map[string]any)
			switch kind {
			case "no-init":
				a.homeVerified = func() bool { return false }
			case "unmatched":
				reply["id"] = 99
			case "error":
				reply["error"] = map[string]any{"code": -32603}
			case "missing-layers":
				delete(r, "layers")
			case "project-layer":
				r["layers"] = []any{map[string]any{"name": map[string]any{"type": "project"}}}
			case "disabled-layer":
				r["layers"] = []any{map[string]any{"name": map[string]any{"type": "user"}, "disabledReason": "untrusted"}}
			case "unknown-layer":
				r["layers"] = []any{map[string]any{"name": map[string]any{"type": "future"}}}
			case "no-root-trust":
				c["projects"] = map[string]any{}
			case "explicit-cwd":
				projects[cwd] = map[string]any{"trust_level": "untrusted"}
			case "untrusted-parent":
				projects["/"] = map[string]any{"trust_level": "untrusted"}
			case "wrong-root":
				c["projects"] = map[string]any{cwd + "-other": map[string]any{"trust_level": "trusted"}}
			case "changed-proof":
				read := a.read
				n := 0
				a.read = func(ctx context.Context, cwd string) (worktreeTrustProof, bool) {
					p, ok := read(ctx, cwd)
					n++
					if n == 2 {
						p.Fingerprint += "changed"
					}
					return p, ok
				}
			}
			body, _ = json.Marshal(reply)
			if kind == "oversized" {
				body = append(body, bytes.Repeat([]byte(" "), trustConfigLimit)...)
			}
			a.request(req)
			if kind == "duplicate-id" {
				a.request(req)
			}
			if kind == "expired" {
				a.now = func() time.Time { return time.Now().Add(time.Minute) }
			}
			if got := a.response(body); !bytes.Equal(got, body) {
				t.Fatal("refused frame changed", kind, string(got))
			}
		})
	}
}

func TestWTR002CanceledAndLateProofCannotAuthorizeOrSpawnMoreReads(t *testing.T) {
	a, req, body, _ := trustConfigFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.ctx = ctx
	entered, release, settled := make(chan struct{}), make(chan struct{}), make(chan struct{})
	reader := a.read
	a.read = func(ctx context.Context, cwd string) (worktreeTrustProof, bool) {
		close(entered)
		<-release
		defer close(settled)
		return reader(ctx, cwd)
	}
	a.request(req)
	done := make(chan []byte, 1)
	go func() { done <- a.response(body) }()
	<-entered
	cancel()
	select {
	case got := <-done:
		if !bytes.Equal(got, body) {
			t.Fatal("late proof authorized")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not release response")
	}
	close(release)
	<-settled
	if !bytes.Equal(a.response(body), body) {
		t.Fatal("late completion resurrected request")
	}
}

func TestWTR002UnknownOrReusedRequestIDsNeverAccumulateUnboundedState(t *testing.T) {
	a, _, body, _ := trustConfigFixture(t)
	for i := 0; i < 200; i++ {
		req := []byte(`{"id":` + strings.Repeat("9", i+1) + `,"method":"config/read","params":{"cwd":"/synthetic","includeLayers":true}}`)
		a.request(req)
	}
	if len(a.pending) > 8 || len(a.seen) > 64 {
		t.Fatal(len(a.pending), len(a.seen))
	}
	if !bytes.Equal(a.response([]byte(`{"id":null,"result":{}}`)), []byte(`{"id":null,"result":{}}`)) {
		t.Fatal("notification changed")
	}
	if !bytes.Equal(a.response(append([]byte("malformed"), body...)), append([]byte("malformed"), body...)) {
		t.Fatal("malformed frame changed")
	}
}
