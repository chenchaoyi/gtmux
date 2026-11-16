package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

func TestGoalActionAuthorityAndExactReceiptEnvelope(t *testing.T) {
	s := hqworkspace.New(t.TempDir())
	m, e := s.Create("goal-one", "Fixture", 100)
	if e != nil {
		t.Fatal(e)
	}
	oldHQ, oldOwner, oldSend := workspaceIsHQCaller, workspaceAssessmentOwner, workspaceDeliverGoal
	t.Cleanup(func() { workspaceIsHQCaller, workspaceAssessmentOwner, workspaceDeliverGoal = oldHQ, oldOwner, oldSend })
	workspaceIsHQCaller = func() bool { return false }
	workspaceAssessmentOwner = func() string { return "hq-one" }
	if _, e = applyWorkspaceAction(s, workspaceAction{Op: "drive", Matter: m.ID, ID: "action-one", Revision: m.Revision}, 101); e == nil {
		t.Fatal("non-HQ drive allowed")
	}
	c := hqworkspace.Conversation{Agent: "codex", HomeID: "home-one", SessionID: "session-one"}
	if e = s.Associate(m.ID, hqworkspace.Association{ID: "worker-one", Conversation: c, Role: "execution", From: 100}, 0, 102); e != nil {
		t.Fatal(e)
	}
	if e = s.ScanGoals(103, 64); e != nil {
		t.Fatal(e)
	}
	d, _ := s.Read()
	m = d.Matters[m.ID]
	workspaceIsHQCaller = func() bool { return true }
	value, e := applyWorkspaceAction(s, workspaceAction{Op: "claim", ObservationID: hqworkspace.GoalFollowupID(m)}, 104)
	if e != nil {
		t.Fatal(e)
	}
	q := value.(hqworkspace.Reconciliation)
	p := hqworkspace.Responsibility{StepID: "step-one", Action: "Finish fixture", AuthorityRef: "user:instruction", Intent: &hqworkspace.ActionIntent{ID: "action-one", Ref: "message:action-one", Target: c, At: 103}}
	a := workspaceAction{Op: "assess", ID: "plan-one", ObservationID: q.Observation.ID, Generation: q.Generation, Matter: m.ID, Revision: m.Revision, Decision: "responsibility", Body: "Explicit current plan", Responsibility: &p, DryRun: true}
	before, _ := s.Read()
	if _, e = applyWorkspaceAction(s, a, 105); e != nil {
		t.Fatal(e)
	}
	after, _ := s.Read()
	b1, _ := json.Marshal(before)
	b2, _ := json.Marshal(after)
	if string(b1) != string(b2) {
		t.Fatal("plan dry run wrote")
	}
	a.DryRun = false
	if _, e = applyWorkspaceAction(s, a, 105); e != nil {
		t.Fatal(e)
	}
	d, _ = s.Read()
	calls := 0
	workspaceDeliverGoal = func(target hqworkspace.Conversation, body, id string, current func() bool) hqworkspace.Delivery {
		calls++
		if !target.Same(c) || !current() {
			t.Fatal("wrong target")
		}
		return hqworkspace.Delivery{State: "queued"}
	}
	drive := workspaceAction{Op: "drive", ID: p.Intent.ID, Matter: m.ID, Revision: d.Matters[m.ID].Revision}
	if _, e = applyWorkspaceAction(s, drive, 106); e != nil {
		t.Fatal(e)
	}
	if _, e = applyWorkspaceAction(s, drive, 107); e != nil || calls != 1 {
		t.Fatal("action replayed", calls, e)
	}
	body := `{"schema":"gtmux.goal-drive/v1","matter":"goal-one","drive":{"step_id":"step-one","action_id":"action-one","ref":"message:action-one","state":"native","next_check":1000}}`
	if proof := workspaceRelayDriveProof(body, m.ID); proof == nil || proof.State != "native" {
		t.Fatal(proof)
	}
	for _, bad := range []string{body + ` {}`, strings.Replace(body, `"native"`, `"queued"`, 1), strings.Replace(body, `"goal-one"`, `"other-goal"`, 1), `{"schema":"gtmux.goal-drive/v1","matter":"goal-one","session_id":"imposter"}`} {
		if workspaceRelayDriveProof(bad, m.ID) != nil {
			t.Fatal("ambiguous or unauthorized envelope accepted", bad)
		}
	}
	// A question is not a delivery/progress report, even if its JSON resembles one.
	oldRead, oldSeq, oldRelay := workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords
	t.Cleanup(func() { workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords = oldRead, oldSeq, oldRelay })
	workspaceReadEvents = func(int64) ([]events.Record, bool) { return nil, false }
	workspaceEventSequence = func() int64 { return 0 }
	kind := "ask"
	workspaceRelayRecords = func() []relay.Request {
		return []relay.Request{{ID: "proof-" + kind, Kind: kind, Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, Body: body, CreatedAt: 110}}
	}
	if e = s.ActivateObservations(0, 108); e != nil {
		t.Fatal(e)
	}
	if e = ingestWorkspaceObservations(s, 111); e != nil {
		t.Fatal(e)
	}
	d, _ = s.Read()
	if g := hqworkspace.GoalResponsibility(d, d.Matters[m.ID], 111); g.State != "queued" {
		t.Fatal("ask became execution proof", g)
	}
	kind = "report"
	if e = ingestWorkspaceObservations(s, 112); e != nil {
		t.Fatal(e)
	}
	d, _ = s.Read()
	if g := hqworkspace.GoalResponsibility(d, d.Matters[m.ID], 112); g.State != "driving" {
		t.Fatal("exact report proof did not attach", g)
	}

}

func TestGoalReminderRechecksCurrentPremiseAndAssessmentLayers(t *testing.T) {
	s := hqworkspace.New(t.TempDir())
	m, e := s.Create("goal-one", "Fixture", 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Update(m.ID, "", "", "Verified 150", m.Revision, 101); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Publish("report-206", m.ID, "", "progress", "Reported 206, accepted 181", "", 102); e != nil {
		t.Fatal(e)
	}
	d, _ := s.Read()
	q, _ := s.ClaimObservation("message-report-206", "hq-one", 103)
	if _, e = s.Assess(hqworkspace.Assessment{ID: "noted-one", ObservationID: q.Observation.ID, Owner: q.Owner, Generation: q.Generation, MatterID: m.ID, Revision: d.Matters[m.ID].Revision, Decision: "noted", Reason: "Source checked; assessment unchanged"}, 104, false); e != nil {
		t.Fatal(e)
	}
	if e = s.ScanGoals(105, 64); e != nil {
		t.Fatal(e)
	}
	d, _ = s.Read()
	p := hqworkspace.ProjectMatter(d, d.Matters[m.ID], 106)
	if p.Assessment.Body != "Verified 150" || p.LatestReceipt.Body != "Reported 206, accepted 181" {
		t.Fatal(p)
	}
	if keep, _ := revalidateWorkspaceReconciliation(d, 106, "workspace reconciliation stale"); !keep {
		t.Fatal("live HQ debt dropped")
	}
	if e = s.Update(m.ID, "", "completed", "Verified whole-goal result", d.Matters[m.ID].Revision, 107); e != nil {
		t.Fatal(e)
	}
	d, _ = s.Read() // deliberately do not scan: probe must recheck stale pending debt itself.
	if keep, _ := revalidateWorkspaceReconciliation(d, 108, "workspace reconciliation stale"); keep {
		t.Fatal("stale reminder survived completion")
	}
}
