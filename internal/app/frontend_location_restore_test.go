package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestCFL001RestoreUsesCurrentSavedLayoutOnceAndKeepsOriginalHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	r := resume.Record{HomeID: transcript.CodexHomeID(), Agent: "codex", SessionID: "moved-synthetic-root", Cwd: "/some/dir", Frontend: strings.Repeat("a", 32), UpdatedAt: 200}
	if err := resume.Save("work:0.1", r); err != nil {
		t.Fatal(err)
	}
	r.UpdatedAt = 201
	if err := resume.Save("work:0.0", r); err != nil {
		t.Fatal(err)
	}
	writeRecoveryFixture(t, r.SessionID, r.Cwd)
	save := filepath.Join(t.TempDir(), "resurrect.txt")
	body := savePaneCmd("work", "0", "0", r.Cwd, "codex", ":codex resume moved-synthetic-root") + "\n" + saveShellPane("work", "0", "1", r.Cwd) + "\n"
	if err := os.WriteFile(save, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	plan := buildRestorePlanFrom(save)
	if plan.agentCount() != 1 || len(plan.Sessions) != 1 || len(plan.Sessions[0].Agents) != 1 {
		t.Fatal("one moved session was promised more than once", plan)
	}
	got := plan.Sessions[0].Agents[0]
	if got.Loc != "work:0.0" || got.SessionID != r.SessionID {
		t.Fatal("restore selected an obsolete layout address", got)
	}
	old, ok := resume.Load("work:0.1")
	if !ok || old.SessionID != r.SessionID || old.UpdatedAt != 200 {
		t.Fatal("read-only restore planning changed original historical record", old, ok)
	}
}
