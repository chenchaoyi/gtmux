package radar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func collectionFixture(t *testing.T) (map[string]string, time.Time) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	if err := os.MkdirAll(state.ConfigDir(), 0700); err != nil {
		t.Fatal(err)
	}
	// Passive fixtures never invoke a real agent's authenticated /usage command.
	if err := os.WriteFile(filepath.Join(state.ConfigDir(), "usage.json"), []byte(`{"limitsCommand":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	paths := map[string]string{}
	for _, id := range []string{"collect-one", "collect-two"} {
		day := filepath.Join(home, ".codex", "sessions", "2026", "10", "15")
		if err := os.MkdirAll(day, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(day, "rollout-2026-10-15T00-00-00-"+id+".jsonl")
		records := []any{
			map[string]any{"type": "session_meta", "payload": map[string]string{"id": id, "originator": "codex-tui"}},
			map[string]any{"type": "response_item", "timestamp": now.Add(-time.Second).Format(time.RFC3339Nano), "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]string{"type": "input_text", "text": "goal-" + id}}}},
			map[string]any{"type": "event_msg", "timestamp": now.Add(-time.Second).Format(time.RFC3339Nano), "payload": map[string]string{"type": "task_complete"}},
		}
		var body []byte
		for _, record := range records {
			line, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			body = append(body, append(line, '\n')...)
		}
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		if err := native.Save(native.Record{Agent: "codex", SessionID: id, HomeID: transcript.CodexHomeID(), State: "working", UpdatedAt: now.Add(-2 * time.Second).Unix()}); err != nil {
			t.Fatal(err)
		}
		if _, err := sessionpolicy.Save(id, sessionpolicy.Settings{HQ: true}); err != nil {
			t.Fatal(err)
		}
		paths[id] = path
	}
	return paths, now
}

func TestCompleteProducersShareDiscoveryThroughContentAndAccounting(t *testing.T) {
	collectionFixture(t)
	withFixture(t, nil, func() {
		for _, kind := range []string{"digest", "usage"} {
			t.Run(kind, func(t *testing.T) {
				c := NewCollection()
				before := transcript.CodexCandidateDiscoveryCount()
				if kind == "digest" {
					rows := c.Digest()
					if len(rows) != 2 {
						t.Fatalf("digest rows = %+v", rows)
					}
					for _, row := range rows {
						if row.Goal != "goal-"+row.SessionID {
							t.Fatalf("content did not reach digest: %+v", row)
						}
					}
				} else {
					rows := c.Usage().Sessions
					if len(rows) != 2 {
						t.Fatalf("usage rows = %+v", rows)
					}
				}
				if got := transcript.CodexCandidateDiscoveryCount() - before; got != 1 {
					t.Fatalf("complete %s enumerated %d times, want 1", kind, got)
				}
			})
		}
	})
}

func TestCollectionSharesPathsButResamplesFactsBetweenOrderedGathers(t *testing.T) {
	paths, _ := collectionFixture(t)
	withFixture(t, nil, func() {
		c := NewCollection()
		before := transcript.CodexCandidateDiscoveryCount()
		first := c.Agents()
		if len(first) != 2 || first[0].Status != "idle" {
			t.Fatalf("initial completed rows = %+v", first)
		}
		if got := c.Digest(); len(got) != 2 {
			t.Fatalf("initial content rows %+v", got)
		}
		file, err := os.OpenFile(paths["collect-one"], os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		line := fmt.Sprintf("{\"type\":\"event_msg\",\"timestamp\":%q,\"payload\":{\"type\":\"task_started\"}}\n", time.Now().Format(time.RFC3339Nano))
		line += fmt.Sprintf("{\"type\":\"response_item\",\"timestamp\":%q,\"payload\":{\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"new goal\"}]}}\n", time.Now().Format(time.RFC3339Nano))
		if _, err := file.WriteString(line); err != nil {
			t.Fatal(err)
		}
		file.Close()
		second := c.Agents()
		working := false
		for _, row := range second {
			if row.sessionID == "collect-one" && row.Status == "working" {
				working = true
			}
		}
		if !working {
			t.Fatalf("collection cached mutable status: %+v", second)
		}
		for _, row := range c.Digest() {
			if row.SessionID == "collect-one" && row.Goal != "new goal" {
				t.Fatalf("content remained stale: %+v", row)
			}
		}
		if got := transcript.CodexCandidateDiscoveryCount() - before; got != 1 {
			t.Fatalf("ordered gathers enumerated %d times", got)
		}
		next := NewCollection()
		_ = next.Digest()
		if got := transcript.CodexCandidateDiscoveryCount() - before; got != 2 {
			t.Fatalf("next operation did not rediscover: %d", got)
		}
	})
}

func TestCollectionDoesNotCacheFollowPermission(t *testing.T) {
	collectionFixture(t)
	withFixture(t, nil, func() {
		c := NewCollection()
		if got := len(c.Digest()); got != 2 {
			t.Fatalf("initial managed digest rows %d", got)
		}
		policy := sessionpolicy.Get("collect-one")
		policy.HQ = false
		if _, err := sessionpolicy.Save("collect-one", policy); err != nil {
			t.Fatal(err)
		}
		if got := len(c.Digest()); got != 1 {
			t.Fatalf("revoked conversation remains in managed digest: %d", got)
		}
		if c.codexPaths().DiscoveryCount() != 1 {
			t.Fatal("permission check invalidated the path scope")
		}
	})
}
