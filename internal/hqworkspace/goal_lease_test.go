package hqworkspace

import (
	"errors"
	"fmt"
	"testing"
)

func leasedGoalFixture(t *testing.T, queued bool) (*Store, Reconciliation, Assessment) {
	t.Helper()
	s, p := goalFixture(t)
	if !queued {
		p.Intent = nil
		p.Hold = &GoalHold{Kind: "paused", SourceRef: "user:pause", Condition: "await explicit user resume"}
	}
	goalPlan(t, s, p, 103, "")
	if queued {
		if _, e := s.DriveIntent("goal-one", p.Intent.ID, phaseRevision(t, s), 104, func(Conversation, string, string, func() bool) Delivery { return Delivery{State: "queued"} }); e != nil {
			t.Fatal(e)
		}
	}
	d := phaseRead(t, s)
	m := d.Matters["goal-one"]
	q, e := s.ClaimObservation(goalKey(m), "hq-current", 150)
	if e != nil {
		t.Fatal(e)
	}
	return s, q, Assessment{ID: "lease-assessment", ObservationID: q.Observation.ID, Owner: q.Owner, Generation: q.Generation, MatterID: m.ID, Revision: m.Revision, Decision: "responsibility", Reason: "Verified the same current responsibility", Responsibility: m.Responsibility}
}

func TestCoveredGoalScanKeepsValidUnchangedLease(t *testing.T) {
	for _, queued := range []bool{false, true} {
		t.Run(fmt.Sprint(queued), func(t *testing.T) {
			s, q, a := leasedGoalFixture(t, queued)
			if e := s.ScanGoals(151, 64); e != nil {
				t.Fatal(e)
			}
			if _, e := s.Assess(a, 152, false); e != nil {
				t.Fatal("unchanged scan revoked the current valid assessment lease", e)
			}
			if got := phaseRead(t, s).Assessments[a.ID].Assessment.Generation; got != q.Generation {
				t.Fatal("generation drift", got, q.Generation)
			}
		})
	}
}

func TestCoveredGoalActualPremiseChangeAndExpiryFenceOldClaim(t *testing.T) {
	s, _, a := leasedGoalFixture(t, true)
	if e := s.ScanGoals(405, 64); e != nil {
		t.Fatal(e)
	} // queued proof expired at404; lease still valid until450.
	if _, e := s.Assess(a, 406, false); !errors.Is(e, ErrConflict) {
		t.Fatal("actual coverage change did not fence the old claim", e)
	}
	s, q, a := leasedGoalFixture(t, false)
	if _, e := s.Assess(a, 450, false); !errors.Is(e, ErrConflict) {
		t.Fatal("expired lease committed", e)
	}
	fresh, e := s.ClaimObservation(q.Observation.ID, "hq-successor", 451)
	if e != nil || fresh.Generation <= q.Generation {
		t.Fatal("successor did not fence", fresh, e)
	}
	if _, e = s.Assess(a, 452, false); !errors.Is(e, ErrConflict) {
		t.Fatal("old owner/generation committed", e)
	}
}
