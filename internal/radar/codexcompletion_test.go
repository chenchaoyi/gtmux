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

func TestCodexCompletedTurnContradictsIdleRepaint(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const pane, loc, sid = "%17", "website:0.0", "codex-session"
	if err := resume.Save(loc, resume.Record{Agent: "codex", SessionID: sid, Cwd: "/work/site"}); err != nil {
		t.Fatal(err)
	}
	complete := func(id string) (string, time.Time) {
		if id != sid {
			t.Fatalf("boundary read for wrong session %q", id)
		}
		return "task_complete", time.Now().Add(-time.Hour)
	}
	if !codexCompletedTurnContradictsFrame(pane, loc, "/work/site", complete) {
		t.Fatal("completed Codex turn should outrank a repaint")
	}
	for name, boundary := range map[string]func(string) (string, time.Time){
		"new turn": func(string) (string, time.Time) { return "task_started", time.Now() },
		"unknown":  func(string) (string, time.Time) { return "", time.Time{} },
	} {
		if codexCompletedTurnContradictsFrame(pane, loc, "/work/site", boundary) {
			t.Errorf("%s was suppressed", name)
		}
	}
	if codexCompletedTurnContradictsFrame(pane, loc, "/work/other", complete) {
		t.Error("other directory was suppressed")
	}
	if err := state.WriteMarker(state.ActivePath(pane), sid); err != nil {
		t.Fatal(err)
	}
	if codexCompletedTurnContradictsFrame(pane, loc, "/work/site", complete) {
		t.Error("active turn marker was suppressed")
	}
	state.Remove(state.ActivePath(pane))
	if err := state.WriteMarker(state.WaitingPath(pane), "permission"); err != nil {
		t.Fatal(err)
	}
	if codexCompletedTurnContradictsFrame(pane, loc, "/work/site", complete) {
		t.Error("waiting marker was suppressed")
	}
}

func TestCodexIdleComposerContradictsOnlyUnownedScreenRepaints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const pane = "%15"
	idle := "⚠ weekly limit: only 1% left · /status\n› Ask Codex to do anything\n  GPT-6-Astra default"
	if !codexIdleComposerContradictsFrame(pane, idle) {
		t.Fatal("settled warning screen should remain idle")
	}
	for name, frame := range map[string]string{
		"active turn":  "• Working (3s • esc to interrupt)\n" + idle,
		"running tool": "• Running gtmux events\n" + idle,
		"queued input": "Messages to be submitted after next tool call\n" + idle,
		"approval":     "› 1. Yes, proceed (y)\n  2. No (esc)",
	} {
		if codexIdleComposerContradictsFrame(pane, frame) {
			t.Errorf("%s was suppressed as idle", name)
		}
	}
	if err := state.WriteMarker(state.ActivePath(pane), "codex-turn"); err != nil {
		t.Fatal(err)
	}
	if codexIdleComposerContradictsFrame(pane, idle) {
		t.Error("active hook marker was suppressed")
	}
}
