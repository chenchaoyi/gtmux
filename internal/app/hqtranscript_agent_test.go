package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestStitchEarlierKeepsEachHQAgentsIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	sid := "a29c685a-7a56-4c61-a735-ddf2b8fcd5f6"
	dir := filepath.Join(home, ".claude", "projects", "-hq")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	log := `{"type":"user","message":{"role":"user","content":"old prompt"}}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Claude reply"}]}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, sid+".jsonl"), []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	newID := "01a0e0b2-1cd5-73f0-8e87-8cd39115af14"
	const now = int64(1_790_477_882)
	events.AuditHQSessionAgents("codex", newID, "claude", sid, now)
	current := []transcript.Turn{{Prompt: "new prompt", Response: "Codex reply", Agent: "codex"}}
	got, oldest := stitchEarlier(current, newID, "", 0, 1, now+1)
	if oldest != sid || len(got) != 2 {
		t.Fatalf("oldest=%q turns=%+v", oldest, got)
	}
	if got[0].Agent != "claude" || got[0].Response != "Claude reply" {
		t.Errorf("old turn = %+v; want Claude", got[0])
	}
	if got[1].Agent != "codex" || got[1].Response != "Codex reply" || got[1].Break == nil {
		t.Errorf("new turn = %+v; want Codex and handoff seam", got[1])
	}
}

func TestTranscriptOnlyStitchesSupervisorHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	oldID := "a29c685a-7a56-4c61-a735-ddf2b8fcd5f6"
	newID := "b29c685a-7a56-4c61-a735-ddf2b8fcd5f6"
	dir := filepath.Join(home, ".claude", "projects", "-hq")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, prompt := range map[string]string{oldID: "old prompt", newID: "new prompt"} {
		log := `{"type":"user","message":{"role":"user","content":"` + prompt + `"}}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(log), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := resume.Save("test:0.0", resume.Record{Agent: "claude", SessionID: newID}); err != nil {
		t.Fatal(err)
	}
	events.AuditHQSession(newID, oldID, 1000)
	stub := filepath.Join(home, "tmux")
	script := `#!/bin/sh
case "$2" in
  display-message)
    case "$6" in
      '#{pane_id}') printf '%s\n' "$5" ;;
      *) printf 'test:0.0\n' ;;
    esac ;;
  list-panes) printf '%%1\t%s/.config/gtmux/hq\t/tmp\t/tmp\n' "$HOME" ;;
esac
`
	if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	oldBin := tmux.Bin
	tmux.Bin = stub
	t.Cleanup(func() { tmux.Bin = oldBin })
	for _, earlier := range []int{0, 1} {
		b, meta, err := transcriptForPane("%2", earlier)
		if err != nil || meta.EarlierAvailable || strings.Contains(string(b), "old prompt") || !strings.Contains(string(b), "new prompt") {
			t.Fatalf("worker reached HQ chain: %s %+v %v", b, meta, err)
		}
	}
	if _, meta, err := transcriptForPane("%1", 0); err != nil || !meta.EarlierAvailable {
		t.Fatalf("HQ lost earlier-history indicator: %+v %v", meta, err)
	}
	b, _, err := transcriptForPane("%1", 1)
	if err != nil || !strings.Contains(string(b), "old prompt") || !strings.Contains(string(b), "new prompt") {
		t.Fatalf("HQ lost stitched history: %s %v", b, err)
	}
}
