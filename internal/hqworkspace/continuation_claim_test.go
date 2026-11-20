package hqworkspace

import "testing"

func TestContinuationClaimCoversOnlyItsExactObservation(t *testing.T) {
	home := t.TempDir()
	s := New(home)
	m, err := s.Create("review-matter", "Verify review findings", 100)
	if err != nil {
		t.Fatal(err)
	}
	ingest := func(cursor, at int64, rows ...Observation) {
		t.Helper()
		for i := range rows {
			rows[i].MatterID = m.ID
		}
		if err := s.Ingest("fixture", cursor, cursor+1, rows, at); err != nil {
			t.Fatal(err)
		}
	}
	claim := func(id, owner string, at int64) Reconciliation {
		t.Helper()
		q, err := s.ClaimObservation(id, owner, at)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	assertQueue := func(at int64, ids ...string) {
		t.Helper()
		db, err := s.Read()
		if err != nil {
			t.Fatal(err)
		}
		items := ContinuationWakeQueue(db, at)
		if len(items) != len(ids) {
			t.Fatalf("at %d, got %v, want %v", at, items, ids)
		}
		for i, id := range ids {
			if items[i].Observation.ID != id {
				t.Fatalf("at %d, got %v, want %v", at, items, ids)
			}
		}
	}
	ingest(0, 109, Observation{ID: "old-inbox", Kind: "relay-inbox"},
		Observation{ID: "old-note", Kind: "UserPromptSubmit"})
	claim("old-inbox", "hq-one", 109)
	claim("old-note", "hq-one", 110)
	ingest(1, 110, Observation{ID: "new-stop", Kind: "Stop", Continuation: true, OccurredAt: 110})
	assertQueue(110, "new-stop")
	old := claim("new-stop", "hq-one", 110)
	ingest(2, 110, Observation{ID: "new-report", Kind: "relay-report", Continuation: true, OccurredAt: 110})
	assertQueue(110, "new-report")
	// Reverse the wall-clock tie for the second claim. Claiming a different
	// exact observation after the new report was stored still doesn't cover it.
	ingest(3, 110, Observation{ID: "old-report", Kind: "relay-report"})
	claim("old-report", "hq-one", 110)
	assertQueue(110, "new-report")
	claim("new-report", "hq-one", 110)
	assertQueue(409)
	s = New(home)
	assertQueue(410, "new-report", "new-stop")
	next := claim("new-stop", "hq-two", 411)
	if next.Generation <= old.Generation || next.Owner == old.Owner {
		t.Fatal("expired observation lease did not pass to a successor", old, next)
	}
	_, err = s.Assess(Assessment{
		ID: "late-assessment", ObservationID: "new-stop", Owner: old.Owner, Generation: old.Generation,
		MatterID: m.ID, Revision: m.Revision, Decision: "noted", Reason: "Old owner",
	}, 412, false)
	if err == nil {
		t.Fatal("superseded HQ lease assessed exact observation")
	}
	assertQueue(412, "new-report")
}
