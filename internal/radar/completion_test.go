package radar

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// A display-only timestamp must never gain lifecycle authority on a later scan.
func TestInferredFinishStaysUnconfirmed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	now := time.Now().Unix()
	lines := []string{paneLine("%711", "review", "0", "0", "✳ quiet", "claude", now, 900711, "/tmp/no-project")}
	withFixture(t, lines, func() {
		for scan := 1; scan <= 3; scan++ {
			rows := GatherAgents()
			if len(rows) != 1 || rows[0].Status != "idle" {
				t.Fatalf("scan %d: unexpected rows: %+v", scan, rows)
			}
			if rows[0].Confirmed {
				t.Errorf("scan %d: screen-only idle gained completion authority without any lifecycle event", scan)
			}
		}
		lines[0] = paneLine("%711", "review", "0", "0", "⠋ working", "claude", now, 900711, "/tmp/no-project")
		if rows := GatherAgents(); len(rows) != 1 || rows[0].Status != "working" {
			t.Fatalf("expected screen-only working row: %+v", rows)
		}
		lines[0] = paneLine("%711", "review", "0", "0", "✳ quiet", "claude", now, 900711, "/tmp/no-project")
		rows := GatherAgents()
		if len(rows) != 1 || rows[0].Status != "idle" {
			t.Fatalf("expected screen-only idle row: %+v", rows)
		}
		if rows[0].Confirmed {
			t.Error("screen-only working-to-idle edge is confirmed without a lifecycle event and will pass the hub's done-alert gate")
		}
	})
}

// The DST adapter records Stop in terminal-lifecycle, not finished/<pane>.
func TestNativeStopConfirmsCompletion(t *testing.T) {
	a, now := terminalFixture(t)
	originalAttachments, originalStuck := terminalAttachments, terminalStuckDispatch
	terminalAttachments = func() map[string]agentclient.Attachment { return map[string]agentclient.Attachment{a.Pane: a} }
	terminalStuckDispatch = func(string, string) string { return "" }
	defer func() { terminalAttachments, terminalStuckDispatch = originalAttachments, originalStuck }()
	lines := []string{paneLine(a.Pane, "review", "0", "0", "", "node", now.Unix(), a.PID, a.Cwd)}
	withFixture(t, lines, func() {
		if err := agentclient.ObserveLifecycle(a, "TurnStarted", "one", now.Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
		if rows := GatherAgents(); len(rows) != 1 || rows[0].Status != "working" {
			t.Fatalf("expected working native row: %+v", rows)
		}
		if err := agentclient.ObserveLifecycle(a, "Stop", "one", now.Add(-time.Second)); err != nil {
			t.Fatal(err)
		}
		rows := GatherAgents()
		if len(rows) != 1 || rows[0].Status != "idle" || rows[0].EndReason != "" {
			t.Fatalf("expected successfully completed native row: %+v", rows)
		}
		if state.Exists(state.FinishedPath(a.Pane)) {
			t.Fatal("fixture unexpectedly acquired a legacy finish marker")
		}
		if !rows[0].Confirmed {
			t.Fatal("native Stop is verified lifecycle evidence, but its completion is unconfirmed and the hub will suppress the done alert")
		}
	})
}

func TestCompletionProofMatchesSessionAndTurn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	const pane, loc = "%711", "review:0.0"
	if err := resume.Save(loc, resume.Record{Agent: "claude", SessionID: "current"}); err != nil {
		t.Fatal(err)
	}
	read := func() string {
		id, _, _ := paneCompletion(pane, loc, "/work", "Claude Code", 900711, "idle", agentclient.Attachment{}, transcript.NewCodexPathScope())
		return id
	}
	if err := state.RecordCompletion(pane, "claude", "old", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if read() != "" {
		t.Fatal("another session's Stop was accepted")
	}
	if err := state.RecordCompletion(pane, "claude", "current", "one", time.Now()); err != nil {
		t.Fatal(err)
	}
	_ = state.WriteMarker(state.TurnIDPath(pane), "two")
	if read() != "" {
		t.Fatal("another turn's Stop was accepted")
	}
	_ = state.WriteMarker(state.TurnIDPath(pane), "one")
	if read() == "" {
		t.Fatal("current lifecycle completion was not accepted")
	}
	_ = state.WriteMarker(state.ActivePath(pane), "current")
	if read() != "" {
		t.Fatal("a still-active turn was reported complete")
	}
}

func TestNativeStartupIdleIsNotACompletion(t *testing.T) {
	r := native.Record{Agent: "claude", SessionID: "one", State: "idle", UpdatedAt: time.Now().Unix(), Revision: "generation"}
	for _, event := range []string{"", "SessionStart", "StopFailure", "Interrupt"} {
		r.Event = event
		if id, _ := nativeCompletion(r, "idle", nil); id != "" {
			t.Fatalf("event %q acquired completion evidence: %s", event, id)
		}
	}
	r.Event = "Stop"
	if id, _ := nativeCompletion(r, "idle", nil); id == "" {
		t.Fatal("native Stop did not confirm completion")
	}
}
