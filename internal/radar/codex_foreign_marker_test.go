package radar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestForeignActiveRequiresCurrentIdentityEndedTurnAndReadyFrame(t *testing.T) {
	const pane, loc = "%17", "synthetic:0.0"
	ready := "› Ask Codex to do anything\n  model · /work"
	for _, tc := range []struct {
		name, marker, agent, cwd, home, frontend, frame, kind       string
		waiting, future, missing, legacy, wrongPane, wrongLoc, want bool
	}{
		{name: "foreign ended", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete", want: true},
		{name: "foreign interrupted", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "turn_aborted", want: true},
		{name: "own active", marker: "own-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete"},
		{name: "legacy unnamed", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete"},
		{name: "legacy saved row only", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete", legacy: true},
		{name: "unknown binding", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete", missing: true},
		{name: "other home", marker: "other-thread", agent: "codex", cwd: "/work", home: "other-home", frame: ready, kind: "task_complete"},
		{name: "other pane", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete", wrongPane: true},
		{name: "other location", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete", wrongLoc: true},
		{name: "other cwd", marker: "other-thread", agent: "codex", cwd: "/different", frame: ready, kind: "task_complete"},
		{name: "other agent", marker: "other-thread", agent: "claude", cwd: "/work", frame: ready, kind: "task_complete"},
		{name: "expired frontend", marker: "other-thread", agent: "codex", cwd: "/work", frontend: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", frame: ready, kind: "task_complete"},
		{name: "new task", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_started"},
		{name: "unknown boundary", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready},
		{name: "future end", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete", future: true},
		{name: "working", marker: "other-thread", agent: "codex", cwd: "/work", frame: "• Working (2s • esc to interrupt)\n" + ready, kind: "task_complete"},
		{name: "tool", marker: "other-thread", agent: "codex", cwd: "/work", frame: "• Running tool\n" + ready, kind: "task_complete"},
		{name: "queued input", marker: "other-thread", agent: "codex", cwd: "/work", frame: "Messages to be submitted after next tool call\n" + ready, kind: "task_complete"},
		{name: "approval", marker: "other-thread", agent: "codex", cwd: "/work", frame: ready, kind: "task_complete", waiting: true},
		{name: "no ready composer", marker: "other-thread", agent: "codex", cwd: "/work", frame: "unknown terminal", kind: "task_complete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", "")
			previous := foreignCodexFrontend
			foreignCodexFrontend = func(string) (resume.CodexFrontend, bool) {
				if tc.missing || tc.legacy || tc.frontend != "" || tc.agent != "codex" {
					return resume.CodexFrontend{}, false
				}
				home := tc.home
				if home == "" {
					home = transcript.CodexHomeID()
				}
				targetPane, targetLoc := pane, loc
				if tc.wrongPane {
					targetPane = "%27"
				}
				if tc.wrongLoc {
					targetLoc = "other:0.0"
				}
				return resume.CodexFrontend{HomeID: home, SessionID: "own-thread", CodexBindingTarget: resume.CodexBindingTarget{Pane: targetPane, Loc: targetLoc, Cwd: tc.cwd}}, true
			}
			defer func() { foreignCodexFrontend = previous }()
			if !tc.missing {
				if err := resume.Save(loc, resume.Record{Agent: tc.agent, SessionID: "own-thread", Cwd: tc.cwd, HomeID: tc.home, Frontend: tc.frontend}); err != nil {
					t.Fatal(err)
				}
			}
			if err := state.WriteMarker(state.ActivePath(pane), tc.marker); err != nil {
				t.Fatal(err)
			}
			if tc.waiting {
				if err := state.WriteMarker(state.WaitingPath(pane), "permission"); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Stat(state.ActivePath(pane))
			if err != nil {
				t.Fatal(err)
			}
			end := time.Now().Add(-time.Hour)
			if tc.future {
				end = time.Now().Add(time.Hour)
			}
			kind, endedAt := codexForeignActiveEndedTurn(pane, loc, "/work", tc.frame, func(sid string) (string, time.Time) {
				if sid != "own-thread" {
					t.Fatal("read another thread's rollout")
				}
				return tc.kind, end
			})
			got := kind != ""
			if got != tc.want {
				t.Fatalf("contradiction=%v want %v", got, tc.want)
			}
			if got && (kind != tc.kind || !endedAt.Equal(end)) {
				t.Fatal("lost current thread end proof")
			}
			after, err := os.Stat(state.ActivePath(pane))
			if err != nil || !before.ModTime().Equal(after.ModTime()) || state.ReadMarker(state.ActivePath(pane)) != tc.marker {
				t.Fatal("display changed marker ownership")
			}
		})
	}
}

func TestForeignActiveProductionReaderRejectsLegacyResumeWithoutFrontend(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")
	const pane, loc = "%17", "synthetic:0.0"
	if err := resume.Save(loc, resume.Record{Agent: "codex", SessionID: "own-thread", Cwd: "/work"}); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteMarker(state.ActivePath(pane), "other-thread"); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'synthetic:0.0\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	previous, bin := foreignCodexFrontend, tmux.Bin
	foreignCodexFrontend = resume.CodexFrontendForPane
	tmux.Bin = script
	defer func() { foreignCodexFrontend = previous; tmux.Bin = bin }()
	called := false
	kind, _ := codexForeignActiveEndedTurn(pane, loc, "/work", "› Ask Codex to do anything\n model · /work", func(string) (string, time.Time) { called = true; return "task_complete", time.Now().Add(-time.Hour) })
	if kind != "" || called {
		t.Fatal("a saved legacy locator was promoted to live ownership")
	}
	if state.ReadMarker(state.ActivePath(pane)) != "other-thread" {
		t.Fatal("reader changed marker")
	}
}
