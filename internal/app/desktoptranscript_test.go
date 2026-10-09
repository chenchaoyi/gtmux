package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func desktopRollout(t *testing.T, root, id, instance, origin string, records ...string) string {
	t.Helper()
	dir := filepath.Join(root, "sessions", "2026", "10", "09")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-10-09T00-00-"+instance+"-"+id+"_"+instance+".jsonl")
	meta := `{"type":"session_meta","payload":{"id":"` + id + `","originator":"` + origin + `"}}`
	if err := os.WriteFile(path, []byte(meta+"\n"+strings.Join(records, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestDesktopTranscriptUsesIdentityAndDoesNotGrantFollow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	desktopRollout(t, root, "desk", "01", "codex_work_desktop",
		`{"type":"event_msg","payload":{"type":"user_message","message":"Please inspect"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"analysis","content":[{"type":"output_text","text":"PRIVATE"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"Inspecting now"}]}}`,
		`{"type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"ls\"}"}}`)
	b, m, err := transcriptForDesktop("desk")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Please inspect") || !strings.Contains(string(b), "Inspecting now") || strings.Contains(string(b), "PRIVATE") || !strings.Contains(string(b), `"title":"exec"`) || m.Etag == "" {
		t.Fatalf("read lost public content or leaked analysis: %s %+v", b, m)
	}
	if got := sessionpolicy.Get("desk"); got.HQ || got.Notify || got.Knowledge || got.Revision != 0 {
		t.Fatalf("read granted permissions: %+v", got)
	}
	if b, _, err := desktopTranscriptRead("desk", m.Etag); err != nil || b != nil {
		t.Fatalf("unchanged CLI must not reparse payload: %s %v", b, err)
	}
	desktopRollout(t, root, "desk", "02", "Codex Desktop",
		`{"type":"event_msg","payload":{"type":"user_message","message":"Continue"}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"Intermediate update"}}`)
	newer, next, err := transcriptForDesktop("desk")
	if err != nil || next.Etag == m.Etag || !strings.Contains(string(newer), "Intermediate update") {
		t.Fatalf("continuation did not update before completion: %s %+v %v", newer, next, err)
	}
	var turns []transcript.Turn
	if err := json.Unmarshal(newer, &turns); err != nil || len(turns) < 2 {
		t.Fatalf("history lost: %s %v", newer, err)
	}
	desktopRollout(t, root, "term", "01", "codex-tui")
	for _, id := range []string{"term", "unknown", "../desk", "desk*", ""} {
		if _, _, err := transcriptForDesktop(id); err != sessionpolicy.ErrUnverified {
			t.Fatalf("unverified %q accepted: %v", id, err)
		}
	}
	if cmdTranscript([]string{"desk", "--json"}) != 0 || cmdTranscript([]string{"desk", "--etag", "bad"}) != 2 || cmdTranscript([]string{"unknown", "--json"}) != 1 {
		t.Fatal("CLI refusal/result contract")
	}
}
func TestDesktopTranscriptEmptyAndBudgetAreExplicit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	desktopRollout(t, root, "empty", "01", "Codex Desktop")
	b, m, e := transcriptForDesktop("empty")
	if e != nil || string(b) != "[]" || m.Etag == "" {
		t.Fatalf("valid empty session must have revision: %s %+v %v", b, m, e)
	}
	var rows []string
	for i := 0; i < 40; i++ {
		v, _ := json.Marshal(map[string]any{"type": "event_msg", "payload": map[string]string{"type": "user_message", "message": fmt.Sprintf("%d %s", i, strings.Repeat("x", 20000))}})
		rows = append(rows, string(v))
	}
	desktopRollout(t, root, "large", "01", "Codex Desktop", rows...)
	b, m, e = transcriptForDesktop("large")
	if e != nil || len(b) > transcriptByteBudget || m.Dropped == 0 {
		t.Fatalf("unbounded/unmarked payload: bytes=%d meta=%+v err=%v", len(b), m, e)
	}
}
