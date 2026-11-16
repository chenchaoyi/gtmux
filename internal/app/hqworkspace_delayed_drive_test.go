package app

import (
	"fmt"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

func TestGoalRelayAdapterFirstIngestAfterDeadlineDoesNotSend(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldRead, oldSeq, oldRelay := workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords
	oldHQ, oldSend := workspaceIsHQCaller, workspaceDeliverGoal
	oldOwner := workspaceAssessmentOwner
	t.Cleanup(func() {
		workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords = oldRead, oldSeq, oldRelay
		workspaceIsHQCaller, workspaceDeliverGoal = oldHQ, oldSend
		workspaceAssessmentOwner = oldOwner
	})
	workspaceReadEvents = func(int64) ([]events.Record, bool) { return nil, false }
	workspaceEventSequence = func() int64 { return 0 }
	workspaceIsHQCaller = func() bool { return true }
	workspaceAssessmentOwner = func() string { return "hq-fixture" }
	for _, kind := range []string{"native", "background"} {
		for _, source := range []string{"report", "ask", "wrong-sid"} {
			t.Run(kind+"/"+source, func(t *testing.T) {
				s := hqworkspace.New(t.TempDir())
				m, e := s.Create("goal-one", "Synthetic", 100)
				if e != nil {
					t.Fatal(e)
				}
				c := hqworkspace.Conversation{Agent: "codex", HomeID: "synthetic-home", SessionID: "worker-one"}
				if e = s.Associate(m.ID, hqworkspace.Association{ID: "worker", Conversation: c, Role: "execution", From: 100}, 0, 101); e != nil {
					t.Fatal(e)
				}
				if e = s.ActivateObservations(0, 101); e != nil {
					t.Fatal(e)
				}
				if e = s.ScanGoals(102, 64); e != nil {
					t.Fatal(e)
				}
				d, _ := s.Read()
				m = d.Matters[m.ID]
				q, e := s.ClaimObservation(hqworkspace.GoalFollowupID(m), "hq-fixture", 103)
				if e != nil {
					t.Fatal(e)
				}
				p := hqworkspace.Responsibility{StepID: "step-one", Action: "Finish fixture", AuthorityRef: "user:fixture", Intent: &hqworkspace.ActionIntent{ID: "action-one", Ref: "native:work-one", Target: c, At: 102}}
				if _, e = s.Assess(hqworkspace.Assessment{ID: "plan-one", ObservationID: q.Observation.ID, Owner: q.Owner, Generation: q.Generation, MatterID: m.ID, Revision: m.Revision, Decision: "responsibility", Reason: "Verified fixture", Responsibility: &p}, 103, false); e != nil {
					t.Fatal(e)
				}
				r := relay.Request{ID: "late-report", Kind: "report", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: 104, Body: fmt.Sprintf(`{"schema":"gtmux.goal-drive/v1","matter":"goal-one","drive":{"step_id":"step-one","action_id":"action-one","ref":"native:work-one","state":%q,"next_check":110}}`, kind)}
				if source == "ask" {
					r.Kind = "ask"
				}
				if source == "wrong-sid" {
					r.ThreadID = "reused-worker"
				}
				workspaceRelayRecords = func() []relay.Request { return []relay.Request{r} }
				if e = ingestWorkspaceObservations(s, 111); e != nil {
					t.Fatal(e)
				}
				d, _ = s.Read()
				if source != "report" {
					if len(d.IntentAttempts) != 0 {
						t.Fatal("invalid source created history", d.IntentAttempts)
					}
					return
				}
				calls := 0
				workspaceDeliverGoal = func(hqworkspace.Conversation, string, string, func() bool) hqworkspace.Delivery {
					calls++
					return hqworkspace.Delivery{State: "landed"}
				}
				_, e = applyWorkspaceAction(s, workspaceAction{Op: "drive", Matter: m.ID, ID: p.Intent.ID, Revision: d.Matters[m.ID].Revision}, 112)
				if e != nil || calls != 0 {
					t.Fatal("late relay report allowed replay through actual action adapter", calls, e)
				}
				a := d.IntentAttempts[p.Intent.ID]
				if a.State != "external" || a.At != 104 || a.Evidence != "relay-late-report" {
					t.Fatal("source creation provenance lost", a)
				}
				if g := hqworkspace.GoalResponsibility(d, d.Matters[m.ID], 112); g.State == "driving" || g.State == "queued" {
					t.Fatal("expired report asserted live coverage", g)
				}
			})
		}
	}
}
