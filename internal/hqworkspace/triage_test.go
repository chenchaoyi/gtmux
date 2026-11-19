package hqworkspace

import (
	"reflect"
	"testing"
)

func TestWorkspaceTriageAssessment(t *testing.T) {
	s := New(t.TempDir())
	c := Conversation{Agent: "claude", SessionID: "worker"}
	o := Observation{ID: "event-1", Kind: "triage-ambiguous:Stop", Ref: "events:1", Conversation: c, OccurredAt: 100}
	p := Observation{ID: "event-2", Kind: "triage-unassigned:UserPromptSubmit", Ref: "events:2", Conversation: c, OccurredAt: 100}
	if p.sourceKind() != "UserPromptSubmit" || o.sourceKind() != "Stop" {
		t.Fatal("triage masked the source lifecycle kind")
	}
	if err := s.Ingest("events", 0, 2, []Observation{o, p}, 102); err != nil {
		t.Fatal(err)
	}
	if err := s.Ingest("events", 2, 2, []Observation{o, p}, 103); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"first", "second"} {
		if _, err := s.Create(id, id, 103); err != nil {
			t.Fatal(err)
		}
		if err := s.Associate(id, Association{ID: "worker", Conversation: c, Role: "execution", From: 99}, 0, 103); err != nil {
			t.Fatal(err)
		}
	}
	db := func() Database {
		t.Helper()
		d, err := s.Read()
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	claim := func(id, owner string, at int64) Reconciliation {
		t.Helper()
		q, err := s.ClaimObservation(id, owner, at)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	d := db()
	if got := ObservationMatters(d, c, 100); !reflect.DeepEqual(got, []string{"first", "second"}) {
		t.Fatal(got)
	}
	if d.Reconciliations[o.ID].ReceivedAt != 102 || len(ContinuationWakeQueue(d, 104)) != 2 {
		t.Fatal("source replay added debt or delayed the exact evidence")
	}
	if err := s.ScanGoals(104, 64); err != nil {
		t.Fatal(err)
	}
	first := d.Matters["first"]
	key := GoalFollowupID(first)
	g := claim(key, "old-hq", 105)
	if _, err := s.Assess(Assessment{ID: "old-defer", ObservationID: key, Owner: g.Owner, Generation: g.Generation, MatterID: first.ID, Revision: first.Revision, Decision: "deferred", NextCheck: 2000, Reason: "old wait"}, 106, false); err != nil {
		t.Fatal(err)
	}
	q := claim(o.ID, "old-hq", 107)
	if rows := ContinuationWakeQueue(db(), 108); len(rows) != 1 || rows[0].Observation.ID != p.ID {
		t.Fatal("an exact claim covered its sibling", rows)
	}
	newQ := claim(o.ID, "new-hq", 410)
	a := Assessment{ID: "handoff", ObservationID: o.ID, Owner: q.Owner, Generation: q.Generation, MatterID: first.ID, Revision: first.Revision, Decision: "handoff", Reason: "Reviewed open step and selected the first goal", ReviewRef: "workspace:verified worker transcript"}
	if _, err := s.Assess(a, 411, false); err == nil {
		t.Fatal("expired owner could assess")
	}
	a.Owner, a.Generation = newQ.Owner, newQ.Generation
	for _, decision := range []string{"noted", "conflict", "responsibility"} {
		bad := a
		bad.Decision = decision
		if _, err := s.Assess(bad, 411, false); err == nil {
			t.Fatal("ordinary disposition cleared triage", decision)
		}
	}
	bad := a
	bad.ReviewRef = ""
	if _, err := s.Assess(bad, 411, false); err == nil {
		t.Fatal("anonymous disposition accepted")
	}
	bad = a
	bad.Revision++
	if _, err := s.Assess(bad, 411, false); err == nil {
		t.Fatal("stale matter version accepted")
	}
	receipt, err := s.Assess(a, 411, false)
	if err != nil || receipt.HandoffID != key || receipt.QueueState != "assessed" {
		t.Fatal("missing handoff receipt", receipt, err)
	}
	again, err := s.Assess(a, 412, false)
	d = db()
	if err != nil || !reflect.DeepEqual(again, receipt) || d.Reconciliations[key].State != "pending" || d.Reconciliations[o.ID].Observation != o || len(d.IntentAttempts) != 0 || len(d.Messages) != 0 {
		t.Fatal("handoff did not preserve exact source and persistent recipient", again, err, d.Reconciliations[key])
	}
	q = claim(p.ID, "new-hq", 413)
	a = Assessment{ID: "later", ObservationID: p.ID, Owner: q.Owner, Generation: q.Generation, Decision: "deferred", Reason: "Waiting for a separately verified result", ReviewRef: "workspace:external result", NextCheck: 500}
	if receipt, err = s.Assess(a, 414, false); err != nil || receipt.QueueState != "deferred" {
		t.Fatal(receipt, err)
	}
	if rows := ContinuationWakeQueue(db(), 499); len(rows) != 0 {
		t.Fatal("premature future check", rows)
	}
	q = claim(p.ID, "new-hq", 501)
	a.ID, a.Decision, a.Generation, a.NextCheck = "closed-review", "no-followup", q.Generation, 0
	if receipt, err = s.Assess(a, 502, false); err != nil || receipt.QueueState != "assessed" {
		t.Fatal(receipt, err)
	}
	g = claim(key, "new-hq", 503)
	pause := &Responsibility{StepID: "paused-step", Action: "Await the user's explicit resumption", AuthorityRef: "user:pause",
		Hold: &GoalHold{Kind: "paused", SourceRef: "user:pause", Condition: "Explicit resumption"}}
	if _, err := s.Assess(Assessment{ID: "pause-goal", ObservationID: key, Owner: g.Owner, Generation: g.Generation,
		Decision: "responsibility", Reason: "Reviewed user pause", MatterID: first.ID, Revision: first.Revision,
		Responsibility: pause}, 504, false); err != nil {
		t.Fatal(err)
	}
	stopped := o
	stopped.ID, stopped.Ref, stopped.OccurredAt = "event-3", "events:3", 505
	if err := s.Ingest("events", 2, 3, []Observation{stopped}, 505); err != nil {
		t.Fatal(err)
	}
	first = db().Matters[first.ID]
	q = claim(stopped.ID, "new-hq", 506)
	a = Assessment{ID: "held-handoff", ObservationID: stopped.ID, Owner: q.Owner, Generation: q.Generation,
		Decision: "handoff", Reason: "Verified the user pause remains active", ReviewRef: "user:pause", MatterID: first.ID, Revision: first.Revision}
	if receipt, err = s.Assess(a, 507, false); err != nil || receipt.HandoffID != GoalFollowupID(first) {
		t.Fatal(receipt, err)
	}
	d = db()
	if !reflect.DeepEqual(d.Matters[first.ID].Responsibility, pause) || GoalResponsibility(d, d.Matters[first.ID], 507).State != "held" || len(d.IntentAttempts) != 0 {
		t.Fatal("handoff changed the pause or attempted delivery")
	}

}
