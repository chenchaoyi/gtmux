package hqworkspace

import "testing"

func TestContinuationEvidenceReopensDeferredResponsibility(t *testing.T) {
	for _, mode := range []string{"report", "stop", "stale", "unmatched", "paused", "covered"} {
		t.Run(mode, func(t *testing.T) {
			s := New(t.TempDir())
			m, err := s.Create("review-matter", "Verify review findings", 100)
			if err != nil {
				t.Fatal(err)
			}
			c := Conversation{Agent: "codex", HomeID: "test-home", SessionID: "reviewer"}
			if err := s.Associate(m.ID, Association{ID: "reviewer", Conversation: c, Role: "execution", From: 100}, 0, 100); err != nil {
				t.Fatal(err)
			}
			if err := s.ScanGoals(101, 64); err != nil {
				t.Fatal(err)
			}
			read := func() Database {
				t.Helper()
				d, err := s.Read()
				if err != nil {
					t.Fatal(err)
				}
				m = d.Matters[m.ID]
				return d
			}
			read()
			id := GoalFollowupID(m)
			q, err := s.ClaimObservation(id, "hq-first", 102)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "paused" || mode == "covered" {
				p := &Responsibility{StepID: "review", Action: "Resolve review findings", AuthorityRef: "user:assignment"}
				if mode == "paused" {
					p.Hold = &GoalHold{Kind: "paused", SourceRef: "user:pause", Condition: "Wait for explicit resumption"}
				} else {
					p.Hold = &GoalHold{Kind: "external", SourceRef: "ci:run-42", Condition: "Wait for the pinned run to finish", NextCheck: 900}
				}
				if _, err := s.Assess(Assessment{ID: "saved-responsibility", ObservationID: id, Owner: q.Owner, Generation: q.Generation, MatterID: m.ID, Revision: m.Revision, Decision: "responsibility", Reason: "Verified pause or external wait", Responsibility: p}, 103, false); err != nil {
					t.Fatal(err)
				}
			} else if _, err := s.Assess(Assessment{ID: "old-check", ObservationID: id, Owner: q.Owner, Generation: q.Generation, MatterID: m.ID, Revision: m.Revision, Decision: "deferred", Reason: "Await review conclusion", NextCheck: 900}, 103, false); err != nil {
				t.Fatal(err)
			}
			o := Observation{ID: "relay-review", Kind: "relay-report", MatterID: m.ID, Conversation: c, OccurredAt: 110, Continuation: true}
			switch mode {
			case "stop":
				o.Kind = "Stop"
			case "stale":
				o.OccurredAt = 102
			case "unmatched":
				o.MatterID, o.Continuation = "", false
			}
			if err := s.Ingest("fixture", 0, 1, []Observation{o}, 111); err != nil {
				t.Fatal(err)
			}
			d := read()
			remind := func(now int64, want int) {
				t.Helper()
				items, err := s.RemindContinuations(read(), now)
				if err != nil || len(items) != want {
					t.Fatalf("at %d: %v, want %d, error %v", now, items, want, err)
				}
			}
			switch mode {
			case "paused", "covered", "unmatched":
				remind(111, 0)
				if len(d.IntentAttempts) != 0 {
					t.Fatal("evidence issued an intent")
				}
				return
			case "stale":
				if d.Reconciliations[id].State != "deferred" {
					t.Fatal("old source evidence reopened a later defer")
				}
				remind(111, 0)
				return
			}
			if q := d.Reconciliations[id]; q.State != "pending" || q.NextCheck != 0 {
				t.Fatal("fresh evidence did not reopen old defer", q)
			}
			remind(111, 1)
			if err := s.Ingest("fixture", 1, 1, []Observation{o}, 112); err != nil {
				t.Fatal(err)
			}
			remind(112, 0)
			remind(140, 0)
			remind(141, 1)
			claim, err := s.ClaimObservation(o.ID, "hq-first", 142)
			if err != nil || claim.ClaimedAt != 142 {
				t.Fatal(claim, err)
			}
			remind(171, 0)
			remind(442, 1)
			successor, err := s.ClaimObservation(o.ID, "hq-next", 443)
			if err != nil || successor.Generation <= claim.Generation {
				t.Fatal(successor, err)
			}
			note := Assessment{ID: "note-review", ObservationID: o.ID, Owner: claim.Owner, Generation: claim.Generation, MatterID: m.ID, Revision: m.Revision, Decision: "noted", Reason: "The source report was read"}
			if _, err := s.Assess(note, 444, false); err == nil {
				t.Fatal("rotated owner assessed newer generation")
			}
			note.Owner, note.Generation = successor.Owner, successor.Generation
			if _, err := s.Assess(note, 444, false); err != nil {
				t.Fatal(err)
			}
			d = read()
			if g := GoalResponsibility(d, m, 445); !goalOwed(g) || d.Reconciliations[id].State != "pending" || len(d.IntentAttempts) != 0 {
				t.Fatal("noted closed the work or authorized input", g)
			}
		})
	}
}
