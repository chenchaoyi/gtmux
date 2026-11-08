package hqworkspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func phaseStore(t *testing.T) *Store {
	t.Helper()
	home := t.TempDir()
	s := New(home)
	if _, err := s.Create("goal-one", "Synthetic goal", 100); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "workspace", "records.json")
	if s.dir != filepath.Join(home, "workspace") {
		t.Fatal("Store escaped fixture directory")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	t.Log("isolated records:", path)
	return s
}
func phaseRead(t *testing.T, s *Store) Database {
	t.Helper()
	d, e := s.Read()
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func phaseRevision(t *testing.T, s *Store) uint64 {
	return phaseRead(t, s).Matters["goal-one"].Revision
}
func phaseQuestion(id string) UserInput {
	return UserInput{ID: id, Kind: "decision", Prompt: "Keep fixture?", Why: "Choose retention.", Impact: "Synthetic fixture only."}
}

func TestPhase1ReportsPreserveCompletedAndQuestion(t *testing.T) {
	s := phaseStore(t)
	if e := s.Update("goal-one", "", "completed", "Verified completion", phaseRevision(t, s), 101); e != nil {
		t.Fatal(e)
	}
	before := phaseRead(t, s).Matters["goal-one"]
	for i := 0; i < 100; i++ {
		m, e := s.Publish("late-report", "goal-one", "", "progress", "Old work ongoing", "open", 102)
		if e != nil || m.Projection != "pending-reconciliation" {
			t.Fatal(m, e)
		}
	}
	d := phaseRead(t, s)
	if !reflect.DeepEqual(d.Matters["goal-one"], before) || len(d.Reconciliations) != 1 {
		t.Fatal("late report changed lifecycle or duplicated debt", d)
	}
	if _, e := s.Publish("late-report", "goal-one", "", "progress", "Different", "open", 103); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if e := s.Update("goal-one", "", "open", "New explicit requirement", before.Revision, 104); e != nil {
		t.Fatal(e)
	}
	if e := s.RequestInput("goal-one", phaseQuestion("question-one"), phaseRevision(t, s), 105); e != nil {
		t.Fatal(e)
	}
	before = phaseRead(t, s).Matters["goal-one"]
	if _, e := s.Publish("result-one", "goal-one", "", "result", "CI green", "completed", 106); e != nil {
		t.Fatal(e)
	}
	if e := s.Update("goal-one", "", "open", "Progress", before.Revision, 107); e == nil {
		t.Fatal("state update hid question")
	}
	if got := phaseRead(t, s).Matters["goal-one"]; !reflect.DeepEqual(got, before) {
		t.Fatal(got)
	}
}

func TestPhase1TransitionCASAndQuestionIdentity(t *testing.T) {
	s := phaseStore(t)
	rev := phaseRevision(t, s)
	if e := s.Update("goal-one", "", "completed", "done", 0, 101); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if e := s.Update("other-goal", "", "completed", "done", rev, 101); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, st := range []string{"completed", "closed"} {
		wg.Add(1)
		go func(state string) {
			defer wg.Done()
			results <- s.Update("goal-one", "", state, "Explicit outcome", rev, 102)
		}(st)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else if !errors.Is(e, ErrConflict) {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatal("CAS accepted multiple writers", success)
	}
	if e := s.Update("goal-one", "", "open", "Explicit renewed request", phaseRevision(t, s), 103); e != nil {
		t.Fatal(e)
	}
	if e := s.RequestInput("goal-one", phaseQuestion("question-one"), phaseRevision(t, s), 104); e != nil {
		t.Fatal(e)
	}
	old := phaseRead(t, s).Matters["goal-one"]
	if _, e := s.SubmitReplyVersion("answer-wrong", "goal-one", "yes", "question-one", old.UserInput.Revision+1, 105); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if _, e := s.SubmitReply("answer-one", "goal-one", "yes", "question-one", 105); e != nil {
		t.Fatal("legacy immutable-ID reply", e)
	}
	if e := s.ClearInput("goal-one", "question-one", "clear-old", "Verified answer", old.Revision, 106); !errors.Is(e, ErrConflict) {
		t.Fatal("answer did not advance matter revision", e)
	}
	clearRevision := phaseRevision(t, s)
	if e := s.ClearInput("goal-one", "question-one", "clear-one", "Verified answer", clearRevision, 106); e != nil {
		t.Fatal(e)
	}
	if e := s.RequestInput("goal-one", phaseQuestion("question-two"), phaseRevision(t, s), 107); e != nil {
		t.Fatal(e)
	}
	current := phaseRead(t, s).Matters["goal-one"]
	if e := s.ClearInput("goal-one", "question-one", "clear-two", "Old answer", current.Revision, 108); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if _, e := s.SubmitReply("answer-late", "goal-one", "yes", "question-one", 108); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if e := s.ClearInput("goal-one", "question-one", "clear-one", "Verified answer", clearRevision, 108); e != nil {
		t.Fatal("receipt retry failed", e)
	}
	if !reflect.DeepEqual(phaseRead(t, s).Matters["goal-one"], current) {
		t.Fatal("old answer removed new question")
	}
}

func TestPhase1RecoveryFencingAtomicAssessment(t *testing.T) {
	s := phaseStore(t)
	rev := phaseRevision(t, s)
	o := Observation{ID: "event-one", Kind: "result", MatterID: "goal-one", MatterRevision: rev, Body: "Verified output", Ref: "synthetic:1", OccurredAt: 101}
	if e := s.Ingest("synthetic", 0, 1, []Observation{o}, 102); e != nil {
		t.Fatal(e)
	}
	if e := s.Ingest("synthetic", 1, 1, []Observation{o}, 103); e != nil {
		t.Fatal(e)
	}
	altered := o
	altered.Body = "different"
	if e := s.Ingest("synthetic", 1, 2, []Observation{altered}, 103); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if phaseRead(t, s).ObservationCursors["synthetic"] != 1 {
		t.Fatal("failed input advanced cursor")
	}
	q, e := s.ClaimObservation(o.ID, "hq-old", 104)
	if e != nil {
		t.Fatal(e)
	}
	s = New(filepath.Dir(s.dir))
	if len(ReconciliationQueue(phaseRead(t, s), 105)) != 1 {
		t.Fatal("restart lost debt")
	}
	if _, e = s.ClaimObservation(o.ID, "hq-new", 105); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	newer, e := s.ClaimObservation(o.ID, "hq-new", 405)
	if e != nil {
		t.Fatal(e)
	}
	a := Assessment{ID: "assess-one", ObservationID: o.ID, Owner: "hq-old", Generation: q.Generation, MatterID: "goal-one", Revision: rev, Decision: "transition", State: "completed", Reason: "Verified current output"}
	if _, e = s.Assess(a, 406, false); !errors.Is(e, ErrConflict) {
		t.Fatal("expired holder committed", e)
	}
	a.Owner = "hq-new"
	a.Generation = newer.Generation
	before := phaseRead(t, s)
	preview, e := s.Assess(a, 406, true)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, phaseRead(t, s)) {
		t.Fatal("dry run wrote records")
	}
	receipt, e := s.Assess(a, 406, false)
	if e != nil || !reflect.DeepEqual(preview, receipt) {
		t.Fatal(receipt, e)
	}
	s = New(filepath.Dir(s.dir))
	d := phaseRead(t, s)
	if d.Matters["goal-one"].State != "completed" || d.Reconciliations[o.ID].ReceiptID != a.ID || receipt.AfterRevision != d.Matters["goal-one"].Revision || len(d.Messages) != 0 {
		t.Fatal("projection/receipt/side-effect mismatch", d)
	}
	replay, e := s.Assess(a, 900, false)
	if e != nil || !reflect.DeepEqual(replay, receipt) {
		t.Fatal(replay, e)
	}
	a.Reason = "Changed"
	if _, e = s.Assess(a, 901, false); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(d, phaseRead(t, s)) {
		t.Fatal("duplicate assessment mutated state")
	}
}

func TestPhase1OldObservationDeferralAndSchemaMigration(t *testing.T) {
	s := phaseStore(t)
	rev := phaseRevision(t, s)
	o := Observation{ID: "old-result", Kind: "result", MatterID: "goal-one", MatterRevision: rev}
	if e := s.Ingest("source", 0, 1, []Observation{o}, 101); e != nil {
		t.Fatal(e)
	}
	if e := s.Update("goal-one", "", "needs-input", "New requirement", rev, 102); e != nil {
		t.Fatal(e)
	}
	q, e := s.ClaimObservation(o.ID, "hq-one", 103)
	if e != nil {
		t.Fatal(e)
	}
	a := Assessment{ID: "assessment-one", ObservationID: o.ID, Owner: q.Owner, Generation: q.Generation, MatterID: "goal-one", Revision: phaseRevision(t, s), Decision: "transition", State: "completed", Reason: "Old result"}
	if _, e = s.Assess(a, 104, false); !errors.Is(e, ErrConflict) {
		t.Fatal("old evidence closed newer target", e)
	}
	a.Decision = "deferred"
	a.State = ""
	a.NextCheck = 110
	if _, e = s.Assess(a, 104, false); e != nil {
		t.Fatal(e)
	}
	if len(ReconciliationQueue(phaseRead(t, s), 109)) != 0 || len(ReconciliationQueue(phaseRead(t, s), 110)) != 1 {
		t.Fatal("deferred debt lost schedule")
	}
	d := Empty()
	d.Schema = 1
	d.Reconciliations = nil
	d.Assessments = nil
	d.ObservationCursors = nil
	d.Matters["legacy-goal"] = Matter{ID: "legacy-goal", State: "completed", Revision: 1}
	d.Revision = 1
	b, _ := json.Marshal(d)
	home := t.TempDir()
	if e := os.Mkdir(filepath.Join(home, "workspace"), 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(home, "workspace", "records.json")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	s = New(home)
	migrated := phaseRead(t, s)
	if migrated.Schema != 2 || len(migrated.Reconciliations) != 0 || migrated.Matters["legacy-goal"].State != "completed" {
		t.Fatal(migrated)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != string(b) {
		t.Fatal("read mutated legacy data")
	}
	if e := s.ActivateObservations(55, 200); e != nil {
		t.Fatal(e)
	}
	raw, _ = os.ReadFile(path)
	if e := json.Unmarshal(raw, &d); e != nil || d.Schema != 2 || d.ObservationCursors["events"] != 55 || d.ObservationCursors["relay-start"] != 200 {
		t.Fatal("migration lost activation", e)
	}
}

func TestPhase1ExactAttributionAndAmbiguity(t *testing.T) {
	s := phaseStore(t)
	a := participant("assoc-one", "session-one", "execution", 100, 150)
	if e := s.Associate("goal-one", a, 0, 151); e != nil {
		t.Fatal(e)
	}
	d := phaseRead(t, s)
	if id, _ := MatchObservation(d, a.Conversation, 120); id != "goal-one" {
		t.Fatal(id)
	}
	wrong := a.Conversation
	wrong.HomeID = "other-home"
	if id, _ := MatchObservation(d, wrong, 120); id != "" {
		t.Fatal("cross-home match", id)
	}
	if _, e := s.Create("goal-two", "Next goal", 151); e != nil {
		t.Fatal(e)
	}
	a.ID = "assoc-two"
	a.From = 150
	a.Until = 0
	if e := s.Associate("goal-two", a, 0, 151); e != nil {
		t.Fatal(e)
	}
	d = phaseRead(t, s)
	if id, _ := MatchObservation(d, a.Conversation, 150); id != "" {
		t.Fatal("same-second ambiguity guessed", id)
	}
	if id, _ := MatchObservation(d, a.Conversation, 151); id != "goal-two" {
		t.Fatal(id)
	}
}
