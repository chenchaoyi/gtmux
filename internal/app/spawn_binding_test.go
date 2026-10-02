package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestSpawnBindingRestoresTranscriptEndpoint(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	cwd := t.TempDir()
	t.Setenv("FAKE_PANE_CWD", cwd)
	stub := filepath.Join(t.TempDir(), "tmux")
	script := `#!/bin/sh
case "$*" in
  *pane_pid*) printf '%s\t%s\t%s\t%s\t%s\n' '%27' 'worker:0.0' "$FAKE_PANE_CWD" 58325 codex;;
  *session_name*) printf '%s\n' 'worker:0.0';;
  *pane_id*) printf '%s\n' '%27';;
esac
`
	if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	tmux.Bin = stub
	t.Cleanup(func() { tmux.Bin = old })
	const goal = "Repair test issue"
	if got := spawnBindingPayload("%27", "claude", goal); got != goal {
		t.Fatal("non-Codex dispatch changed")
	}
	wire := spawnBindingPayload("%27", "codex", goal)
	if transcript.SessionBindingToken(wire) == "" {
		t.Fatal("fresh Codex spawn did not prepare a witness")
	}
	if b, _, err := transcriptForPane("%27", 0); err != nil || string(b) != "[]" {
		t.Fatal("unbound pane should not borrow another conversation", string(b), err)
	}
	dir := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "03")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]string{"id": "worker-session", "cwd": cwd, "originator": "codex-tui"}})
	msg, _ := json.Marshal(map[string]any{"timestamp": "2026-10-03T00:00:01Z", "type": "event_msg", "payload": map[string]string{"type": "user_message", "message": wire}})
	reply := `{"timestamp":"2026-10-03T00:00:02Z","type":"event_msg","payload":{"type":"agent_message","message":"I found the issue."}}` + "\n"
	data := append(append(meta, '\n'), append(msg, '\n')...)
	data = append(data, reply...)
	if err := os.WriteFile(filepath.Join(dir, "rollout-2026-10-03T00-00-00-worker-session.jsonl"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	intent, ok := resume.CodexBindingForPrompt(wire, time.Now().Unix())
	if !ok {
		t.Fatal("prepared witness missing")
	}
	live, ok := resume.LiveCodexBindingTarget("%27")
	if !ok {
		t.Fatal("live target missing")
	}
	if err := resume.CompleteCodexBinding(intent, live, "worker-session", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	b, metaOut, err := transcriptForPane("%27", 0)
	var turns []transcript.Turn
	if err != nil || json.Unmarshal(b, &turns) != nil || len(turns) != 1 || turns[0].Prompt != goal || turns[0].Response != "I found the issue." || metaOut.Etag == "" {
		t.Fatal("endpoint did not recover the worker's own history", string(b), metaOut, err)
	}
	if got := spawnBindingPayload("%27", "codex", goal); got != goal {
		t.Fatal("bound Codex dispatch unnecessarily added a witness")
	}
}
