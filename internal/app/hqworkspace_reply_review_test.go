package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestReviewedReplyAuthorityAndOrder(t *testing.T) {
	for _, scenario := range []string{"direct", "direct-superseded", "late-send-audit", "explicit-disposition", "direct-unattested", "wrong-source", "machine-event", "cleared-answer", "unresolved-answer", "noted-fetch", "earlier-received", "earlier-empty-progress", "consumed-pause", "coordination-echo"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			seedReplyJournal(t)
			source := &phase1CursorSource{}
			_, _, c := phase1CursorStore(t, source, true)
			home := t.TempDir()
			s := hqworkspace.New(home)
			const matter = "registered-goal"
			m, err := s.Register(matter, "Fixture", "Verified existing assignment", "open", hqworkspace.Origin{Kind: "task", Ref: "task:fixture"}, 100)
			if err != nil {
				t.Fatal(err)
			}
			must := func(e error) {
				t.Helper()
				if e != nil {
					t.Fatal(e)
				}
			}
			must(s.Associate(m.ID, hqworkspace.Association{ID: "worker-fixture", Conversation: c, Role: "execution", From: 100}, m.Revision, 100))
			must(s.ActivateObservations(0, 100))
			current := func() hqworkspace.Matter { return phase1CursorDatabase(t, s).Matters[matter] }
			r := relay.Request{ID: "step-receipt", Kind: "ask", For: "hq", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: 105, ReplyAt: 106, State: "replied", ReplyChannel: "inbox", Body: "fixture request", Reply: "fixture reply"}
			rows := []relay.Request{r}
			workspaceRelayRecords = func() []relay.Request { return rows }
			hq, owner := workspaceIsHQCaller, workspaceAssessmentOwner
			t.Cleanup(func() { workspaceIsHQCaller, workspaceAssessmentOwner = hq, owner })
			workspaceIsHQCaller = func() bool { return true }
			workspaceAssessmentOwner = func() string { return "fixture-hq" }
			direct := strings.HasPrefix(scenario, "direct") || scenario == "late-send-audit" || scenario == "wrong-source" || scenario == "machine-event"
			ref := "message:user-scope"
			if direct {
				ref = "events:1"
				row := events.Record{Seq: 1, Event: "UserPromptSubmit", Origin: events.OriginInstruction, Agent: "Codex", HomeID: c.HomeID, AgentSession: c.SessionID, Pane: "%9", Summary: "existing fixture scope", Ts: 101}
				if scenario == "wrong-source" {
					row.AgentSession = "other"
				}
				source.rows = []events.Record{row}
				source.latest = 1
				if scenario == "direct-superseded" {
					source.rows = append(source.rows, events.Record{Seq: 2, Event: "UserPromptSubmit", Origin: events.OriginInstruction, Agent: "Codex", HomeID: c.HomeID, AgentSession: c.SessionID, Ts: 104})
					source.latest = 2
				}
				if scenario == "machine-event" {
					source.rows = append(source.rows, events.Record{Seq: 2, Event: events.AuditEventSend, Pane: "%9", Actor: "hq", Summary: "landed: existing fixture scope", Ts: 102})
					source.latest = 2
				}
			} else {
				_, err = s.Submit("user-scope", matter, "Original user assignment", 101)
				must(err)
			}
			if scenario == "cleared-answer" || scenario == "unresolved-answer" {
				q := hqworkspace.UserInput{ID: "user-choice", Kind: "decision", Prompt: "Run fixture?", Why: "User decision", Impact: "Fixture output"}
				must(s.RequestInput(matter, q, current().Revision, 102))
				_, err = s.SubmitReply("chosen-answer", matter, "Proceed with the verified fixture", q.ID, 103)
				must(err)
				ref = "message:chosen-answer"
				if scenario == "cleared-answer" {
					must(s.ClearInput(matter, q.ID, "clear-receipt", "Verified answer", current().Revision, 104))
				}
			}
			must(ingestWorkspaceObservations(s, 107))
			const action = "original-action"
			p := hqworkspace.Responsibility{StepID: "original-step", Action: "Run fixture", AuthorityRef: ref, Intent: &hqworkspace.ActionIntent{ID: action, Ref: "task:fixture", Target: c, At: 105}}
			must(s.ScanGoals(108, 64))
			claim, err := s.ClaimObservation(hqworkspace.GoalFollowupID(current()), "reviewer", 109)
			must(err)
			_, err = s.Assess(hqworkspace.Assessment{ID: "original-review", Owner: claim.Owner, ObservationID: claim.Observation.ID, Generation: claim.Generation, MatterID: matter, Revision: current().Revision, Decision: "responsibility", Reason: "Reviewed original user source and step", Responsibility: &p}, 110, false)
			must(err)
			if scenario == "consumed-pause" {
				source.latest = 1
				source.rows = []events.Record{{Seq: 1, Ts: 111, Event: "UserPromptSubmit", Origin: events.OriginInstruction, Agent: "Codex", AgentSession: c.SessionID, HomeID: c.HomeID, Summary: "Pause"}}
				must(ingestWorkspaceObservations(s, 112))
			}
			if scenario == "noted-fetch" || scenario == "explicit-disposition" || strings.HasPrefix(scenario, "earlier-") {
				rows[0].FetchedAt = 110
				if strings.HasPrefix(scenario, "earlier-") {
					state := "received"
					if scenario == "earlier-empty-progress" {
						state = "progress"
					}
					body, _ := json.Marshal(map[string]any{"schema": "gtmux.goal-drive/v1", "matter": matter, "drive": hqworkspace.DriveProof{State: state, StepID: p.StepID, ActionID: action, Ref: p.Intent.Ref, NextCheck: 200}})
					rows = append(rows, relay.Request{ID: "prior-report", For: "hq", Kind: "report", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: 109, Body: string(body)})
				}
				must(ingestWorkspaceObservations(s, 111))
				f, err := s.ClaimObservation("fetched:"+r.ID, "reviewer", 112)
				must(err)
				_, err = s.Assess(hqworkspace.Assessment{ID: "ordinary-note", Owner: f.Owner, ObservationID: f.Observation.ID, Generation: f.Generation, MatterID: matter, Revision: current().Revision, Decision: "noted", Reason: "Read fetch receipt"}, 112, false)
				must(err)
				s = hqworkspace.New(home)
				d := phase1CursorDatabase(t, s)
				g := hqworkspace.GoalResponsibility(d, current(), 113)
				if g.ObservationID != "fetched:"+r.ID || g.State == "driving" {
					t.Errorf("acknowledged fetch or transport hid unapplied step: %+v", g)
				}
			}
			if scenario == "explicit-disposition" {
				other := r
				other.ID = "another-receipt"
				rows = append(rows, other)
				must(ingestWorkspaceObservations(s, 113))
				read := func() map[string]hqworkspace.Observation {
					return hqworkspace.ReplyObservations(phase1CursorDatabase(t, s))
				}
				if len(read()) != 2 || read()[r.ID].Kind != "relay-fetched" {
					t.Fatal(read())
				}
				f, err := s.ClaimObservation("fetched:"+r.ID, "reviewer", 113)
				must(err)
				_, err = s.Assess(hqworkspace.Assessment{ID: "close-receipt", Owner: f.Owner, ObservationID: f.Observation.ID, Generation: f.Generation, MatterID: matter, Revision: current().Revision, Decision: "no-followup", Reason: "Reviewed source; this receipt requires no action"}, 113, false)
				must(err)
				if len(read()) != 1 || read()[other.ID].Kind != "relay-inbox" {
					t.Fatal(read())
				}
			}
			var arm workspaceAction
			input := map[string]any{"op": "resume-reply", "id": action, "matter": matter, "revision": current().Revision, "reply_id": r.ID}
			if direct && scenario != "direct-unattested" {
				input["verified_user_event"] = ref
			}
			b, _ := json.Marshal(input)
			must(json.Unmarshal(b, &arm))
			value, e := applyWorkspaceAction(s, arm, 113)
			switch scenario {
			case "consumed-pause", "direct-superseded", "explicit-disposition", "wrong-source", "machine-event", "unresolved-answer", "direct-unattested":
				if e == nil {
					t.Fatal("unsafe reservation", value)
				}
				return
			}
			if e != nil {
				t.Fatal("verified existing step became unreachable", e)
			}
			a := value.(hqworkspace.IntentAttempt)
			if scenario == "late-send-audit" {
				source.rows = append(source.rows, events.Record{Seq: 2, Event: events.AuditEventSend, Pane: "%9", Actor: "hq", Summary: "landed: existing fixture scope", Ts: 114})
				source.latest = 2
				if why := replySourceHold(a, phase1CursorDatabase(t, s)); why == "" {
					t.Fatal("newly attributed machine input authorized a send")
				}
				return
			}
			calls := 0
			_, e = s.DriveReply(a, 114, func(target hqworkspace.Conversation, body, id string, valid func() bool) hqworkspace.Delivery {
				calls++
				if !target.Same(c) || !valid() {
					t.Fatal("fresh exact target refused")
				}
				if scenario == "coordination-echo" {
					if _, kind := transcript.ClassifyUserPrompt(body); kind != transcript.PromptDrop {
						t.Error("reminder classified as user instruction")
					}
					if text, kind := transcript.ClassifyUserPrompt(body + "\nPlease pause the fixture."); kind != transcript.PromptUser || text != "Please pause the fixture." {
						t.Error("real user line lost", text, kind)
					}
				}
				return hqworkspace.Delivery{State: "queued"}
			})
			must(e)
			want := 1
			if strings.HasPrefix(scenario, "earlier-") {
				want = 0
			}
			if strings.HasPrefix(scenario, "earlier-") {
				body, err := json.Marshal(map[string]any{"schema": "gtmux.goal-drive/v1", "matter": matter, "drive": hqworkspace.DriveProof{State: "progress", StepID: p.StepID, ActionID: action, Ref: p.Intent.Ref, NextCheck: 200, Evidence: "synthetic checked fixture result"}})
				must(err)
				proof := rows[1]
				proof.ID, proof.Body = "work-result", string(body)
				rows = append(rows, proof)
				must(ingestWorkspaceObservations(s, 115))
				g := hqworkspace.GoalResponsibility(phase1CursorDatabase(t, s), current(), 115)
				if g.State != "driving" || g.ObservationID != "relay-work-result" {
					t.Fatal(g)
				}
			}
			if calls != want {
				t.Fatalf("action writes=%d want=%d; %+v", calls, want, a)
			}
		})
	}
}
