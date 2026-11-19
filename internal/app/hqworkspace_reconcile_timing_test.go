package app

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

func TestWorkspaceReconciliationRetriesFromReminderTime(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := workspaceStore()
	matter, err := s.Create("clock-fixture", "Verify wake retry", 100)
	if err != nil {
		t.Fatal(err)
	}
	source := hqworkspace.Conversation{Agent: "codex", HomeID: "clock-home", SessionID: "clock-session"}
	if err := s.Associate(matter.ID, hqworkspace.Association{
		ID: "clock-source", Role: "execution", Conversation: source, From: 100,
	}, 0, 101); err != nil {
		t.Fatal(err)
	}
	if err := s.ActivateObservations(10, 102); err != nil {
		t.Fatal(err)
	}

	oldRead, oldSeq, oldRelay, oldObserve := workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceObserveEvent
	oldNext, oldWakeAt, oldStamp := workspaceReconciliationNext, workspaceReconciliationWakeAt, workspaceGoalCheckStamp
	oldNow, oldWake := workspaceReconciliationNow, workspaceReconcileWake
	t.Cleanup(func() {
		workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceObserveEvent = oldRead, oldSeq, oldRelay, oldObserve
		workspaceReconciliationNext, workspaceReconciliationWakeAt, workspaceGoalCheckStamp = oldNext, oldWakeAt, oldStamp
		workspaceReconciliationNow, workspaceReconcileWake = oldNow, oldWake
	})

	var clock atomic.Int64
	workspaceReconciliationNow = clock.Load
	workspaceReconciliationNext, workspaceReconciliationWakeAt, workspaceGoalCheckStamp = 0, 1000, 0
	workspaceEventSequence = func() int64 { return 11 }
	workspaceRelayRecords = func() []relay.Request { return nil }
	workspaceObserveEvent = func(string, string, string, string, int64, ...string) bool { return true }
	workspaceReadEvents = func(after int64) ([]events.Record, bool) {
		if after >= 11 {
			return nil, false
		}
		clock.Store(111) // the real scan starts at 110 and the reminder is recorded at 111
		return []events.Record{{
			Seq: 11, Ts: 110, Event: "Stop", Agent: "Codex", HomeID: source.HomeID,
			AgentSession: source.SessionID,
		}}, false
	}
	attempts := 0
	workspaceReconcileWake = func(string) { attempts++ } // intentionally do not deliver or claim
	runAt := func(at int64, want int) {
		t.Helper()
		clock.Store(at)
		scheduleWorkspaceReconciliation()
		deadline := time.Now().Add(3 * time.Second)
		for workspaceReconciliationRunner.running.Load() {
			if time.Now().After(deadline) {
				t.Fatal("workspace runner did not settle")
			}
			time.Sleep(time.Millisecond)
		}
		if attempts != want {
			t.Fatalf("at %d got %d wake attempts, want %d", at, attempts, want)
		}
	}

	runAt(110, 1)
	runAt(140, 1)
	runAt(141, 2)
	d, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	q := d.Reconciliations["event-11"]
	if q.State != "pending" || q.ReminderAt != 141 || q.ReminderCount != 2 || q.ClaimedAt != 0 {
		t.Fatalf("lost wake must remain exact pending evidence: %+v", q)
	}
	t.Log("scan 110, first reminder 111, first eligible retry 141")
}
