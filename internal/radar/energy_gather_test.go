package radar

import (
	"testing"
	"time"
)

func TestPWR002GatherSharesFramesOnlyInsideOnePass(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := paneCapture
	t.Cleanup(func() { paneCapture = saved })
	calls := map[string]int{}
	frame := "› Ask Codex to do anything\n"
	paneCapture = func(id string) string { calls[id]++; return frame }
	now := time.Now().Unix()
	lines := []string{paneLine("%901", "energy", "0", "0", "Codex review", "codex", now, 900001, "/synthetic/no-repo"), paneLine("%902", "energy", "0", "1", "Codex next", "codex", now, 900002, "/synthetic/no-repo"), paneLine("%903", "energy", "0", "2", "✳ ready", "claude", now, 900003, "/synthetic/no-repo")}
	withFixture(t, lines, func() {
		rows := GatherAgents()
		if len(rows) != 3 {
			t.Fatalf("rows=%+v", rows)
		}
		for _, id := range []string{"%901", "%902"} {
			if calls[id] != 1 {
				t.Errorf("pane %s frame count=%d want 1", id, calls[id])
			}
		}
		frame = "Would you like to run this command?\n› 1. Yes, proceed\n  2. No, tell Codex what to do\nPress enter to confirm\n"
		rows = GatherAgents()
		for _, id := range []string{"%901", "%902"} {
			if calls[id] != 2 {
				t.Errorf("pane %s next-pass count=%d want 2", id, calls[id])
			}
		}
		for _, r := range rows {
			if r.Agent == "Codex" && r.Status != "waiting" {
				t.Errorf("new approval missed: %+v", r)
			}
			if r.PaneID == "%903" && r.Status != "idle" {
				t.Errorf("non-Codex changed: %+v", r)
			}
		}
	})
}
