package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHSP004StartupContextRecordsKeepFirstInputUnambiguous(t *testing.T) {
	const agents = "# AGENTS.md instructions\nsynthetic repository context"
	const env = "<environment_context>\n<cwd>/synthetic</cwd>\n</environment_context>"
	message, check := startupContextWitnessFixture(t, "/synthetic")
	for _, tc := range []struct {
		name, context string
		wantProof     bool
	}{
		{"two known blocks and a known non-user message", message("user", "", agents, env) + message("developer", "", "synthetic developer context"), true},
		{"known context mixed with human input", message("user", "", agents, "different synthetic input"), false},
		{"context attached to another turn", message("user", "foreign-turn", agents, env), false},
		{"message with no role", message("", "", "synthetic unclassified input"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check(t, tc.context, tc.wantProof)
		})
	}
}

func startupContextWitnessFixture(t *testing.T, cwd string) (
	message func(role, turnID string, texts ...string) string,
	check func(t *testing.T, context string, wantProof bool),
) {
	t.Helper()
	const (
		session = "01a20000-0000-7000-8000-000000000031"
		turn    = "01a20000-0000-7000-8000-000000000032"
		prompt  = "» gtmux·startup │ synthetic briefing · #abc123"
	)
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := filepath.Join(CodexHome(), "sessions", "2026", "11", "22")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-11-22T02-00-00-"+session+".jsonl")
	since := time.Date(2026, 11, 22, 2, 0, 0, 0, time.UTC)
	started, delivered := since.Add(time.Second), since.Add(2*time.Second)
	row := func(at time.Time, kind string, payload map[string]any) string {
		t.Helper()
		body, err := json.Marshal(map[string]any{"timestamp": at, "type": kind, "payload": payload})
		if err != nil {
			t.Fatal(err)
		}
		return string(body) + "\n"
	}
	message = func(role, turnID string, texts ...string) string {
		content := make([]map[string]string, 0, len(texts))
		for _, text := range texts {
			content = append(content, map[string]string{"type": "input_text", "text": text})
		}
		payload := map[string]any{"type": "message", "content": content, "turn_id": turnID}
		if role != "" {
			payload["role"] = role
		}
		return row(delivered, "response_item", payload)
	}
	header := row(since, "session_meta", map[string]any{"id": session, "cwd": cwd})
	start := row(started, "event_msg", map[string]any{"type": "task_started", "turn_id": turn})
	input := message("user", turn, prompt)
	padding := row(started, "world_state", map[string]any{"context": strings.Repeat("x", 240<<10)})
	growth := row(delivered, "world_state", map[string]any{"context": strings.Repeat("x", 24<<10)})
	check = func(t *testing.T, context string, wantProof bool) {
		t.Helper()
		if err := os.WriteFile(path, []byte(header+start+context+padding+input+growth), 0600); err != nil {
			t.Fatal(err)
		}
		if got := CodexWakeWitness(session, turn, prompt, since); got != (WakeWitness{}) {
			t.Fatal("fixture did not displace the start out of the ordinary tail")
		}
		want := WakeWitness{}
		if wantProof {
			want = WakeWitness{Started: started, Delivered: delivered, StartupPrefix: true}
		}
		if got := CodexStartupWakeWitness(session, turn, prompt, since); got != want {
			t.Fatalf("proof mismatch: got %+v, want %+v", got, want)
		}
	}
	return message, check
}
