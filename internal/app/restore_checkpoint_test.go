package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestRestorePlanDoesNotReplaceUnsavedThreadWithLaunchID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	writeRecoveryFixture(t, "previous", home)
	r := resume.Record{Agent: "codex", SessionID: "previous", Cwd: home, HomeID: transcript.CodexHomeID()}
	if err := resume.Save("hq:0.0", r); err != nil {
		t.Fatal(err)
	}
	r.SessionID = "new-empty"
	if err := resume.Save("hq:0.0", r); err != nil {
		t.Fatal(err)
	}
	save := filepath.Join(t.TempDir(), "save.txt")
	if err := os.WriteFile(save, []byte(savePaneCmd("hq", "0", "0", home, "gtmux", ":gtmux codex -- resume previous")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := buildRestorePlanFrom(save)
	if len(plan.Sessions) != 1 || len(plan.Sessions[0].Agents) != 1 {
		t.Fatal(plan)
	}
	a := plan.Sessions[0].Agents[0]
	if a.SessionID != "new-empty" || a.Alive || a.RecoveryState != transcript.RecoveryMissing || a.CheckpointID != "previous" {
		t.Fatal(a)
	}
	writeRecoveryFixture(t, "new-empty", home)
	a = buildRestorePlanFrom(save).Sessions[0].Agents[0]
	if !a.Alive || a.SessionID != "new-empty" {
		t.Fatal(a)
	}
}
