package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestGatherCompletedOwnTurnSurvivesLateActiveMarker(t *testing.T) {
	// CET-002/CET-003: exact production GatherAgents path and actual rollout parser.
	for _, tc := range []struct {
		name, kind, turn, current, frame, want, home, cwd, marker string
		wait, missing, conflict, future                           bool
	}{
		{name: "completed same turn", kind: "task_complete", turn: "one", current: "one", want: "idle"},
		{name: "aborted same turn", kind: "turn_aborted", turn: "one", current: "one", want: "idle"},
		{name: "new prompt ahead of rollout", kind: "task_complete", turn: "one", current: "two", want: "working"},
		{name: "missing old rollout ID", kind: "task_complete", current: "one", want: "working"},
		{name: "missing current turn", kind: "task_complete", turn: "one", want: "working"},
		{name: "latest start", kind: "task_started", turn: "two", current: "two", want: "working"},
		{name: "busy", kind: "task_complete", turn: "one", current: "one", frame: "• Working (3s • esc to interrupt)\n", want: "working"},
		{name: "queued", kind: "task_complete", turn: "one", current: "one", frame: "Messages to be submitted after next tool call\n", want: "working"},
		{name: "approval", kind: "task_complete", turn: "one", current: "one", wait: true, want: "waiting"},
		{name: "no frontend", kind: "task_complete", turn: "one", current: "one", missing: true, want: "working"},
		{name: "wrong home", kind: "task_complete", turn: "one", current: "one", home: "other-home", want: "working"},
		{name: "wrong cwd", kind: "task_complete", turn: "one", current: "one", cwd: "/other", want: "working"},
		{name: "unnamed marker", kind: "task_complete", turn: "one", current: "one", marker: "unnamed", want: "working"},
		{name: "conflicting same time", kind: "task_complete", turn: "one", current: "one", conflict: true, want: "working"},
		{name: "future completion", kind: "task_complete", turn: "one", current: "one", future: true, want: "working"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CODEX_HOME", "")
			const pane, loc, sid = "%17", "synthetic:0.0", "own-thread"
			old := foreignCodexFrontend
			t.Cleanup(func() { foreignCodexFrontend = old })
			foreignCodexFrontend = func(string) (resume.CodexFrontend, bool) {
				h, c := transcript.CodexHomeID(), "/work"
				if tc.home != "" {
					h = tc.home
				}
				if tc.cwd != "" {
					c = tc.cwd
				}
				return resume.CodexFrontend{HomeID: h, SessionID: sid, CodexBindingTarget: resume.CodexBindingTarget{Pane: pane, Loc: loc, Cwd: c}}, !tc.missing
			}
			if err := resume.Save(loc, resume.Record{Agent: "codex", SessionID: sid, Cwd: "/work"}); err != nil {
				t.Fatal(err)
			}
			at := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
			if tc.future {
				at = time.Now().Add(time.Hour)
			}
			dir := filepath.Join(home, ".codex", "sessions", "2026", "11", "05")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			write := func(name, kind, turn string) {
				b, err := json.Marshal(map[string]any{"timestamp": at.UTC().Format(time.RFC3339Nano), "type": "event_msg", "payload": map[string]string{"type": kind, "turn_id": turn}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name+"-"+sid+".jsonl"), append(b, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("rollout-a", tc.kind, tc.turn)
			if tc.conflict {
				write("rollout-b", "task_started", "two")
			}
			marker := sid
			if tc.marker == "unnamed" {
				marker = ""
			}
			if err := state.WriteMarker(state.ActivePath(pane), marker); err != nil {
				t.Fatal(err)
			}
			late := time.Now().Add(-time.Hour + 6*time.Second)
			if err := os.Chtimes(state.ActivePath(pane), late, late); err != nil {
				t.Fatal(err)
			}
			if tc.current != "" {
				if err := state.WriteMarker(state.TurnIDPath(pane), tc.current); err != nil {
					t.Fatal(err)
				}
			}
			if tc.wait {
				if err := state.WriteMarker(state.WaitingPath(pane), "permission"); err != nil {
					t.Fatal(err)
				}
			}
			if err := state.WriteLastFinished("%other"); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(state.ActivePath(pane))
			if err != nil {
				t.Fatal(err)
			}
			capture := paneCapture
			t.Cleanup(func() { paneCapture = capture })
			paneCapture = func(string) string {
				// A real approval is not a ready composer with a separate wait marker.
				if tc.wait {
					return "Would you like to run this command?\n› 1. Yes, proceed\n  2. No, tell Codex what to do\nPress enter to confirm\n"
				}
				return tc.frame + "› Ask Codex to do anything\n model · /work"
			}
			withFixture(t, []string{paneLine(pane, "synthetic", "0", "0", "Codex", "codex", time.Now().Unix(), 900017, "/work")}, func() {
				for i := 0; i < 2; i++ {
					rows := GatherAgents()
					if len(rows) != 1 || rows[0].Status != tc.want {
						t.Fatalf("iteration %d want %s got %+v", i, tc.want, rows)
					}
					if tc.want == "idle" {
						r := rows[0]
						if r.Since != at.Unix() || r.Latest || r.Confirmed || r.CompletionID != "" {
							t.Fatal("minted/re-dated completion", r)
						}
						reason := ""
						if tc.kind == "turn_aborted" {
							reason = "interrupted"
						}
						if r.EndReason != reason {
							t.Fatal("lost actual outcome", r.EndReason)
						}
					}
				}
			})
			after, err := os.Stat(state.ActivePath(pane))
			if err != nil || !before.ModTime().Equal(after.ModTime()) || state.ReadMarker(state.ActivePath(pane)) != marker {
				t.Fatal("display correction mutated active marker")
			}
			if state.Exists(state.FinishedPath(pane)) || state.ReadLastFinished() != "%other" || state.ReadMarker(state.TurnIDPath(pane)) != tc.current {
				t.Fatal("display correction altered lifecycle")
			}
			if tc.wait && !state.Exists(state.WaitingPath(pane)) {
				t.Fatal("approval lost")
			}
		})
	}
}
