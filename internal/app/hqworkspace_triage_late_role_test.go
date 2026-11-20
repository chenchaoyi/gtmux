package app

import (
	"strconv"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestWorkspaceTriageLateWorkerCannotClaimSupervisorRole(t *testing.T) {
	r := newWorkspaceCLIReplay(t)
	s := workspaceStore()
	if err := s.ActivateObservations(events.CurrentSeq(), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	const workerID = "late-worker"
	const hqID = "actual-supervisor"
	old := triageRolePane(t, "worker", r.home)
	triageRoleHook(t, r, old, workerID, r.home, "SessionStart")
	if !tmux.OK("kill-server") {
		t.Fatal("failed to restart the task's private tmux")
	}
	hq := triageRolePane(t, "hq", state.HQHome())
	if old != hq {
		t.Fatal("fixture requires reused numeric pane", old, hq)
	}
	hqpane.Stamp(hq)
	triageRoleHook(t, r, old, workerID, r.home, "Stop")
	triageRoleHook(t, r, old, workerID, "", "Stop")
	triageRoleHook(t, r, hq, hqID, state.HQHome(), "SessionStart")
	triageRoleHook(t, r, hq, hqID, state.HQHome(), "Stop")

	rows, gap := events.ReadSince(0)
	if gap {
		t.Fatal("fixture lost the source interval")
	}
	var late []events.Record
	var genuine events.Record
	for _, row := range rows {
		if row.Event != "Stop" {
			continue
		}
		if row.AgentSession == workerID {
			late = append(late, row)
		}
		if row.AgentSession == hqID {
			genuine = row
		}
	}
	if len(late) != 2 || genuine.Seq == 0 || genuine.TerminalRole != events.TerminalRoleHQ {
		t.Fatal(late, genuine)
	}
	for _, row := range late {
		if row.Pane != hq || row.TerminalRole != events.TerminalRoleUnverified {
			t.Fatalf("late source %s at events:%d did not retain an unverified role: %q", row.AgentSession, row.Seq, row.TerminalRole)
		}
	}
	if err := ingestWorkspaceObservations(s, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	d, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range late {
		q := d.Reconciliations["event-"+strconv.FormatInt(row.Seq, 10)]
		if q.Observation.Conversation.SessionID != workerID || q.Observation.Kind != "triage-unassigned:Stop" || q.State != "pending" {
			t.Fatal("late worker evidence was not retained", row.Seq, q)
		}
	}
	if _, ok := d.Reconciliations["event-"+strconv.FormatInt(genuine.Seq, 10)]; ok || d.ObservationCursors["events"] < genuine.Seq {
		t.Fatal("HQ echo counted or cursor did not advance", d.Reconciliations)
	}
	t.Logf("late worker events:%d and %d kept after reuse of %s; genuine HQ events:%d excluded",
		late[0].Seq, late[1].Seq, hq, genuine.Seq)
}
