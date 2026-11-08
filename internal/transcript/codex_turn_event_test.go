package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexTurnEventIdentityAndFreshness(t *testing.T) {
	// CET-001: same scope must observe newly appended facts.
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	dir := filepath.Join(root, "sessions", "2026", "11", "09")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-a-thread.jsonl")
	line := func(kind, turn string, at time.Time) []byte {
		b, err := json.Marshal(map[string]any{"timestamp": at.Format(time.RFC3339Nano), "type": "event_msg", "payload": map[string]string{"type": kind, "turn_id": turn}})
		if err != nil {
			t.Fatal(err)
		}
		return append(b, '\n')
	}
	at := time.Date(2026, 11, 9, 0, 0, 0, 123, time.UTC)
	if err := os.WriteFile(path, line("task_complete", "one", at), 0600); err != nil {
		t.Fatal(err)
	}
	scope := NewCodexPathScope()
	check := func(kind, turn string, at time.Time) {
		t.Helper()
		e := scope.CodexLastTurnEvent("thread")
		if e.Kind != kind || e.TurnID != turn || !e.At.Equal(at) || e.Ambiguous {
			t.Fatalf("event %+v", e)
		}
		k, a := scope.CodexLastTurnBoundary("thread")
		if k != kind || !a.Equal(at) {
			t.Fatal("legacy boundary changed", k, a)
		}
	}
	check("task_complete", "one", at)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(line("task_started", "two", at.Add(time.Second)))
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	check("task_started", "two", at.Add(time.Second))
	if err := os.WriteFile(path, line("turn_aborted", "", at.Add(2*time.Second)), 0600); err != nil {
		t.Fatal(err)
	}
	check("turn_aborted", "", at.Add(2*time.Second))
	// Equally dated but conflicting files must not become exact-turn proof.
	if err := os.WriteFile(filepath.Join(dir, "rollout-b-thread.jsonl"), line("task_started", "three", at.Add(2*time.Second)), 0600); err != nil {
		t.Fatal(err)
	}
	if e := CodexLastTurnEvent("thread"); !e.Ambiguous {
		t.Fatal("conflicting equal-time metadata accepted", e)
	}
}

func TestCodexTurnEventTailBudget(t *testing.T) {
	// CET-001: malformed records and oversized body stay bounded/unknown.
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	dir := filepath.Join(root, "sessions", "2026", "11", "09")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-a-thread.jsonl")
	terminal := `{"timestamp":"2026-11-09T00:00:00Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"one"}}` + "\n"
	if err := os.WriteFile(path, []byte(terminal+strings.Repeat("x", 300<<10)), 0600); err != nil {
		t.Fatal(err)
	}
	if e := CodexLastTurnEvent("thread"); e.Kind != "" {
		t.Fatal("read past tail budget", e)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 300<<10)+"\nnot-json\n"+terminal+`{"type":"event_msg","payload":{"type":"task_started"},"timestamp":"bad"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if e := CodexLastTurnEvent("thread"); e.Kind != "task_complete" || e.TurnID != "one" {
		t.Fatal("lost valid bounded metadata", e)
	}
}
