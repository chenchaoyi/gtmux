package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

func TestReplyReconcilesSavedHistory(t *testing.T) {
	for _, scenario := range []string{"later-answer", "unreviewed-answer", "old-answer", "legacy-event", "late-membership", "late-received", "late-execution", "late-user-report", "late-source-unavailable", "ambiguous-history", "intermediate-overlap", "wrong-source", "ordinary-queued", "ordinary-landed", "ordinary-uncertain"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			source := &phase1CursorSource{}
			_, _, c := phase1CursorStore(t, source, true)
			home := t.TempDir()
			s := hqworkspace.New(home)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(s.ActivateObservations(0, 90))
			const matter = "existing-task"
			r := relay.Request{ID: "saved-reply", Kind: "ask", For: "hq", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: 106, ReplyAt: 107, State: "replied", ReplyChannel: "inbox"}
			rows := []relay.Request{r}
			priorReport := scenario == "late-received" || scenario == "late-execution" || scenario == "late-user-report" || scenario == "late-source-unavailable"
			if priorReport {
				state, owner := "received", "hq"
				if scenario == "late-execution" {
					state = "progress"
				}
				if scenario == "late-user-report" {
					owner = "user"
				}
				proof, err := json.Marshal(map[string]any{"schema": "gtmux.goal-drive/v1", "matter": matter, "drive": hqworkspace.DriveProof{StepID: "saved-step", Ref: "task:fixture", ActionID: "saved-action", State: state, Evidence: "fixture work result", NextCheck: 200}})
				must(err)
				rows = append(rows, relay.Request{ID: "prior-work", Kind: "report", For: owner, Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: 108, Body: string(proof)})
			}
			workspaceRelayRecords = func() []relay.Request { return rows }
			late := strings.HasPrefix(scenario, "late-") || scenario == "wrong-source" || scenario == "ambiguous-history" || scenario == "intermediate-overlap"
			registeredAt := int64(100)
			if scenario == "legacy-event" || late {
				source.rows = []events.Record{{Seq: 1, Ts: 102, Event: "UserPromptSubmit", Origin: events.OriginInstruction, Agent: "Codex", HomeID: c.HomeID, AgentSession: c.SessionID, Summary: "Original scope", Pane: "%test"}}
				source.latest = 1
			}
			if late {
				registeredAt = 109
				must(ingestWorkspaceObservations(s, 108))
			}
			m, e := s.Register(matter, "Existing task", "Original scope", "open", hqworkspace.Origin{Kind: "task", Ref: "task:fixture"}, registeredAt)
			must(e)
			participant := hqworkspace.Association{ID: "original-worker", Conversation: c, Role: "execution", From: 99}
			must(s.Associate(m.ID, participant, m.Revision, 109))
			current := func() hqworkspace.Matter { return phase1CursorDatabase(t, s).Matters[matter] }
			authority := "message:original-user"
			if scenario == "legacy-event" || late {
				authority = "user:instruction"
			} else {
				_, e = s.Submit("original-user", matter, "Complete the fixture", 101)
				must(e)
			}
			must(ingestWorkspaceObservations(s, 110))
			if late && phase1CursorDatabase(t, s).Reconciliations["relay-"+r.ID].Observation.MatterID != "" {
				t.Fatal("fixture lost original unassigned observation")
			}
			const action = "saved-action"
			p := hqworkspace.Responsibility{StepID: "saved-step", Action: "Execute original fixture", AuthorityRef: authority, Intent: &hqworkspace.ActionIntent{ID: action, Ref: "task:fixture", Target: c, At: 105}}
			review := func(id string, at int64) {
				must(s.ScanGoals(at, 64))
				q, e := s.ClaimObservation(hqworkspace.GoalFollowupID(current()), "reviewer", at)
				must(e)
				_, e = s.Assess(hqworkspace.Assessment{ID: id, Owner: q.Owner, ObservationID: q.Observation.ID, Generation: q.Generation, MatterID: matter, Revision: current().Revision, Decision: "responsibility", Reason: "Verified current scope for saved action", Responsibility: &p}, at, false)
				must(e)
			}
			review("first-review", 111)
			in := map[string]any{"op": "resume-reply", "matter": matter, "id": action, "reply_id": r.ID}
			original := 0
			if strings.HasPrefix(scenario, "ordinary-") {
				state := strings.TrimPrefix(scenario, "ordinary-")
				_, e := s.DriveIntent(matter, action, current().Revision, 112, func(_ hqworkspace.Conversation, _ string, _ string, valid func() bool) hqworkspace.Delivery {
					if !valid() {
						t.Fatal("stale original drive")
					}
					original++
					return hqworkspace.Delivery{State: state}
				})
				must(e)
			}
			if strings.Contains(scenario, "answer") || scenario == "ordinary-uncertain" {
				question := hqworkspace.UserInput{ID: "decision", Kind: "decision", Prompt: "Proceed with the original task?", Why: "User confirmation", Impact: "Execute the fixture"}
				must(s.RequestInput(matter, question, current().Revision, 113))
				_, e = s.SubmitReply("confirmed", matter, "Continue the same action", question.ID, 114)
				must(e)
				must(s.ClearInput(matter, question.ID, "confirmed-choice", "Verified same step", current().Revision, 115))
				if scenario != "unreviewed-answer" {
					review("after-answer", 116)
				}
				if scenario != "old-answer" {
					in["authority_ref"] = "message:confirmed"
				}
			}
			if scenario == "late-source-unavailable" {
				rows = rows[:1]
			}
			if scenario == "legacy-event" || late {
				in["verified_user_event"] = "events:1"
				in["authority_ref"] = "events:1"
			}
			if scenario == "ambiguous-history" || scenario == "intermediate-overlap" {
				other, e := s.Register("other-task", "Other task", "Original scope", "open", hqworkspace.Origin{Kind: "task", Ref: "task:other"}, 116)
				must(e)
				interval := participant
				interval.Until = 108
				if scenario == "intermediate-overlap" {
					interval.From = 108
				}
				must(s.Associate(other.ID, interval, other.Revision, 116))
			}
			if scenario == "wrong-source" {
				must(s.Move(matter, "", participant.ID, current().Revision, 116))
				other := c
				other.SessionID = "replacement"
				participant.ID = "replacement-worker"
				participant.Conversation = other
				must(s.Associate(matter, participant, current().Revision, 117))
			}
			oldHQ, oldOwner := workspaceIsHQCaller, workspaceAssessmentOwner
			t.Cleanup(func() { workspaceIsHQCaller, workspaceAssessmentOwner = oldHQ, oldOwner })
			workspaceIsHQCaller = func() bool { return true }
			workspaceAssessmentOwner = func() string { return "fixture-hq" }
			in["revision"] = current().Revision
			b, e := json.Marshal(in)
			must(e)
			var arm workspaceAction
			must(json.Unmarshal(b, &arm))
			value, e := applyWorkspaceAction(s, arm, 118)
			switch scenario {
			case "old-answer", "unreviewed-answer", "ambiguous-history", "intermediate-overlap", "wrong-source":
				if e == nil {
					t.Fatal("unsafe arm", value)
				}
				return
			}
			must(e)
			a := value.(hqworkspace.IntentAttempt)
			if a.Intent.ID != action || current().Responsibility.AuthorityRef != authority || current().Responsibility.Intent.At != 105 {
				t.Fatal("original plan rewritten", a)
			}
			s = hqworkspace.New(home)
			d := phase1CursorDatabase(t, s)
			if late {
				if d.Reconciliations["relay-"+r.ID].Observation.MatterID != "" || d.Reconciliations["inbox:"+r.ID].Observation.MatterID != "" || hqworkspace.ReplyObservations(d)[r.ID].MatterID != matter {
					t.Fatal("attribution missing or raw evidence rewritten")
				}
			}
			if scenario == "late-source-unavailable" {
				if why := replySourceHold(a, d); !strings.Contains(why, "historical source report") {
					t.Fatal("missing provenance must be reconciled", why)
				}
				return
			}
			if original > 0 {
				g := hqworkspace.GoalResponsibility(d, current(), 119)
				if strings.Contains(g.Reason, "reminder submitted") || !strings.Contains(g.Reason, "original") {
					t.Fatal("prior drive misrepresented", g)
				}
			}
			calls := 0
			for range 2 {
				_, e = s.DriveReply(a, 119, func(target hqworkspace.Conversation, _ string, _ string, valid func() bool) hqworkspace.Delivery {
					if !valid() || !target.Same(c) {
						t.Fatal("unverified delivery")
					}
					calls++
					return hqworkspace.Delivery{State: "queued"}
				})
				must(e)
			}
			if scenario == "late-execution" || scenario == "late-received" {
				g := hqworkspace.GoalResponsibility(d, current(), 119)
				if calls != 0 || a.State != "external" || (g.State == "driving") != (scenario == "late-execution") || d.Reconciliations["relay-prior-work"].Observation.MatterID != "" {
					t.Fatal("late proof was lost or replayed", a, g, calls)
				}
				return
			}
			if original+calls != 1 {
				t.Fatal("authoritative step delivery count", original, calls)
			}
		})
	}
}
