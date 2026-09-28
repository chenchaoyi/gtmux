package radar

import (
	"os"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestCodexTurnCompletedRequiresCurrentPaneAndTurn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const pane, loc, sid = "%19", "dev:0.0", "dev-session"
	if err := resume.Save(loc, resume.Record{Agent: "codex", SessionID: sid, Cwd: "/work/dev"}); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteMarker(state.ActivePath(pane), sid); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteMarker(state.WaitingPath(pane), "permission"); err != nil {
		t.Fatal(err)
	}
	marked := time.Now().Add(-2 * time.Minute)
	for _, path := range []string{state.ActivePath(pane), state.WaitingPath(pane)} {
		if err := os.Chtimes(path, marked, marked); err != nil {
			t.Fatal(err)
		}
	}
	boundary := func(string) (string, time.Time) { return "task_complete", marked.Add(time.Minute) }
	if at, ok := codexTurnCompleted(pane, loc, "/work/dev", boundary); !ok || !at.Equal(marked.Add(time.Minute)) {
		t.Fatalf("completed turn = %s,%v", at, ok)
	}
	if err := state.WriteMarker(state.ActivePath(pane), ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(state.ActivePath(pane), marked, marked); err != nil {
		t.Fatal(err)
	}
	if _, ok := codexTurnCompleted(pane, loc, "/work/dev", boundary); !ok {
		t.Fatal("plain Codex turn marker did not reconcile from its bound rollout")
	}
	for name, got := range map[string]bool{
		"other cwd":      completedForTest(pane, loc, "/work/other", boundary),
		"other pane":     completedForTest("%16", loc, "/work/dev", boundary),
		"new turn":       completedForTest(pane, loc, "/work/dev", func(string) (string, time.Time) { return "task_started", time.Now() }),
		"old completion": completedForTest(pane, loc, "/work/dev", func(string) (string, time.Time) { return "task_complete", marked.Add(-time.Second) }),
	} {
		if got {
			t.Errorf("%s claimed completed turn", name)
		}
	}
	if err := state.WriteMarker(state.ActivePath(pane), "other-session"); err != nil {
		t.Fatal(err)
	}
	if _, ok := codexTurnCompleted(pane, loc, "/work/dev", boundary); ok {
		t.Fatal("another session's turn was claimed")
	}
}

func completedForTest(pane, loc, cwd string, boundary func(string) (string, time.Time)) bool {
	_, ok := codexTurnCompleted(pane, loc, cwd, boundary)
	return ok
}
