package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHSP001HSP002StartupWitnessUsesOneContinuousPrefix(t *testing.T) {
	const (
		session = "01a20000-0000-7000-8000-000000000001"
		turn    = "01a20000-0000-7000-8000-000000000002"
		foreign = "01a20000-0000-7000-8000-000000000003"
		prompt  = "» gtmux·startup │ synthetic startup briefing · #abc123"
		budget  = 256 << 10
	)
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := filepath.Join(CodexHome(), "sessions", "2026", "11", "22")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-11-22T02-00-00-"+session+".jsonl")
	write := func(t *testing.T, path, contents string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

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
	meta := func(id string) string {
		return row(since, "session_meta", map[string]any{"id": id, "base_instructions": strings.Repeat("m", 16<<10)})
	}
	boundary := func(at time.Time, event, id string) string {
		return row(at, "event_msg", map[string]any{"type": event, "turn_id": id})
	}
	user := func(texts ...string) string {
		blocks := make([]map[string]string, 0, len(texts))
		for _, text := range texts {
			blocks = append(blocks, map[string]string{"type": "input_text", "text": text})
		}
		return row(delivered, "response_item", map[string]any{"type": "message", "role": "user",
			"content": blocks})
	}
	padding := func(size int) string {
		base := row(started, "world_state", map[string]any{"context": ""})
		if size < len(base) {
			t.Fatal("synthetic padding does not fit")
		}
		return row(started, "world_state", map[string]any{"context": strings.Repeat("x", size-len(base))})
	}
	header := meta(session)
	start := boundary(started, "task_started", turn)
	const agents = "# AGENTS.md instructions\nsynthetic repository context"
	const environment = "<environment_context>\n<cwd>/synthetic</cwd>\n</environment_context>"
	harness := user(agents, environment)
	nonUser := row(started, "response_item", map[string]any{"type": "message", "role": "developer",
		"content": []map[string]string{{"type": "input_text", "text": "synthetic developer context"}}})
	input := user(prompt)
	// The original tail begins just before the end of metadata. An append then
	// moves it into the padding, with the start absent and the input still whole.
	before := header + start + harness + nonUser + padding(budget-64-len(start)-len(harness)-len(nonUser)-len(input)) + input
	after := before + padding(24<<10)
	if lo, startOffset, inputOffset := len(after)-budget, len(header), len(before)-len(input); !(startOffset < lo && lo < inputOffset) {
		t.Fatal("fixture did not displace only the boundary")
	}
	wantTail := WakeWitness{Started: started, Delivered: delivered}
	wantPrefix := WakeWitness{Started: started, Delivered: delivered, StartupPrefix: true}
	t.Run("same exact input before and after growth", func(t *testing.T) {
		write(t, path, before)
		if got := CodexStartupWakeWitness(session, turn, prompt, since); got != wantTail {
			t.Fatal("tail did not prove the complete turn before growth", got)
		}
		write(t, path, after)
		if got := CodexWakeWitness(session, turn, prompt, since); got != (WakeWitness{}) {
			t.Fatal("ordinary tail proved a missing start", got)
		}
		if got := CodexStartupWakeWitness(session, turn, prompt, since); got != wantPrefix {
			t.Fatal("bounded contiguous prefix did not prove the original timestamps", got)
		}
	})

	padded := padding(230 << 10)
	growth := padding(48 << 10)
	extraContent := row(delivered, "response_item", map[string]any{"type": "message", "role": "user",
		"content": []map[string]string{{"type": "input_text", "text": prompt}, {"type": "input_image"}}})
	foreignInput := row(delivered, "event_msg", map[string]any{"type": "user_message", "message": prompt, "turn_id": foreign})
	untyped := row(delivered, "response_item", map[string]any{"type": "message",
		"content": []map[string]string{{"type": "input_text", "text": "synthetic unclassified input"}}})
	foreignContext := row(delivered, "response_item", map[string]any{"type": "message", "role": "user", "turn_id": foreign,
		"content": []map[string]string{{"type": "input_text", "text": agents}, {"type": "input_text", "text": environment}}})
	for _, tc := range []struct{ name, data string }{
		{"foreign start", header + start + boundary(started, "task_started", foreign) + padded + input + growth},
		{"complete", header + start + boundary(started, "task_complete", turn) + padded + input + growth},
		{"abort", header + start + boundary(started, "turn_aborted", turn) + padded + input + growth},
		{"explicitly foreign input", header + start + padded + foreignInput + growth},
		{"different first input", header + start + user("another synthetic prompt") + padded + input + growth},
		{"known context mixed with human input", header + start + user(agents, "another synthetic prompt") + padded + input + growth},
		{"context attached to another turn", header + start + foreignContext + padded + input + growth},
		{"message with no role", header + start + untyped + padded + input + growth},
		{"incomplete first input", header + start + extraContent + padded + input + growth},
		{"unknown first input", header + start + row(delivered, "response_item", map[string]any{"type": "message", "role": "user"}) + padded + input + growth},
		{"wrong metadata", meta(foreign) + start + padded + input + growth},
		{"missing metadata", start + padded + input + growth},
		{"old start", header + boundary(since.Add(-time.Nanosecond), "task_started", turn) + padded + input + growth},
		{"malformed complete record", header + start + "{\n" + padded + input + growth},
		{"no start", header + padded + input + growth},
		{"input crosses budget", header + start + padding(budget-len(start)-len(input)+1) + input + growth},
		{"unterminated input with visible start", header + start + padded + strings.TrimSuffix(input, "\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write(t, path, tc.data)
			if got := CodexStartupWakeWitness(session, turn, prompt, since); got != (WakeWitness{}) {
				t.Fatal("unproved input was accepted", got)
			}
		})
	}

	t.Run("competing rollout", func(t *testing.T) {
		write(t, path, after)
		other := filepath.Join(dir, "rollout-2026-11-22T02-01-00-"+session+".jsonl")
		write(t, other, header)
		if got := CodexStartupWakeWitness(session, turn, prompt, since); got != (WakeWitness{}) {
			t.Fatal("ambiguous rollout authorized a prefix", got)
		}
	})
	t.Run("unavailable rollout", func(t *testing.T) {
		if got := CodexStartupWakeWitness(foreign, turn, prompt, since); got != (WakeWitness{}) {
			t.Fatal("an unavailable rollout produced a witness", got)
		}
	})
}
