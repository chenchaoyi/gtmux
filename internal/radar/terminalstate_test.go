package radar

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func terminalFixture(t *testing.T) (agentclient.Attachment, time.Time) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	now := time.Now().Truncate(time.Second)
	a := agentclient.Attachment{Version: 1, Agent: "dsh", Client: "dst", Pane: "%99117", PID: 99117, Instance: strings.Repeat("a", 32), HomeID: dsh.HomeID(), SessionID: "synthetic-session", Cwd: "/synthetic/project", StartedAt: now.Add(-time.Hour).UnixMilli(), BindingEpoch: now.Add(-time.Hour).UnixMilli(), ServerEpoch: "synthetic", NativeSession: true}
	return a, now
}

func TestGatherNativeTerminalUsesHookRatherThanRepaintOrInferredMarkers(t *testing.T) {
	a, now := terminalFixture(t)
	oldAttachments, oldCapture, oldStuck := terminalAttachments, paneCapture, terminalStuckDispatch
	stuck := ""
	terminalStuckDispatch = func(string, string) string { return stuck }
	terminalAttachments = func() map[string]agentclient.Attachment { return map[string]agentclient.Attachment{a.Pane: a} }
	paneCapture = func(string) string { t.Fatal("native status sampled weak screen evidence"); return "" }
	defer func() { terminalAttachments, paneCapture, terminalStuckDispatch = oldAttachments, oldCapture, oldStuck }()
	lines := []string{paneLine(a.Pane, "synthetic", "0", "0", "", "node", now.Unix(), a.PID, a.Cwd)}
	read := func() Pane {
		t.Helper()
		var rows []Pane
		withFixture(t, lines, func() { rows = GatherAgents() })
		if len(rows) != 1 {
			t.Fatalf("rows=%d", len(rows))
		}
		return rows[0]
	}
	// These caches can be written by inference; they are never native proof.
	_ = state.Touch(state.FinishedPath(a.Pane))
	_ = state.WriteMarker(state.TurnOutcomePath(a.Pane), "error")
	_ = state.WriteMarker(state.ActivePath(a.Pane), a.SessionID)
	_ = state.WriteMarker(state.WaitingPath(a.Pane), "question")
	_ = state.WriteLastFinished(a.Pane)
	for i := 0; i < 3; i++ {
		if r := read(); r.Status != "running" || r.Since != 0 || r.Latest || r.Errored || r.EndReason != "" {
			t.Fatal("inference claimed a native phase", r)
		}
	}
	emit := func(event, turn string, at time.Time) {
		t.Helper()
		if err := agentclient.ObserveLifecycle(a, event, turn, at); err != nil {
			t.Fatal(err)
		}
	}
	emit("TurnStarted", "one", now.Add(-30*time.Minute))
	if r := read(); r.Status != "working" || r.Since != now.Add(-30*time.Minute).Unix() || r.EndReason != "" {
		t.Fatal(r)
	}
	emit("Waiting", "one", now.Add(-20*time.Minute))
	for i := 0; i < 3; i++ {
		if r := read(); r.Status != "waiting" || r.Since != now.Add(-20*time.Minute).Unix() {
			t.Fatal("native question was aged out", r)
		}
	}
	emit("Resumed", "one", now.Add(-time.Minute))
	if r := read(); r.Status != "working" {
		t.Fatal(r)
	}
	emit("Stop", "one", now.Add(-30*time.Second))
	for i := 0; i < 3; i++ {
		if r := read(); r.Status != "idle" || r.Since != now.Add(-30*time.Second).Unix() || !r.Latest || r.EndReason != "" {
			t.Fatal("completion was overridden", r)
		}
	}
	stuck = "draft"
	if r := read(); r.Status != "waiting" || r.Latest {
		t.Fatal("undelivered goal reported complete", r)
	}
	emit("TurnStarted", "two", now.Add(-20*time.Second))
	if r := read(); r.Status != "working" {
		t.Fatal("new native turn hidden", r)
	}
	stuck = "" // accepted work clears the separate pending-delivery fixture
	emit("StopFailure", "two", now.Add(-10*time.Second))
	if r := read(); r.Status != "idle" || !r.Errored || r.EndReason != "error" || r.Latest {
		t.Fatal(r)
	}
	a.SessionID = "another-session"
	if r := read(); r.Status != "running" || r.Since != 0 {
		t.Fatal("old binding inherited", r)
	}
}

func TestNativeTerminalExplicitHistoryRecoversMissingHook(t *testing.T) {
	a, now := terminalFixture(t)
	original := terminalHistory
	defer func() { terminalHistory = original }()
	s := dsh.Session{Header: dsh.Header{ID: a.SessionID, Cwd: a.Cwd}, HomeID: a.HomeID, State: "idle"}
	terminalHistory = func(string) (dsh.Session, error) { return s, nil }
	if got := terminalStatus(a, now); got.status != "running" {
		t.Fatal("default history idle became completion", got)
	}
	s.TurnAt = now.Add(-time.Minute).UnixMilli()
	if got := terminalStatus(a, now); got.status != "idle" || got.since != s.TurnAt/1000 {
		t.Fatal(got)
	}
	if err := agentclient.ObserveLifecycle(a, "Waiting", "one", now.Add(-30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := terminalStatus(a, now); got.status != "waiting" {
		t.Fatal("older durable start/end answered question", got)
	}
	s.TurnAt = now.Add(-10 * time.Second).UnixMilli()
	s.EndReason = "interrupted"
	if got := terminalStatus(a, now); got.status != "idle" || got.reason != "interrupted" {
		t.Fatal("new explicit boundary failed recovery", got)
	}
	for _, kind := range []string{"wrong session", "wrong home", "wrong cwd", "subagent", "old incarnation", "future", "missing"} {
		t.Run(kind, func(t *testing.T) {
			previous := s
			defer func() { s = previous }()
			switch kind {
			case "wrong session":
				s.ID = "other"
			case "wrong home":
				s.HomeID = "other"
			case "wrong cwd":
				s.Cwd = "/other"
			case "subagent":
				s.Origin = "subagent"
			case "old incarnation":
				s.TurnAt = a.StartedAt - 1
			case "future":
				s.TurnAt = now.Add(time.Minute).UnixMilli()
			case "missing":
				terminalHistory = func(string) (dsh.Session, error) { return dsh.Session{}, errors.New("missing") }
			}
			if got := terminalStatus(a, now); got.status != "waiting" {
				t.Fatal("unrelated history overrode native phase", got)
			}
		})
	}
}
