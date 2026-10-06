package app

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// A native session (an agent outside tmux, sensed by its hooks) is listed with no tmux
// server, as the agents contract says. `agents --json`, /api/agents and the SSE snapshot
// all returned nothing first (%12, 2026-10-06). No tmux is reachable here (tmux.Bin is
// emptied), HOME is a temp dir, and the native record names this test's own process.
func TestAgentsWithoutTmuxListNativeSessions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_LANG", "en")
	saved := tmux.Bin
	tmux.Bin = ""
	t.Cleanup(func() { tmux.Bin = saved })
	if tmux.ServerUp() {
		t.Fatal("a tmux server is reachable; this test needs none")
	}
	if err := native.Save(native.Record{Agent: "claude", SessionID: "outside-tmux", Cwd: t.TempDir(),
		State: "idle", UpdatedAt: time.Now().Unix(), PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}

	var rows []struct {
		Source    string `json:"source"`
		SessionID string `json:"session_id"`
	}
	out := captureStdout(t, func() {
		if code := cmdAgents([]string{"--json"}); code != 0 {
			t.Errorf("agents --json exit = %d", code)
		}
	})
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("agents --json printed %q: %v", out, err)
	}
	if len(rows) != 1 || rows[0].Source != "native" || rows[0].SessionID != "outside-tmux" {
		t.Fatalf("agents --json = %s, want the native session", out)
	}

	statuses := serveAgentStatuses()
	if len(statuses) != 1 || statuses[0].PaneID != "" || statuses[0].SessionID != "outside-tmux" || statuses[0].Status != "idle" {
		t.Fatalf("SSE snapshot = %+v, want the native session, idle", statuses)
	}

	// The table still says there is no tmux server: that is true, and it is not JSON.
	if code := cmdAgents(nil); code != 1 {
		t.Errorf("agents (table) exit = %d, want 1 with no tmux server", code)
	}
}
