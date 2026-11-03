package radar

import (
	"os"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestTargetPaneUsesFreshSharedVerdictWithoutScanningOthersOrReaping(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	now := time.Now().Unix()
	lines := []string{paneLine("%971", "target", "0", "0", "✳ ready", "claude", now, 900971, "/fixture"), paneLine("%972", "other", "0", "0", "codex", "codex", now, 900972, "/fixture")}
	original := paneCapture
	paneCapture = func(id string) string {
		if id != "%971" {
			t.Fatalf("unrelated capture %s", id)
		}
		return "❯ "
	}
	defer func() { paneCapture = original }()
	if err := state.WriteMarker(state.WaitingPath("%999"), "permission"); err != nil {
		t.Fatal(err)
	}
	withFixture(t, lines, func() {
		rows := GatherPaneAgents("%971")
		if len(rows) != 1 || rows[0].Status != "idle" {
			t.Fatal(rows)
		}
		if err := state.WriteMarker(state.WaitingPath("%971"), "permission"); err != nil {
			t.Fatal(err)
		}
		rows = GatherPaneAgents("%971")
		if len(rows) != 1 || rows[0].Status != "waiting" {
			t.Fatal("fresh question missed", rows)
		}
		if _, err := os.Stat(state.WaitingPath("%999")); err != nil {
			t.Fatal("target read reaped unrelated markers", err)
		}
		if rows := GatherPaneAgents("%973"); len(rows) != 0 {
			t.Fatal("missing pane invented", rows)
		}
	})
}

func TestTargetPaneKeepsNativeLifecycleAuthority(t *testing.T) {
	a, now := terminalFixture(t)
	oldAttachments, oldCapture, oldStuck := terminalAttachments, paneCapture, terminalStuckDispatch
	terminalAttachments = func() map[string]agentclient.Attachment { return map[string]agentclient.Attachment{a.Pane: a} }
	paneCapture = func(string) string { t.Fatal("native lifecycle replaced by screen polling"); return "" }
	terminalStuckDispatch = func(string, string) string { return "" }
	defer func() { terminalAttachments, paneCapture, terminalStuckDispatch = oldAttachments, oldCapture, oldStuck }()
	lines := []string{paneLine(a.Pane, "target", "0", "0", "", "node", now.Unix(), a.PID, a.Cwd)}
	withFixture(t, lines, func() {
		for _, phase := range []struct{ event, status string }{{"TurnStarted", "working"}, {"Waiting", "waiting"}, {"Stop", "idle"}} {
			if err := agentclient.ObserveLifecycle(a, phase.event, "one", now); err != nil {
				t.Fatal(err)
			}
			rows := GatherPaneAgents(a.Pane)
			if len(rows) != 1 || rows[0].Status != phase.status {
				t.Fatalf("%s: %+v", phase.event, rows)
			}
		}
	})
}
