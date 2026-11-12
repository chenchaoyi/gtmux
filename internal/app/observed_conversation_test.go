package app

import (
	"context"
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSID005ObservedReaderKeepsBoundedWindowAndExactCursor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	path := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "11", "12")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"type":"session_meta","payload":{"id":"history","originator":"codex-tui","source":"cli","cwd":"/synthetic"}}` + "\n")
	for i := 0; i < 45; i++ {
		row, _ := json.Marshal(map[string]any{"timestamp": time.Unix(1700000000+int64(i), 0).UTC().Format(time.RFC3339), "type": "event_msg", "payload": map[string]string{"type": "user_message", "message": "synthetic"}})
		data = append(data, append(row, '\n')...)
		data = append(data, []byte(`{"type":"event_msg","payload":{"type":"agent_message","message":"synthetic answer"}}`+"\n")...)
	}
	if err := os.WriteFile(filepath.Join(path, "rollout-synthetic-history.jsonl"), data, 0600); err != nil {
		t.Fatal(err)
	}
	oldBin, oldView := tmux.Bin, observedConversationViews
	t.Cleanup(func() { tmux.Bin = oldBin; observedConversationViews = oldView })
	stub := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho fixture:0.0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	tmux.Bin = stub
	observedConversationViews = func(*transcript.CodexPathScope, map[string]string, int64) map[string]resume.ConversationObservation {
		return map[string]resume.ConversationObservation{"%1": {SessionID: "history", HomeID: transcript.CodexHomeID(), State: "last-known"}}
	}
	b, m, err := transcriptObservedForPane("%1", 0, 20)
	var turns []transcript.Turn
	if json.Unmarshal(b, &turns) != nil || err != nil || len(turns) != 20 || !m.AssociationReadOnly || m.Etag == "" {
		t.Fatalf("unbounded/unmarked history len=%d meta=%+v err=%v", len(turns), m, err)
	}
	if turns[0].HistoryBefore == "" {
		t.Fatal("missing old-history cursor")
	}
	_, page, err := m.History(context.Background(), turns[0].HistoryBefore)
	if err != nil || !page.AssociationReadOnly {
		t.Fatal("page lost read-only state", err)
	}
	b, m, err = transcriptObservedForPane("%1", 0, 0)
	if err != nil || string(b) != "[]" || m.History == nil || !m.AssociationReadOnly {
		t.Fatal("history-only parsed current window", string(b), err)
	}
}
