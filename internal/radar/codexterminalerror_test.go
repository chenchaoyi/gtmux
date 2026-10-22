package radar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const capacityError = "Selected model is at capacity. Please try a different model."

func TestCodexTerminalFailureRequiresCurrentErrorCell(t *testing.T) {
	ready := "\n\n› Ask Codex to do anything\n  model · /work"
	for _, tc := range []struct {
		name, frame string
		failed      bool
	}{
		{"terminal capacity", "■ " + capacityError + ready, true},
		{"ANSI", "\x1b[31m■ " + capacityError + "\x1b[0m" + ready, true},
		{"wrapped", "■ Selected model is at capacity. Please try a\n different model." + ready, true},
		{"plain prose", capacityError + ready, false},
		{"quoted", "> ■ " + capacityError + ready, false},
		{"tool indentation", "  ■ " + capacityError + ready, false},
		{"later output", "■ " + capacityError + "\n\n• Recovered and continued." + ready, false},
		{"working", "■ " + capacityError + "\n\n• Working (2s • esc to interrupt)" + ready, false},
		{"retry above error", "• Working (2s • esc to interrupt)\n\n■ " + capacityError + ready, false},
		{"running tool", "• Running task\n\n■ " + capacityError + ready, false},
		{"queued", "Messages to be submitted after next tool call\n\n■ " + capacityError + ready, false},
		{"approval", "■ " + capacityError + "\n\n› 1. Yes, proceed (y)\n  2. No (esc)", false},
		{"no composer", "■ " + capacityError, false},
		{"unknown error", "■ An unknown message" + ready, false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := codexTerminalFailure(tc.frame)
			if (got != "") != tc.failed || got != "" && got != capacityError {
				t.Fatalf("failure = %q, want failed=%v", got, tc.failed)
			}
		})
	}
}

func TestGatherCodexCapacityErrorDoesNotCompleteTurn(t *testing.T) {
	for _, marked := range []bool{false, true} {
		t.Run(map[bool]string{false: "no hook marker", true: "stale active marker"}[marked], func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", "")
			const pane = "%7"
			if marked {
				if err := state.WriteMarker(state.ActivePath(pane), "session"); err != nil {
					t.Fatal(err)
				}
				old := time.Now().Add(-5 * time.Second)
				if err := os.Chtimes(state.ActivePath(pane), old, old); err != nil {
					t.Fatal(err)
				}
			}
			oldCapture := paneCapture
			frame := "■ " + capacityError + "\n\n› Ask Codex to do anything\n  model · /work"
			paneCapture = func(string) string { return frame }
			defer func() { paneCapture = oldCapture }()
			lines := []string{paneLine(pane, "work", "0", "0", "Codex", "codex", time.Now().Unix(), 900007, "/work")}
			withFixture(t, lines, func() {
				rows := GatherAgents()
				if len(rows) != 1 || rows[0].Status != "idle" || !rows[0].Errored || rows[0].EndReason != "error" || rows[0].ErrorText != capacityError {
					t.Fatalf("failed turn shown as successful: %+v", rows)
				}
				if state.Exists(state.FinishedPath(pane)) || state.ReadLastFinished() != "" {
					t.Fatal("screen fallback stamped a successful completion")
				}
				if state.Exists(state.ActivePath(pane)) != marked {
					t.Fatal("screen fallback mutated hook ownership")
				}
				frame = "• Working (2s • esc to interrupt)\n\n› Ask Codex to do anything"
				for _, row := range GatherAgents() {
					if row.Errored || row.EndReason == "error" || row.ErrorText != "" {
						t.Fatalf("a new working turn retained screen error: %+v", row)
					}
				}
			})
		})
	}
}

func TestFreshCodexStartOutranksPreviousErrorFrame(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const pane = "%7"
	if err := state.WriteMarker(state.ActivePath(pane), "new-turn"); err != nil {
		t.Fatal(err)
	}
	at := FileMtime(state.ActivePath(pane))
	frame := "■ " + capacityError + "\n\n› Ask Codex to do anything"
	if got := codexTerminalFailureForPane(pane, frame, at+1); got != "" {
		t.Fatalf("new start was overridden by previous frame: %q", got)
	}
	if got := codexTerminalFailureForPane(pane, frame, at-1); got != "" {
		t.Fatalf("future start was overridden: %q", got)
	}
	if got := codexTerminalFailureForPane(pane, frame, at+2); got != capacityError {
		t.Fatalf("settled terminal error was missed: %q", got)
	}
}

func TestCodexCapacityErrorOutranksRolloutCompletion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	const pane, sid = "%7", "capacity-session"
	if err := resume.Save("work:0.0", resume.Record{Agent: "codex", SessionID: sid, Cwd: "/work"}); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteMarker(state.ActivePath(pane), sid); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(state.ActivePath(pane), old, old); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "22")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"timestamp":"` + old.Add(time.Second).UTC().Format(time.RFC3339Nano) + `","type":"event_msg","payload":{"type":"task_complete"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-test-"+sid+".jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	oldCapture := paneCapture
	paneCapture = func(string) string { return "■ " + capacityError + "\n\n› Ask Codex to do anything" }
	defer func() { paneCapture = oldCapture }()
	lines := []string{paneLine(pane, "work", "0", "0", "Codex", "codex", time.Now().Unix(), 900007, "/work")}
	withFixture(t, lines, func() {
		rows := GatherAgents()
		if len(rows) != 1 || !rows[0].Errored || rows[0].EndReason != "error" {
			t.Fatalf("rollout completion hid terminal failure: %+v", rows)
		}
	})
	if !state.Exists(state.ActivePath(pane)) || state.Exists(state.FinishedPath(pane)) {
		t.Fatal("terminal fallback changed turn markers through completion reconciliation")
	}
}
