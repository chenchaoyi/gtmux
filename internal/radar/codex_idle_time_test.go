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

func TestCodexIdleMaintenanceDoesNotPromoteOldCompletion(t *testing.T) {
	for _, cached := range []bool{false, true} {
		name := "after-session-end"
		if cached {
			name = "already-poisoned-cache"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CODEX_HOME", "")
			const pane, sid, cwd = "%2", "old-session", "/work/old"
			old := time.Now().Add(-18 * 24 * time.Hour).Truncate(time.Second)
			newer := old.Add(24 * time.Hour)
			maintenance := time.Now().Add(-time.Hour).Truncate(time.Second)
			if err := resume.Save("old:0.0", resume.Record{Agent: "codex", SessionID: sid, Cwd: cwd}); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(home, ".codex", "sessions", "2026", "09", "28")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			body := `{"timestamp":"` + old.Format(time.RFC3339) + `","type":"event_msg","payload":{"type":"task_complete"}}` + "\n" +
				`{"timestamp":"` + maintenance.Format(time.RFC3339) + `","type":"event_msg","payload":{"type":"thread_settings_applied"}}` + "\n"
			if err := os.WriteFile(filepath.Join(dir, "rollout-test-"+sid+".jsonl"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if cached {
				stampFinished(t, pane, maintenance)
			}
			stampFinished(t, "%3", newer)
			if err := state.WriteLastFinished("%3"); err != nil {
				t.Fatal(err)
			}
			lines := []string{
				paneLine(pane, "old", "0", "0", "Codex", "codex", maintenance.Unix(), 900002, cwd),
				paneLine("%3", "newer", "0", "0", "✳ done", "claude", newer.Unix(), 900003, "/work/newer"),
			}
			withFixture(t, lines, func() {
				for i := 0; i < 2; i++ {
					rows := GatherAgents()
					if len(rows) != 2 || rows[0].PaneID != "%3" || rows[1].PaneID != pane {
						t.Fatalf("old session promoted above newer completion: %+v", rows)
					}
					if rows[1].Status != "idle" || rows[1].Since != old.Unix() || rows[1].Latest {
						t.Fatalf("old completion changed: %+v", rows[1])
					}
				}
			})
			if got := FileMtime(state.FinishedPath(pane)); got != old.Unix() {
				t.Fatalf("persisted completion = %d, want %d", got, old.Unix())
			}
			if got := state.ReadLastFinished(); got != "%3" {
				t.Fatalf("timestamp repair claimed latest completion: %s", got)
			}
		})
	}
}

func TestCodexIdleTimeCorrectionRequiresSettledMatchingSession(t *testing.T) {
	for _, tc := range []struct {
		name, agent, homeID, cwd, marker, after string
		correct                                 bool
	}{
		{name: "completed", correct: true},
		{name: "new active turn", marker: "active"},
		{name: "pending approval", marker: "waiting"},
		{name: "different cwd", cwd: "/other"},
		{name: "different Codex home", homeID: "other-home"},
		{name: "different foreground agent", agent: "Claude Code"},
		{name: "new task boundary", after: `"type":"event_msg","payload":{"type":"task_started"}`},
		{name: "new message without boundary", after: `"type":"response_item","payload":{"type":"message","role":"user","content":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CODEX_HOME", "")
			const pane, loc, sid = "%7", "work:0.0", "bound-session"
			old := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
			cached := old.Add(time.Hour)
			stampFinished(t, pane, cached)
			if err := resume.Save(loc, resume.Record{Agent: "codex", SessionID: sid, Cwd: "/work", HomeID: tc.homeID}); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "16")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			log := `{"timestamp":"` + old.Format(time.RFC3339) + `","type":"event_msg","payload":{"type":"task_complete"}}` + "\n"
			if tc.after != "" {
				log += `{"timestamp":"` + cached.Format(time.RFC3339) + `",` + tc.after + "}\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "rollout-test-"+sid+".jsonl"), []byte(log), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.marker != "" {
				if err := state.WriteMarker(filepath.Join(state.Dir(), tc.marker, pane), sid); err != nil {
					t.Fatal(err)
				}
			}
			agent, cwd := tc.agent, tc.cwd
			if agent == "" {
				agent = "Codex"
			}
			if cwd == "" {
				cwd = "/work"
			}
			want := cached.Unix()
			if tc.correct {
				want = old.Unix()
			}
			if got := idleSince(pane, loc, cwd, agent, cached.Unix(), transcript.NewCodexPathScope()); got != want {
				t.Fatalf("idle time = %d, want %d", got, want)
			}
			if got := FileMtime(state.FinishedPath(pane)); got != want {
				t.Fatalf("cached time changed without evidence: %d, want %d", got, want)
			}
		})
	}
}

func TestCodexMaintenanceOnlyRolloutHasUnknownIdleTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	const pane, loc, sid = "%2", "old:0.0", "metadata-only"
	if err := resume.Save(loc, resume.Record{Agent: "codex", SessionID: sid, Cwd: "/work"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "16")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-test-"+sid+".jsonl"), []byte(`{"timestamp":"2026-10-16T06:33:50Z","type":"event_msg","payload":{"type":"thread_settings_applied"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := []string{paneLine(pane, "old", "0", "0", "Codex", "codex", time.Now().Unix(), 900002, "/work")}
	withFixture(t, lines, func() {
		rows := GatherAgents()
		if len(rows) != 1 || rows[0].Since != 0 {
			t.Fatalf("metadata-only rollout borrowed window activity: %+v", rows)
		}
	})
	if state.Exists(state.FinishedPath(pane)) {
		t.Fatal("unknown completion wrote a fresh marker")
	}
}

func stampFinished(t *testing.T, pane string, at time.Time) {
	t.Helper()
	if err := state.Touch(state.FinishedPath(pane)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(state.FinishedPath(pane), at, at); err != nil {
		t.Fatal(err)
	}
}
