package app

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

func TestReplyLifecycleAcrossStop(t *testing.T) {
	for _, before := range []bool{false, true} {
		name := "reply-after-stop"
		if before {
			name = "reply-before-stop"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			source := &phase1CursorSource{}
			s, home, c := phase1CursorStore(t, source, true)
			workspaceRelayRecords = relay.List
			r, _, err := relay.Put(relay.Request{ID: "target-receipt", Kind: "ask", For: "hq", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, Session: "fixture", Body: "Request coordination"})
			if err != nil {
				t.Fatal(err)
			}
			worker := relay.Source{Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID}
			if rows := relay.Inbox(worker, false, 20); len(rows) != 0 {
				t.Fatal(rows)
			}
			scan := func() {
				t.Helper()
				if e := ingestWorkspaceObservations(s, time.Now().Unix()); e != nil {
					t.Fatal(e)
				}
			}
			scan()
			reply := func() {
				t.Helper()
				if _, e := relay.Claim(r.ID); e != nil {
					t.Fatal(e)
				}
				if _, e := relay.Resolve(r.ID, "PRIVATE REPLY BODY"); e != nil {
					t.Fatal(e)
				}
				if e := relay.BeginNotice(r.ID, "synthetic-turn"); e != nil {
					t.Fatal(e)
				}
				if e := relay.FinishNotice(r.ID, true, true); e != nil {
					t.Fatal(e)
				}
				scan()
			}
			if before {
				reply()
			}
			source.latest = 1
			source.rows = []events.Record{{Seq: 1, Event: "Stop", Agent: "codex", AgentSession: c.SessionID, HomeID: c.HomeID, Ts: r.CreatedAt}}
			scan()
			s = hqworkspace.New(home)
			if !before {
				reply()
			}
			d := phase1CursorDatabase(t, s)
			for _, id := range []string{"event-1", "inbox:" + r.ID, "notice-accepted:" + r.ID} {
				q := d.Reconciliations[id]
				if q.Observation.MatterID != "cursor-goal" || !q.Observation.Conversation.Same(c) || strings.Contains(q.Observation.Body, "PRIVATE") {
					t.Fatalf("missing or incorrect attributed stage %s: %+v", id, q)
				}
			}
			if d.Reconciliations["fetched:"+r.ID].Observation.ID != "" || len(d.IntentAttempts) != 0 {
				t.Fatal("hint armed or fetched a reply")
			}
			if _, e := relay.Fetch(r.ID, worker); e != nil {
				t.Fatal(e)
			}
			scan()
			d = phase1CursorDatabase(t, s)
			g := hqworkspace.GoalResponsibility(d, d.Matters["cursor-goal"], time.Now().Unix())
			if d.Reconciliations["inbox:"+r.ID].State != "superseded" || g.ObservationID != "fetched:"+r.ID || g.State == "driving" {
				t.Fatal("fetch erased or duplicated step debt", g)
			}
			scan()
			if again := phase1CursorDatabase(t, s); again.Revision != d.Revision || len(again.IntentAttempts) != 0 {
				t.Fatal("repeated scan produced work")
			}
		})
	}
}

func TestReplyContinuationDispatchAndEvidence(t *testing.T) {
	for _, scenario := range []string{"execute", "crash", "unknown", "refused", "working", "draft", "unreadable", "unsupported", "reused-pane", "ambiguous", "stale-revision", "user-question", "user-relay", "foreign-authority", "legacy", "unknown-hint", "new-instruction", "new-target-prompt", "mid-submit-change", "already-driving"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TMUX", "")
			t.Setenv("TMUX_TMPDIR", t.TempDir())
			oldBin := tmux.Bin
			tmux.Bin = ""
			oldHQ, oldOwner, oldCandidates, oldIO := workspaceIsHQCaller, workspaceAssessmentOwner, workspaceGoalCandidates, workspaceDispatch
			t.Cleanup(func() {
				tmux.Bin = oldBin
				workspaceIsHQCaller, workspaceAssessmentOwner, workspaceGoalCandidates, workspaceDispatch = oldHQ, oldOwner, oldCandidates, oldIO
			})
			seedReplyJournal(t)
			source := &phase1CursorSource{}
			s, home, c := phase1CursorStore(t, source, true)
			workspaceRelayRecords = relay.List
			const matter = "cursor-goal"
			if scenario == "execute" {
				home = state.HQHome()
				s = workspaceStore()
				if _, e := s.Create(matter, "Synthetic goal", 100); e != nil {
					t.Fatal(e)
				}
				if e := s.Associate(matter, hqworkspace.Association{ID: "runtime-association", Conversation: c, Role: "execution", From: 100}, 0, 100); e != nil {
					t.Fatal(e)
				}
				if e := s.ActivateObservations(0, 100); e != nil {
					t.Fatal(e)
				}
			}
			const action = "run-fixture"
			const step = "verify-output"
			r, _, e := relay.Put(relay.Request{ID: "fetch-me", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, Session: "fixture", For: "hq", Kind: "ask", Body: "private request prose"})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = relay.Claim(r.ID); e != nil {
				t.Fatal(e)
			}
			if _, e = relay.Resolve(r.ID, "private reply prose"); e != nil {
				t.Fatal(e)
			}
			if scenario == "legacy" {
				if e = relay.BeginReplyAttempt(r.ID); e != nil {
					t.Fatal(e)
				}
			}
			clock := func() int64 { return time.Now().Unix() }
			scan := func() {
				t.Helper()
				if e := ingestWorkspaceObservations(s, clock()); e != nil {
					t.Fatal(e)
				}
			}
			scan()
			m := func() hqworkspace.Matter { return phase1CursorDatabase(t, s).Matters[matter] }
			if _, e = s.WithInstructionCursor(events.CurrentSeq(), events.RecordingPolicy()).Submit("source-scope", matter, "original user instruction", 101); e != nil {
				t.Fatal(e)
			}
			if e = s.ScanGoals(clock(), 64); e != nil {
				t.Fatal(e)
			}
			q, e := s.ClaimObservation(hqworkspace.GoalFollowupID(m()), "reviewer", clock())
			if e != nil {
				t.Fatal(e)
			}
			authority := "message:source-scope"
			if scenario == "foreign-authority" {
				authority = "relay:" + r.ID
			}
			p := hqworkspace.Responsibility{StepID: step, AuthorityRef: authority, Action: "private authorized instruction", Intent: &hqworkspace.ActionIntent{ID: action, Ref: "task:fixture", Target: c, At: 105}}
			_, e = s.Assess(hqworkspace.Assessment{ID: "save-reviewed-intent", MatterID: matter, ObservationID: q.Observation.ID, Owner: q.Owner, Generation: q.Generation, Revision: m().Revision, Decision: "responsibility", Responsibility: &p, Reason: "reviewed synthetic source instruction"}, clock(), false)
			if e != nil {
				t.Fatal(e)
			}
			arm := workspaceAction{Op: "resume-reply", ID: action, Matter: matter, Revision: m().Revision, ReplyID: r.ID}
			workspaceIsHQCaller = func() bool { return false }
			workspaceAssessmentOwner = func() string { return "reviewer" }
			if _, e = applyWorkspaceAction(s, arm, clock()); e == nil {
				t.Fatal("non-HQ armed a continuation")
			}
			workspaceIsHQCaller = func() bool { return true }
			_, e = applyWorkspaceAction(s, arm, clock())
			if scenario == "legacy" || scenario == "foreign-authority" {
				if e == nil || len(phase1CursorDatabase(t, s).IntentAttempts) != 0 {
					t.Fatal("ineligible continuation", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = applyWorkspaceAction(s, arm, clock()); e != nil {
				t.Fatal(e)
			}
			s = hqworkspace.New(home)
			visible := workspaceCandidate{Conversation: c, Status: "idle"}
			visible.Pane = "%123"
			visible.Loc = "fixture:0.0"
			frame := "› \n\n  ? for shortcuts"
			workers := []workspaceCandidate{visible}
			switch scenario {
			case "working":
				workers[0].Status = "working"
			case "draft":
				frame = "› human draft\n\n  ? for shortcuts"
			case "unreadable":
				frame = ""
			case "ambiguous":
				workers = append(workers, visible)
			case "reused-pane":
				workers[0].SessionID = "different-thread"
			case "stale-revision":
				e = s.Update(matter, "new revision", "", "", m().Revision, clock())
			case "user-question":
				e = s.RequestInput(matter, hqworkspace.UserInput{ID: "choose-now", Kind: "decision", Prompt: "Choose", Why: "Private decision", Impact: "fixture choice"}, m().Revision, clock())
			case "user-relay":
				_, _, e = relay.Put(relay.Request{ID: "ask-user", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, Session: "fixture", For: "user", Blocking: true, Kind: "ask", Body: "decision needed"})
			case "unknown-hint":
				e = relay.BeginNotice(r.ID, "original-turn")
			case "new-target-prompt":
				source.latest = 1
				source.rows = []events.Record{{Seq: 1, Ts: clock(), Event: "UserPromptSubmit", Origin: events.OriginInstruction, AgentSession: c.SessionID, HomeID: c.HomeID}}
			case "new-instruction":
				_, e = s.WithInstructionCursor(events.CurrentSeq(), events.RecordingPolicy()).Submit("new-scope", matter, "changed instruction", clock())
			}
			if e != nil {
				t.Fatal(e)
			}
			workspaceGoalCandidates = func() []workspaceCandidate { return workers }
			writes := 0
			proof := func(id, state, evidence string) {
				t.Helper()
				body, _ := json.Marshal(map[string]any{"schema": "gtmux.goal-drive/v1", "matter": matter, "drive": hqworkspace.DriveProof{StepID: step, ActionID: action, Ref: p.Intent.Ref, State: state, NextCheck: clock() + 300, Evidence: evidence}})
				if _, _, err := relay.Put(relay.Request{ID: id, Kind: "report", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, Session: "fixture", For: "hq", Body: string(body)}); err != nil {
					t.Fatal(err)
				}
				scan()
			}
			if scenario == "already-driving" {
				proof("existing-receipt", "native", "job:existing")
			}
			workspaceDispatch = func(string, ...string) dispatch.IO {
				io := dispatch.IO{Capture: func() string { return frame }, Now: clock, Sleep: func() {}, Paste: func(string) error { t.Fatal("terminal fallback"); return nil }, Enter: func() error { t.Fatal("terminal fallback"); return nil }}
				if scenario == "unsupported" {
					return io
				}
				io.SubmitNative = func(o dispatch.Opts, body string, hold func() string) dispatch.Result {
					if scenario == "mid-submit-change" {
						workers[0].SessionID = "replacement"
						if hold() == "" {
							t.Fatal("final check missed target change")
						}
						return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: "identity-changed"}
					}
					if !o.IdleWakeInput || o.DirectUserInput || o.Force || o.ClobberDraft || o.ExpectedSessionID != c.SessionID || o.ExpectedHomeID != c.HomeID || o.DeliveryID != action || hold() != "" {
						t.Fatal("wrong native policy", o)
					}
					if !strings.Contains(body, "gtmux relay show "+r.ID) || !strings.Contains(body, step) || !strings.Contains(body, matter) || strings.Contains(body, "private") || strings.Contains(body, "original user instruction") {
						t.Fatal("unsafe reminder", body)
					}
					if scenario == "refused" {
						return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: "unsupported-rpc"}
					}
					writes++
					switch scenario {
					case "crash":
						panic("simulated process loss after native write")
					case "unknown":
						return dispatch.Result{State: dispatch.StateFailed, Evidence: "lost response"}
					}
					return dispatch.Result{State: dispatch.StateQueued, Evidence: "accepted-reminder"}
				}
				return io
			}
			if scenario == "execute" {
				oldNext, oldStamp, oldWakeAt, oldWake := workspaceReconciliationNext, workspaceGoalCheckStamp, workspaceReconciliationWakeAt, workspaceReconcileWake
				t.Cleanup(func() {
					workspaceReconciliationNext, workspaceGoalCheckStamp, workspaceReconciliationWakeAt, workspaceReconcileWake = oldNext, oldStamp, oldWakeAt, oldWake
				})
				workspaceReconcileWake = func(string) {}
			}
			pump := func() {
				t.Helper()
				if scenario == "execute" {
					workspaceReconciliationNext = 0
					scheduleWorkspaceReconciliation()
					deadline := time.Now().Add(5 * time.Second)
					for workspaceReconciliationRunner.running.Load() && time.Now().Before(deadline) {
						time.Sleep(time.Millisecond)
					}
					if workspaceReconciliationRunner.running.Load() {
						t.Fatal("serve reconciliation did not settle")
					}
					return
				}
				if e := pumpWorkspaceReplies(s, clock()); e != nil {
					t.Fatal(e)
				}
			}
			if scenario == "crash" {
				func() {
					defer func() {
						if recover() != "simulated process loss after native write" {
							t.Fatal("crash boundary not reached")
						}
					}()
					pump()
				}()
			} else {
				pump()
			}
			s = hqworkspace.New(home)
			pump()
			if _, e = applyWorkspaceAction(s, workspaceAction{Op: "drive", ID: action, Matter: matter, Revision: m().Revision}, clock()); e != nil {
				t.Fatal(e)
			}
			d := phase1CursorDatabase(t, s)
			a := d.IntentAttempts[action]
			g := hqworkspace.GoalResponsibility(d, d.Matters[matter], clock())
			wantWrites := 0
			switch scenario {
			case "execute", "unknown", "crash":
				wantWrites = 1
			}
			if writes != wantWrites {
				t.Fatal("native writes", writes, wantWrites, a)
			}
			switch scenario {
			case "already-driving":
				if g.State != "driving" {
					t.Fatal(g)
				}
			case "crash", "unknown":
				if !strings.Contains(g.Reason, "reconcile") || (a.State != "submitting" && a.State != "uncertain") {
					t.Fatal(g, a)
				}
			case "refused", "mid-submit-change":
				if a.State != "held" || a.Evidence == "" {
					t.Fatal(a)
				}
			case "user-question":
				if g.State != "waiting-user" {
					t.Fatal(g)
				}
			case "execute":
				if a.State != "queued" || g.State != "owes-hq" {
					t.Fatal("transport masquerades as execution", g, a)
				}
				worker := relay.Source{Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID}
				if _, e = relay.Fetch(r.ID, worker); e != nil {
					t.Fatal(e)
				}
				scan()
				proof("transport-only", "received", "received")
				proof("empty-progress", "progress", "")
				d = phase1CursorDatabase(t, s)
				g = hqworkspace.GoalResponsibility(d, d.Matters[matter], clock())
				if g.State != "owes-hq" || !strings.Contains(g.Reason, "fetched") {
					t.Fatal("fetch/input became work", g)
				}
				output := filepath.Join(t.TempDir(), "result")
				if e = os.WriteFile(output, []byte("verified fixture output"), 0600); e != nil {
					t.Fatal(e)
				}
				b, e := os.ReadFile(output)
				if e != nil {
					t.Fatal(e)
				}
				proof("actual-progress", "progress", fmt.Sprintf("result sha256:%x", sha256.Sum256(b)))
				d = phase1CursorDatabase(t, s)
				g = hqworkspace.GoalResponsibility(d, d.Matters[matter], clock())
				if g.State != "driving" || g.ObservationID != "relay-actual-progress" {
					t.Fatal("source execution evidence lost", g)
				}
			default:
				if a.State != "armed" || a.Evidence == "" || !strings.Contains(g.Reason, a.Evidence) {
					t.Fatal("missing concrete block", g, a)
				}
				if scenario == "working" || scenario == "draft" {
					workers[0].Status = "idle"
					frame = "› \n\n  ? for shortcuts"
					pump()
					pump()
					if writes != 1 {
						t.Fatal("ready target failed to resume once", writes)
					}
				}
			}
		})
	}
}
