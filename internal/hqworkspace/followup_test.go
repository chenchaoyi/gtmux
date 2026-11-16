package hqworkspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func goalPlan(t *testing.T, s *Store, p Responsibility, now int64, resume string) {
	t.Helper()
	if e := s.ScanGoals(now, 64); e != nil {
		t.Fatal(e)
	}
	d := phaseRead(t, s)
	m := d.Matters["goal-one"]
	q, e := s.ClaimObservation(goalKey(m), "hq-current", now)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Assess(Assessment{ID: fmt.Sprintf("plan-%d", now), ObservationID: q.Observation.ID, Owner: q.Owner, Generation: q.Generation, MatterID: m.ID, Revision: m.Revision, Decision: "responsibility", Reason: "Verified next responsibility", Responsibility: &p, ResumeRef: resume}, now, false)
	if e != nil {
		t.Fatal(e)
	}
}
func goalFixture(t *testing.T) (*Store, Responsibility) {
	t.Helper()
	s := phaseStore(t)
	c := Conversation{Agent: "codex", HomeID: "home-one", SessionID: "session-one", Pane: "%99"}
	if e := s.Associate("goal-one", Association{ID: "worker-one", Conversation: c, Role: "execution", From: 100}, 0, 101); e != nil {
		t.Fatal(e)
	}
	return s, Responsibility{StepID: "step-one", Action: "Finish the authorized fixture", AuthorityRef: "user:instruction-one", Intent: &ActionIntent{ID: "action-one", Ref: "message:action-one", Target: c, At: 102}}
}
func goalState(t *testing.T, s *Store, now int64) GoalStatus {
	t.Helper()
	d := phaseRead(t, s)
	return GoalResponsibility(d, d.Matters["goal-one"], now)
}

func TestGoalDebtSurvivesNotedEmptyUnreadRestartAndRotation(t *testing.T) {
	s := phaseStore(t)
	if _, e := s.Publish("stage-one", "goal-one", "", "progress", "Stage complete", "", 101); e != nil {
		t.Fatal(e)
	}
	q, e := s.ClaimObservation("message-stage-one", "hq-old", 102)
	if e != nil {
		t.Fatal(e)
	}
	a := Assessment{ID: "noted-one", ObservationID: q.Observation.ID, Owner: q.Owner, Generation: q.Generation, MatterID: "goal-one", Revision: phaseRevision(t, s), Decision: "noted", Reason: "Observation checked; unread watermark is empty"}
	if _, e = s.Assess(a, 103, false); e != nil {
		t.Fatal(e)
	}
	if e = s.ScanGoals(104, 64); e != nil {
		t.Fatal(e)
	}
	d := phaseRead(t, s)
	id := goalKey(d.Matters["goal-one"])
	before := len(d.Reconciliations)
	old, e := s.ClaimObservation(id, "hq-old", 105)
	if e != nil {
		t.Fatal(e)
	}
	bad := Assessment{ID: "false-noted", ObservationID: id, Owner: old.Owner, Generation: old.Generation, MatterID: "goal-one", Revision: phaseRevision(t, s), Decision: "noted", Reason: "All reports noted; stop"}
	if _, e = s.Assess(bad, 106, false); e == nil {
		t.Fatal("noted cleared goal debt")
	}
	for n := int64(107); n < 115; n++ {
		s = New(filepath.Dir(s.dir))
		if e = s.ScanGoals(n, 64); e != nil {
			t.Fatal(e)
		}
	}
	d = phaseRead(t, s)
	if len(d.Reconciliations) != before || goalState(t, s, 115).State != "verify" {
		t.Fatal("tick/restart lost or duplicated debt", d)
	}
	fresh, e := s.ClaimObservation(id, "hq-new", 406)
	if e != nil {
		t.Fatal(e)
	}
	bad.ID = "stale-generation"
	bad.Decision = "deferred"
	bad.NextCheck = 1000
	if _, e = s.Assess(bad, 407, false); !errors.Is(e, ErrConflict) {
		t.Fatal("old generation committed", e)
	}
	if fresh.Generation <= old.Generation {
		t.Fatal("rotation did not fence old owner")
	}
	// Renaming and unrelated reports do not create a different material step.
	if e = s.Update("goal-one", "Renamed", "", "", phaseRevision(t, s), 408); e != nil {
		t.Fatal(e)
	}
	if e = s.ScanGoals(409, 64); e != nil {
		t.Fatal(e)
	}
	if got := goalKey(phaseRead(t, s).Matters["goal-one"]); got != id {
		t.Fatal("rename manufactured debt")
	}
}

func TestGoalQueuedReceiptAndLateReminderNeverDispatchTwice(t *testing.T) {
	s, p := goalFixture(t)
	goalPlan(t, s, p, 103, "")
	calls := 0
	send := func(c Conversation, body, id string, current func() bool) Delivery {
		calls++
		if !c.Same(p.Intent.Target) || id != p.Intent.ID || !current() {
			t.Fatal("wrong addressed intent")
		}
		return Delivery{State: "queued", Evidence: "durable-queue-receipt"}
	}
	rev := phaseRevision(t, s)
	out, e := s.DriveIntent("goal-one", p.Intent.ID, rev, 104, send)
	if e != nil || out.State != "queued" {
		t.Fatal(out, e)
	}
	if g := goalState(t, s, 105); g.State != "queued" {
		t.Fatal("queued reported started", g)
	}
	for n := int64(106); n < 110; n++ {
		if e = s.ScanGoals(n, 64); e != nil {
			t.Fatal(e)
		}
		if _, e = s.DriveIntent("goal-one", p.Intent.ID, rev, n, send); e != nil {
			t.Fatal(e)
		}
	}
	proof := Observation{ID: "received-one", Kind: "relay-report", MatterID: "goal-one", Ref: "relay:receipt-one", Conversation: p.Intent.Target, OccurredAt: 110, DriveProof: &DriveProof{StepID: p.StepID, ActionID: p.Intent.ID, Ref: p.Intent.Ref, State: "progress", NextCheck: 1000}}
	if e = s.Ingest("test-receipts", 0, 1, []Observation{proof}, 111); e != nil {
		t.Fatal(e)
	}
	if g := goalState(t, s, 112); g.State != "driving" || g.ObservationID != "received-one" {
		t.Fatal(g)
	}
	s = New(filepath.Dir(s.dir))
	if _, e = s.DriveIntent("goal-one", p.Intent.ID, rev, 113, send); e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatal("queued, receipt or restart replayed", calls)
	}
	if e = s.Update("goal-one", "", "completed", "Verified terminal result", phaseRevision(t, s), 114); e != nil {
		t.Fatal(e)
	}
	if e = s.ScanGoals(115, 64); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DriveIntent("goal-one", p.Intent.ID, rev, 116, send); e != nil {
		t.Fatal(e)
	}
	if calls != 1 || len(ReconciliationQueue(phaseRead(t, s), 116)) != 2 {
		t.Fatal("late reminder replayed or source receipt vanished")
	}
}

func TestGoalCrashDraftAndIdentityRefusalAreNotRetryQueues(t *testing.T) {
	for _, verdict := range []string{"crash", "draft", "identity"} {
		t.Run(verdict, func(t *testing.T) {
			s, p := goalFixture(t)
			goalPlan(t, s, p, 103, "")
			rev := phaseRevision(t, s)
			calls := 0
			send := func(Conversation, string, string, func() bool) Delivery {
				calls++
				if verdict == "crash" {
					panic("after submit, before receipt write")
				}
				return Delivery{State: verdict, SafeHold: true}
			}
			func() {
				defer func() {
					if verdict == "crash" {
						_ = recover()
					}
				}()
				_, _ = s.DriveIntent("goal-one", p.Intent.ID, rev, 104, send)
			}()
			s = New(filepath.Dir(s.dir))
			if _, e := s.DriveIntent("goal-one", p.Intent.ID, rev, 500, send); e != nil {
				t.Fatal(e)
			}
			if calls != 1 {
				t.Fatal("uncertain/refused action retried", calls)
			}
			if g := goalState(t, s, 500); !goalOwed(g) {
				t.Fatal("missing receipt became execution", g)
			}
		})
	}
}

func TestGoalNativeBackgroundExactStepAndIdentity(t *testing.T) {
	for _, kind := range []string{"native", "background"} {
		t.Run(kind, func(t *testing.T) {
			s, p := goalFixture(t)
			goalPlan(t, s, p, 103, "")
			proof := Observation{ID: "native-proof", Kind: "relay-report", MatterID: "goal-one", Conversation: p.Intent.Target, Ref: "native:goal-one", OccurredAt: 104, DriveProof: &DriveProof{StepID: p.StepID, ActionID: p.Intent.ID, Ref: p.Intent.Ref, State: kind, NextCheck: 1000}}
			if e := s.Ingest("native", 0, 1, []Observation{proof}, 105); e != nil {
				t.Fatal(e)
			}
			if g := goalState(t, s, 106); g.State != "driving" {
				t.Fatal(g)
			}
			if _, e := s.DriveIntent("goal-one", p.Intent.ID, phaseRevision(t, s), 107, func(Conversation, string, string, func() bool) Delivery {
				t.Fatal("idle native work redispatched")
				return Delivery{}
			}); e == nil {
				t.Fatal("duplicate native action allowed")
			}
			if e := s.Interval("goal-one", "worker-one", 100, 108, phaseRevision(t, s), 109); e != nil {
				t.Fatal(e)
			}
			reused := p.Intent.Target
			reused.SessionID = "new-session" // same pane
			if e := s.Associate("goal-one", Association{ID: "worker-new", Conversation: reused, Role: "execution", From: 109}, 0, 109); e != nil {
				t.Fatal(e)
			}
			if g := goalState(t, s, 110); !goalOwed(g) {
				t.Fatal("old SID still drives new interval", g)
			}
			if _, e := s.DriveIntent("goal-one", p.Intent.ID, phaseRevision(t, s), 111, func(Conversation, string, string, func() bool) Delivery {
				t.Fatal("sent to reused pane")
				return Delivery{}
			}); e == nil {
				t.Fatal("old target not refused")
			}
		})
	}
}

func TestGoalPauseRefusalExternalAndCrossChannelAnswer(t *testing.T) {
	for _, kind := range []string{"paused", "refused", "external"} {
		t.Run(kind, func(t *testing.T) {
			s, p := goalFixture(t)
			p.Intent = nil
			p.Hold = &GoalHold{Kind: kind, SourceRef: "user:pause-one", Condition: "explicit resumption or resource window"}
			if kind == "external" {
				p.Hold.NextCheck = 200
			}
			goalPlan(t, s, p, 103, "")
			if g := goalState(t, s, 150); g.State != "held" {
				t.Fatal(g)
			}
			if e := s.ScanGoals(1000, 64); e != nil {
				t.Fatal(e)
			}
			g := goalState(t, s, 1000)
			if kind == "external" && g.State != "owes-hq" || kind != "external" && g.State != "held" {
				t.Fatal("time resumed a hold", g)
			}
			if kind != "external" {
				d := phaseRead(t, s)
				m := d.Matters["goal-one"]
				q, e := s.ClaimObservation(goalKey(m), "hq-current", 1001)
				if e != nil {
					t.Fatal(e)
				}
				p.Hold = nil
				a := Assessment{ID: "resume-without-user", ObservationID: q.Observation.ID, Owner: q.Owner, Generation: q.Generation, MatterID: m.ID, Revision: m.Revision, Decision: "responsibility", Reason: "Keep going", Responsibility: &p}
				if _, e = s.Assess(a, 1002, false); e == nil {
					t.Fatal("pause silently resumed")
				}
			}
		})
	}
	s, p := goalFixture(t)
	p.Intent = nil
	goalPlan(t, s, p, 103, "")
	if e := s.RequestInput("goal-one", phaseQuestion("question-one"), phaseRevision(t, s), 104); e != nil {
		t.Fatal(e)
	}
	if g := goalState(t, s, 105); g.State != "waiting-user" {
		t.Fatal(g)
	}
	if e := s.ClearInput("goal-one", "question-one", "answer-verified", "Verified user reply through original terminal channel", phaseRevision(t, s), 106); e != nil {
		t.Fatal(e)
	}
	if g := goalState(t, s, 107); g.State != "owes-hq" {
		t.Fatal("answer hid next responsibility", g)
	}
}

func TestGoalLegacyMigrationBoundedSweepAndProjection(t *testing.T) {
	s := phaseStore(t)
	d := phaseRead(t, s)
	d.Schema = 2
	d.IntentAttempts = nil
	for i := 0; i < 140; i++ {
		id := fmt.Sprintf("old-%03d", i)
		d.Matters[id] = Matter{ID: id, State: "open", Revision: 1}
	}
	raw, _ := json.Marshal(d)
	if e := os.WriteFile(filepath.Join(s.dir, "records.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if e := s.ScanGoals(200+int64(i), 64); e != nil {
			t.Fatal(e)
		}
	}
	d = phaseRead(t, s)
	if len(d.Reconciliations) != 141 || len(d.IntentAttempts) != 0 {
		t.Fatal("legacy scan unbounded/lost debt", len(d.Reconciliations))
	}
	before := d
	if e := s.Update("goal-one", "", "", "Verified 150", d.Matters["goal-one"].Revision, 204); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Publish("receipt-206", "goal-one", "", "progress", "Reported 206, accepted 181", "", 206); e != nil {
		t.Fatal(e)
	}
	d = phaseRead(t, s)
	p := ProjectMatter(d, d.Matters["goal-one"], 207)
	if p.Assessment.Body != "Verified 150" || p.LatestReceipt.Body != "Reported 206, accepted 181" || p.LatestReceipt.SourceRef != "receipt-206" || p.Assessment.At != 204 {
		t.Fatal("latest receipt overwrote current assessment", p)
	}
	if reflect.DeepEqual(before, d) {
		t.Fatal("fixture did not change")
	}
	if e := s.Update("goal-one", "", "", "unversioned overwrite", 0, 208); !errors.Is(e, ErrConflict) {
		t.Fatal("zero revision updated assessment", e)
	}
	// Relay revision 0 remains an observation, never a fabricated current version.
	o := Observation{ID: "relay-new", Kind: "relay-report", MatterID: "goal-one", MatterRevision: 0, Body: "206"}
	if e := s.Ingest("relay-test", 0, 1, []Observation{o}, 209); e != nil {
		t.Fatal(e)
	}
	q, _ := s.ClaimObservation(o.ID, "hq-current", 210)
	if _, e := s.Assess(Assessment{ID: "bad-transition", ObservationID: o.ID, Owner: q.Owner, Generation: q.Generation, MatterID: "goal-one", Revision: phaseRevision(t, s), Decision: "transition", State: "open", Reason: "206"}, 211, false); !errors.Is(e, ErrConflict) {
		t.Fatal("revision zero crossed fence", e)
	}
}

func TestGoalRestoreSuspendsDriveAndRetainsPause(t *testing.T) {
	for _, paused := range []bool{false, true} {
		t.Run(fmt.Sprint(paused), func(t *testing.T) {
			s, p := goalFixture(t)
			if paused {
				p.Intent = nil
				p.Hold = &GoalHold{Kind: "paused", SourceRef: "user:pause", Condition: "wait for explicit resume"}
			}
			goalPlan(t, s, p, 103, "")
			if !paused {
				if _, e := s.DriveIntent("goal-one", p.Intent.ID, phaseRevision(t, s), 104, func(Conversation, string, string, func() bool) Delivery { return Delivery{State: "queued"} }); e != nil {
					t.Fatal(e)
				}
			}
			if e := s.MarkRestored(105); e != nil {
				t.Fatal(e)
			}
			g := goalState(t, s, 106)
			if paused && g.State != "held" || !paused && g.State != "verify" {
				t.Fatal("restore trusted old drive or lost pause", g)
			}
			if !paused {
				calls := 0
				if _, e := s.DriveIntent("goal-one", p.Intent.ID, phaseRevision(t, s), 107, func(Conversation, string, string, func() bool) Delivery { calls++; return Delivery{State: "landed"} }); e != nil {
					t.Fatal(e)
				}
				if calls != 0 {
					t.Fatal("restore replayed")
				}
			}
		})
	}
}

func TestGoalWrongStepAmbiguousIntervalAndDurableReminder(t *testing.T) {
	s, p := goalFixture(t)
	goalPlan(t, s, p, 103, "")
	wrong := Observation{ID: "wrong-proof", Kind: "relay-report", MatterID: "goal-one", Conversation: p.Intent.Target, OccurredAt: 104, DriveProof: &DriveProof{StepID: "another-step", ActionID: p.Intent.ID, Ref: p.Intent.Ref, State: "background", NextCheck: 1000}}
	if e := s.Ingest("wrong", 0, 1, []Observation{wrong}, 105); e != nil {
		t.Fatal(e)
	}
	if !goalOwed(goalState(t, s, 106)) {
		t.Fatal("unrelated step covered")
	}
	wrong.ID = "unstructured-stop"
	wrong.Kind = "Stop"
	wrong.DriveProof = nil
	wrong.Body = "worker is working in the background"
	if e := s.Ingest("stop", 0, 1, []Observation{wrong}, 107); e != nil {
		t.Fatal(e)
	}
	if !goalOwed(goalState(t, s, 108)) {
		t.Fatal("prose or activity became drive proof")
	}
	if _, e := s.Create("goal-two", "Concurrent interval", 108); e != nil {
		t.Fatal(e)
	}
	if e := s.Associate("goal-two", Association{ID: "overlap-one", Conversation: p.Intent.Target, Role: "execution", From: 100}, 0, 109); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DriveIntent("goal-one", p.Intent.ID, phaseRevision(t, s), 110, func(Conversation, string, string, func() bool) Delivery {
		t.Fatal("ambiguous association delivered")
		return Delivery{}
	}); e == nil {
		t.Fatal("ambiguous interval accepted")
	}
	if e := s.ScanGoals(111, 64); e != nil {
		t.Fatal(e)
	}
	n, repeated, e := s.RemindGoals(112)
	if e != nil || n != 2 || repeated {
		t.Fatal(n, repeated, e)
	}
	s = New(filepath.Dir(s.dir))
	n, _, e = s.RemindGoals(113)
	if e != nil || n != 0 {
		t.Fatal("restart duplicated reminder", n, e)
	}
	_, _, _ = s.RemindGoals(412)
	n, repeated, e = s.RemindGoals(712)
	if e != nil || n != 2 || !repeated {
		t.Fatal("HQ coordination not escalated", n, repeated, e)
	}
	if e := RequestGoalCheck(filepath.Dir(s.dir)); e != nil || GoalCheckStamp(filepath.Dir(s.dir)) == 0 {
		t.Fatal("Stop marker missing", e)
	}
}
