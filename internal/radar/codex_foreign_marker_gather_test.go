package radar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestGatherDoesNotReviveAnotherSessionsOldMarkerFromWindowActivity(t *testing.T) {
	for _, tc := range []struct {
		name, marker, kind, frame, want string
		waiting, missing, legacy        bool
	}{
		{name: "foreign completed", marker: "other-thread", kind: "task_complete", want: "idle"},
		{name: "foreign interrupted", marker: "other-thread", kind: "turn_aborted", want: "idle"},
		{name: "own ongoing marker", marker: "own-thread", kind: "task_complete", want: "working"},
		{name: "legacy unnamed marker", kind: "task_complete", want: "working"},
		{name: "legacy saved row only", marker: "other-thread", kind: "task_complete", want: "working", legacy: true},
		{name: "unknown binding", marker: "other-thread", kind: "task_complete", want: "working", missing: true},
		{name: "genuine next turn", marker: "other-thread", kind: "task_started", frame: "• Working (3s • esc to interrupt)\n", want: "working"},
		{name: "real approval", marker: "other-thread", kind: "task_complete", want: "waiting", waiting: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CODEX_HOME", "")
			const pane = "%17"
			previousFrontend := foreignCodexFrontend
			foreignCodexFrontend = func(string) (resume.CodexFrontend, bool) {
				if tc.missing || tc.legacy {
					return resume.CodexFrontend{}, false
				}
				return resume.CodexFrontend{HomeID: transcript.CodexHomeID(), SessionID: "own-thread", CodexBindingTarget: resume.CodexBindingTarget{Pane: pane, Loc: "synthetic:0.0", Cwd: "/work"}}, true
			}
			defer func() { foreignCodexFrontend = previousFrontend }()
			if !tc.missing {
				if err := resume.Save("synthetic:0.0", resume.Record{Agent: "codex", SessionID: "own-thread", Cwd: "/work"}); err != nil {
					t.Fatal(err)
				}
			}
			dir := filepath.Join(home, ".codex", "sessions", "2026", "11", "05")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			at := time.Now().Add(-3 * time.Hour)
			body := `{"timestamp":"` + at.UTC().Format(time.RFC3339Nano) + `","type":"event_msg","payload":{"type":"` + tc.kind + `"}}` + "\n"
			if err := os.WriteFile(filepath.Join(dir, "rollout-test-own-thread.jsonl"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := state.WriteMarker(state.ActivePath(pane), tc.marker); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-2 * time.Hour)
			if err := os.Chtimes(state.ActivePath(pane), old, old); err != nil {
				t.Fatal(err)
			}
			if tc.waiting {
				if err := state.WriteMarker(state.WaitingPath(pane), "permission"); err != nil {
					t.Fatal(err)
				}
			}
			if err := state.WriteMarker(state.LastFinishedPath(), pane); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(state.ActivePath(pane))
			if err != nil {
				t.Fatal(err)
			}
			capture := paneCapture
			paneCapture = func(string) string { return tc.frame + "› Ask Codex to do anything\n model · /work" }
			defer func() { paneCapture = capture }()
			lines := []string{paneLine(pane, "synthetic", "0", "0", "Codex", "codex", time.Now().Unix(), 900017, "/work")}
			withFixture(t, lines, func() {
				rows := GatherAgents()
				if len(rows) != 1 || rows[0].Status != tc.want {
					t.Fatalf("rows=%+v want %s", rows, tc.want)
				}
				if tc.want == "idle" {
					if rows[0].Since != at.Unix() || rows[0].Latest || rows[0].Confirmed || rows[0].CompletionID != "" {
						t.Fatal("display correction minted/re-dated completion", rows[0])
					}
					wantReason := ""
					if tc.kind == "turn_aborted" {
						wantReason = "interrupted"
					}
					if rows[0].EndReason != wantReason {
						t.Fatal("lost actual current outcome", rows[0].EndReason)
					}
				}
			})
			after, err := os.Stat(state.ActivePath(pane))
			if err != nil || !after.ModTime().Equal(before.ModTime()) || state.ReadMarker(state.ActivePath(pane)) != tc.marker {
				t.Fatal("gather changed the foreign or legacy marker")
			}
			if state.Exists(state.FinishedPath(pane)) || state.ReadLastFinished() != pane {
				t.Fatal("status correction created a new completion")
			}
			if _, ok := state.ReadCompletion(pane); ok {
				t.Fatal("gather created lifecycle completion proof")
			}
			if tc.waiting && !state.Exists(state.WaitingPath(pane)) {
				t.Fatal("status correction removed a real approval")
			}
		})
	}
}
