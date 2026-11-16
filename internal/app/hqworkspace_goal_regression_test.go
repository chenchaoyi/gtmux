package app

import (
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"strings"
	"testing"
)

// This test deliberately uses only pre-change interfaces so the exact same
// regression fails on the fetched 183 baseline and passes on the candidate.
func TestGoalNoNewEventsAfterNotedStillRequiresHQ(t *testing.T) {
	s := hqworkspace.New(t.TempDir())
	m, e := s.Create("goal-one", "Continuing authorized fixture", 100)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Publish("stage-report", "goal-one", "", "progress", "Stage checked; goal still unfinished", "", 101); e != nil {
		t.Fatal(e)
	}
	q, e := s.ClaimObservation("message-stage-report", "synthetic-hq", 102)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Assess(hqworkspace.Assessment{ID: "noted-stage", ObservationID: q.Observation.ID, Owner: q.Owner, Generation: q.Generation, MatterID: m.ID, Revision: m.Revision, Decision: "noted", Reason: "Observation checked; unread is empty"}, 103, false); e != nil {
		t.Fatal(e)
	}
	old := workspaceReconcileWake
	t.Cleanup(func() { workspaceReconcileWake = old })
	wakes := []string{}
	workspaceReconcileWake = func(line string) { wakes = append(wakes, line) }
	if _, e = notifyWorkspaceReconciliation(s, 104, 0); e != nil {
		t.Fatal(e)
	}
	if len(wakes) != 1 || !strings.Contains(wakes[0], "workspace reconcile") {
		t.Fatal("unfinished goal disappeared after ordinary noted and empty unread", wakes)
	}
	d, e := s.Read()
	if e != nil {
		t.Fatal(e)
	}
	if len(hqworkspace.ReconciliationQueue(d, 104)) != 1 {
		t.Fatal("expected exactly one continuing-goal debt")
	}
	if _, e = notifyWorkspaceReconciliation(s, 105, 0); e != nil {
		t.Fatal(e)
	}
	if len(wakes) != 1 {
		t.Fatal("restart throttle forgot durable goal reminder")
	}
}
