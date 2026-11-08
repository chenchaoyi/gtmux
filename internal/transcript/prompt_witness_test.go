package transcript

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func witnessRecord(text string, at int64) string {
	b, _ := json.Marshal(map[string]any{"type": "event_msg", "timestamp": time.Unix(at, 0).UTC().Format(time.RFC3339), "payload": map[string]string{"type": "user_message", "message": text}})
	return string(b) + "\n"
}
func witnessRollout(t *testing.T, root, sid, instance, body string) string {
	t.Helper()
	dir := filepath.Join(root, "sessions", "2026", "11", "09")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "rollout-2026-11-09T00-00-"+instance+"-"+sid+"_"+instance+".jsonl")
	if err := os.WriteFile(p, []byte(`{"type":"session_meta","payload":{"id":"`+sid+`","originator":"codex-tui"}}`+"\n"+body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestHRS001FirstPromptWitnesses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	sid := "witness-thread"
	text := "synthetic exact reply"
	p := witnessRollout(t, root, sid, "01", witnessRecord(text, 90)+witnessRecord("different", 100)+witnessRecord(text, 110)+`{"type":"event_msg","payload":{"type":"agent_message","message":"ack"}}`+"\n"+witnessRecord(text, 120))
	first := FirstPromptRecords(context.Background(), "codex", sid, map[string]int64{text: 100})[text]
	if first == "" {
		t.Fatal("no first witness")
	}
	ts, err := Load("codex", sid, 20)
	if err != nil || len(ts) != 4 || first != ts[2].RecordID || first == ts[3].RecordID {
		t.Fatal("not earliest eligible record", ts, err)
	}
	// Caller mutation must never corrupt the cache.
	copy := FirstPromptRecords(context.Background(), "codex", sid, map[string]int64{text: 100})
	copy[text] = "forged"
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(witnessRecord(text, 130))
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got := FirstPromptRecords(context.Background(), "codex", sid, map[string]int64{text: 100})[text]; got != first {
		t.Fatal("append/caller changed first", got, first)
	}
	// An atomic replacement can remove the old first source record.
	replacement := p + ".replacement"
	body := `{"type":"session_meta","payload":{"id":"` + sid + `","originator":"codex-tui"}}` + "\n" + witnessRecord("different", 100) + witnessRecord(text, 140)
	if err := os.WriteFile(replacement, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, p); err != nil {
		t.Fatal(err)
	}
	got := FirstPromptRecords(context.Background(), "codex", sid, map[string]int64{text: 100})[text]
	if got == "" || got == first {
		t.Fatal("replacement kept old witness", got, first)
	}
	// A newly discovered continuation is included; incomplete final lines are not.
	more := "later reply"
	q := witnessRollout(t, root, sid, "02", witnessRecord(more, 150))
	out := FirstPromptRecords(context.Background(), "codex", sid, map[string]int64{text: 100, more: 100})
	if out[more] == "" || out[text] != got {
		t.Fatal("continuation missing", out)
	}
	if err := os.WriteFile(q, []byte(`{"type":"session_meta","payload":{"id":"`+sid+`","originator":"codex-tui"}}`+"\n"+witnessRecord("other", 150)+`{"type":"event_msg","payload":{"type":"user_message","message":"later reply"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if out := FirstPromptRecords(context.Background(), "codex", sid, map[string]int64{more: 100}); out[more] != "" {
		t.Fatal("partial/stale record attributed", out)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out := FirstPromptRecords(ctx, "codex", sid, map[string]int64{text: 100}); len(out) != 0 {
		t.Fatal("cancelled source read succeeded", out)
	}
}
