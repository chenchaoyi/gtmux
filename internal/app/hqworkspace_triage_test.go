package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/conversation"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestWorkspaceTriageIngestion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", filepath.Join(state.Home(), ".codex"))
	oldBin := tmux.Bin
	tmux.Bin = ""
	t.Cleanup(func() { tmux.Bin = oldBin })
	source := &phase1CursorSource{}
	s, _, linked := phase1CursorStore(t, source, true)
	now := time.Now().Unix()
	for _, id := range []string{"one", "two"} {
		if _, err := s.Create(id, id, now); err != nil {
			t.Fatal(err)
		}
		c := hqworkspace.Conversation{Agent: "claude", SessionID: "overlap"}
		if err := s.Associate(id, hqworkspace.Association{ID: id, Conversation: c, Role: "execution", From: now - 10}, 0, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := resume.Save("hq:0.0", resume.Record{Agent: "claude", Cwd: state.HQHome(), SessionID: "old-hq", UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	home := conversation.HomeID("codex")
	rollout := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "11", "19", "rollout-2026-11-19T00-00-00-followed.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollout, []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"followed\",\"originator\":\"codex_desktop\",\"source\":\"vscode\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	follow, err := sessionpolicy.Save("followed", sessionpolicy.Settings{HQ: true})
	if err != nil {
		t.Fatal(err)
	}
	now = follow.FollowSince
	source.rows = []events.Record{
		{Seq: 1, Ts: now, Agent: "Claude Code", AgentSession: "plain", Pane: "%2", Session: "terminal", Event: "UserPromptSubmit", Origin: events.OriginInstruction},
		{Seq: 2, Ts: now, Agent: "Claude Code", AgentSession: "overlap", Pane: "%3", Session: "terminal", Event: "Stop"},
		{Seq: 3, Ts: now, Agent: "Claude Code", AgentSession: "old-hq", Pane: "%8", Session: "hq", Event: "Stop"},
		{Seq: 4, Ts: now, Agent: "Claude Code", Pane: "%5", Session: "missing", Event: "Stop"},
		{Seq: 5, Ts: now, Agent: "Codex", AgentSession: "not-followed", HomeID: home, Event: "Stop"},
		{Seq: 6, Ts: now, Agent: "Codex", AgentSession: "followed", HomeID: "foreign", Event: "Stop"},
		{Seq: 7, Ts: now - 1, Agent: "Codex", AgentSession: "followed", HomeID: home, Event: "Stop"},
		{Seq: 8, Ts: now, Agent: "Codex", AgentSession: "followed", HomeID: home, Event: "Stop"},
		phase1CursorEvent(linked, 9),
		{Seq: 10, Ts: now, Agent: "Claude Code", AgentSession: "sent", Pane: "%9", Session: "terminal", Event: "UserPromptSubmit", Origin: events.OriginInstruction, Actor: "hq"},
	}
	source.latest = 10
	if err := ingestWorkspaceObservations(s, now); err != nil {
		t.Fatal(err)
	}
	first := phase1CursorDatabase(t, s)
	if err := ingestWorkspaceObservations(s, now+1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, phase1CursorDatabase(t, s)) || first.ObservationCursors["events"] != 10 || len(first.Reconciliations) != 4 {
		t.Fatal("source cursor or dedup changed", first)
	}
	for id, kind := range map[string]string{"event-1": "triage-unassigned:UserPromptSubmit", "event-2": "triage-ambiguous:Stop", "event-8": "triage-unassigned:Stop"} {
		o := first.Reconciliations[id].Observation
		if o.Kind != kind || o.MatterID != "" || o.Continuation {
			t.Fatal(o)
		}
	}
	if o := first.Reconciliations["event-9"].Observation; !o.Continuation || o.MatterID != "cursor-goal" {
		t.Fatal("unique participant lost continuation", o)
	}
}
