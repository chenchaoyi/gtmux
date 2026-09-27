package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
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
