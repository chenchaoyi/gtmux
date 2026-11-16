package hqworkspace

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestDelayedFirstDriverReceiptNeverPermitsReplay(t *testing.T) {
	for _, kind := range []string{"native", "background"} {
		for _, now := range []int64{110, 111} {
			for _, ended := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/ingest-%d/interval-ended-%t", kind, now, ended), func(t *testing.T) {
					s, p := goalFixture(t)
					goalPlan(t, s, p, 103, "")
					if ended {
						if e := s.Interval("goal-one", "worker-one", 100, 105, phaseRevision(t, s), 106); e != nil {
							t.Fatal(e)
						}
					}
					o := Observation{ID: "late-proof", Kind: "relay-report", MatterID: "goal-one", Ref: "relay:late-proof", Conversation: p.Intent.Target, OccurredAt: 104, DriveProof: &DriveProof{StepID: p.StepID, ActionID: p.Intent.ID, Ref: p.Intent.Ref, State: kind, NextCheck: 110}}
					if e := s.Ingest("late", 0, 1, []Observation{o}, now); e != nil {
						t.Fatal(e)
					}
					if !goalOwed(goalState(t, s, now)) {
						t.Fatal("historical receipt falsely provided live coverage")
					}
					calls := 0
					send := func(Conversation, string, string, func() bool) Delivery { calls++; return Delivery{State: "landed"} }
					for n := int64(0); n < 2; n++ {
						if n == 1 {
							s = New(filepath.Dir(s.dir))
						}
						_, e := s.DriveIntent("goal-one", p.Intent.ID, phaseRevision(t, s), now+1+n, send)
						if calls != 0 {
							t.Fatalf("first ingestion at/after deadline replayed historical %s work: sends=%d", kind, calls)
						}
						if e != nil {
							t.Fatal(e)
						}
					}
					a := phaseRead(t, s).IntentAttempts[p.Intent.ID]
					if a.State != "external" || a.Evidence != o.ID || a.At != o.OccurredAt {
						t.Fatal("historical driver fact was not preserved", a)
					}
					if e := s.MarkRestored(now + 3); e != nil {
						t.Fatal(e)
					}
					a, e := s.DriveIntent("goal-one", p.Intent.ID, phaseRevision(t, s), now+4, send)
					if e != nil || calls != 0 || a.State != "uncertain" || !goalOwed(goalState(t, s, now+4)) {
						t.Fatal("restore lost late receipt fence or verification duty", a, calls, e)
					}
				})
			}
		}
	}
}

func TestDelayedDriverHistoryRequiresExactHistoricalAttribution(t *testing.T) {
	for _, bad := range []string{"sid", "home", "agent", "step", "action", "ref", "matter", "future", "before-intent", "deadline", "state", "ambiguous", "outside-interval"} {
		t.Run(bad, func(t *testing.T) {
			s, p := goalFixture(t)
			goalPlan(t, s, p, 103, "")
			o := Observation{ID: "bad-proof", Kind: "relay-report", MatterID: "goal-one", Ref: "relay:bad-proof", Conversation: p.Intent.Target, OccurredAt: 104, DriveProof: &DriveProof{StepID: p.StepID, ActionID: p.Intent.ID, Ref: p.Intent.Ref, State: "native", NextCheck: 110}}
			switch bad {
			case "sid":
				o.Conversation.SessionID = "reused-pane-session"
			case "home":
				o.Conversation.HomeID = "other-home"
			case "agent":
				o.Conversation.Agent = "claude"
			case "step":
				o.DriveProof.StepID = "other-step"
			case "action":
				o.DriveProof.ActionID = "other-action"
			case "ref":
				o.DriveProof.Ref = "other-work"
			case "matter":
				o.MatterID = "other-matter"
			case "future":
				o.OccurredAt = 112
				o.DriveProof.NextCheck = 120
			case "before-intent":
				o.OccurredAt = 101
			case "deadline":
				o.DriveProof.NextCheck = o.OccurredAt
			case "state":
				o.DriveProof.State = "idle"
			case "ambiguous":
				if _, e := s.Create("other-goal", "Other", 103); e != nil {
					t.Fatal(e)
				}
				if e := s.Associate("other-goal", Association{ID: "overlap", Conversation: p.Intent.Target, Role: "execution", From: 100}, 0, 103); e != nil {
					t.Fatal(e)
				}
			case "outside-interval":
				if e := s.Interval("goal-one", "worker-one", 100, 103, phaseRevision(t, s), 105); e != nil {
					t.Fatal(e)
				}
			}
			if e := s.Ingest("late", 0, 1, []Observation{o}, 111); e != nil {
				t.Fatal(e)
			}
			d := phaseRead(t, s)
			if len(d.IntentAttempts) != 0 || d.Matters["goal-one"].Responsibility.Drive != nil {
				t.Fatal("unattributable report established driver history", d.IntentAttempts)
			}
		})
	}
}
