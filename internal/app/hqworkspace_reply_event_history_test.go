package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

// The compatibility fixtures synthesize the producer stream separately. Give
// their message-side safety probe a readable, initialized isolated journal.
func seedReplyJournal(t *testing.T) {
	t.Helper()
	if err := events.AppendChecked(events.Record{Ts: 1, Event: "Stop", Agent: "Codex", AgentSession: "other-fixture"}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceMessageChecksOriginalTargetHistory(t *testing.T) {
	for _, scenario := range []string{"pre-enrollment-pause", "same-second", "legacy-same-second", "legacy-clean", "other-target", "explicit-resume", "resume-same-second", "missing-tail", "missing-prefix", "invalid-record", "bad-watermark", "regressed-watermark", "valid-rotation", "lost-at-watermark", "no-journal", "unreadable", "history-lost-after-arm"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			source := &phase1CursorSource{}
			_, _, c := phase1CursorStore(t, source, true)
			workspaceReadEvents, workspaceEventSequence = events.ReadSince, events.CurrentSeq
			oldHQ, oldOwner := workspaceIsHQCaller, workspaceAssessmentOwner
			t.Cleanup(func() { workspaceIsHQCaller, workspaceAssessmentOwner = oldHQ, oldOwner })
			workspaceIsHQCaller = func() bool { return true }
			workspaceAssessmentOwner = func() string { return "fixture-hq" }
			must := func(e error) {
				t.Helper()
				if e != nil {
					t.Fatal(e)
				}
			}
			s := hqworkspace.New(t.TempDir())
			const matter = "message-history"
			m, e := s.Register(matter, "Target history", "Original scope", "open", hqworkspace.Origin{Kind: "task", Ref: "task:fixture"}, 95)
			must(e)
			must(s.ActivateObservations(0, 95))
			// The original target event really enters the journal. The ordinary serve
			// consumer passes it before the conversation has workspace membership.
			add := func(kind, session string, at int64) {
				must(events.AppendChecked(events.Record{Ts: at, Event: kind, Origin: events.OriginInstruction, HomeID: c.HomeID, Agent: "Codex", AgentSession: session, Summary: "Fixture target instruction"}))
			}
			if strings.HasPrefix(scenario, "legacy-") {
				other := c
				other.SessionID = "earlier-source"
				n, err := s.Register("previous", "Earlier work", "Earlier scope", "open", hqworkspace.Origin{Kind: "task", Ref: "task:earlier"}, 90)
				must(err)
				must(s.Associate(n.ID, hqworkspace.Association{ID: "earlier-worker", Role: "execution", Conversation: other, From: 90}, n.Revision, 91))
				add("Stop", other.SessionID, 96)
				must(ingestWorkspaceObservations(s, 97))
			} else if scenario != "no-journal" {
				add("Stop", c.SessionID, 96)
			}
			send := func(id string, at int64) {
				_, err := applyWorkspaceAction(s, workspaceAction{Op: "send", Matter: matter, ID: id, Body: "User authorizes the fixture"}, at)
				must(err)
			}
			if strings.HasPrefix(scenario, "legacy-") {
				_, e = s.Submit("original", matter, "Original instruction", 100)
				must(e)
			} else {
				send("original", 100)
			}
			ref := "message:original"
			r := relay.Request{ID: "inbox-history", Kind: "ask", For: "hq", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: 101, ReplyAt: 102, State: "replied", ReplyChannel: "inbox"}
			workspaceRelayRecords = func() []relay.Request { return []relay.Request{r} }
			switch scenario {
			case "no-journal":
			case "pre-enrollment-pause", "explicit-resume", "resume-same-second":
				add("UserPromptSubmit", c.SessionID, 103)
			case "same-second", "legacy-same-second":
				add("UserPromptSubmit", c.SessionID, 100)
			case "other-target":
				add("UserPromptSubmit", "another-session", 103)
			default:
				add("Stop", c.SessionID, 103)
			}
			must(ingestWorkspaceObservations(s, 104))
			if scenario == "explicit-resume" || scenario == "resume-same-second" || scenario == "lost-at-watermark" {
				when := int64(105)
				if scenario == "resume-same-second" {
					when = 103
				}
				send("resume", when)
				ref = "message:resume"
			}
			current := func() hqworkspace.Matter { return phase1CursorDatabase(t, s).Matters[matter] }
			must(s.Associate(m.ID, hqworkspace.Association{ID: "exact-member", Role: "execution", Conversation: c, From: 95}, current().Revision, 106))
			p := hqworkspace.Responsibility{StepID: "fixture-step", Action: "Run fixture", AuthorityRef: ref, Intent: &hqworkspace.ActionIntent{ID: "fixture-action", Ref: "task:fixture", Target: c, At: 106}}
			must(s.ScanGoals(107, 64))
			q, e := s.ClaimObservation(hqworkspace.GoalFollowupID(current()), "reviewer", 108)
			must(e)
			_, e = s.Assess(hqworkspace.Assessment{ID: "history-review", Owner: q.Owner, ObservationID: q.Observation.ID, Generation: q.Generation, MatterID: matter, Revision: current().Revision, Decision: "responsibility", Reason: "Verified user source", Responsibility: &p}, 109, false)
			must(e)
			removeTail := func() {
				b, err := os.ReadFile(events.Path())
				must(err)
				lines := strings.SplitAfter(string(b), "\n")
				must(os.WriteFile(events.Path(), []byte(lines[0]), 0600))
			}
			switch scenario {
			case "missing-tail":
				removeTail()
			case "missing-prefix":
				add("Stop", c.SessionID, 109)
				b, err := os.ReadFile(events.Path())
				must(err)
				lines := strings.SplitAfter(string(b), "\n")
				must(os.WriteFile(events.Path(), []byte(lines[len(lines)-2]), 0600))
			case "lost-at-watermark":
				must(os.Rename(events.Path(), filepath.Join(t.TempDir(), "removed-generation")))
			case "valid-rotation":
				b, err := os.ReadFile(events.Path())
				must(err)
				lines := strings.SplitAfter(string(b), "\n")
				must(os.WriteFile(events.Path(), []byte(lines[1]), 0600))
			case "bad-watermark", "regressed-watermark":
				value := "broken"
				if scenario == "regressed-watermark" {
					value = "1"
				}
				must(os.WriteFile(filepath.Join(filepath.Dir(events.Path()), "events.seq"), []byte(value), 0600))
			case "invalid-record":
				f, err := os.OpenFile(events.Path(), os.O_APPEND|os.O_WRONLY, 0600)
				must(err)
				_, err = f.WriteString("{invalid}\n")
				must(err)
				must(f.Close())
			case "unreadable":
				must(os.Rename(events.Path(), filepath.Join(t.TempDir(), "saved")))
				must(os.Mkdir(events.Path(), 0700))
			}
			v, e := applyWorkspaceAction(s, workspaceAction{Op: "resume-reply", Matter: matter, ID: p.Intent.ID, ReplyID: r.ID, Revision: current().Revision}, 110)
			allowed := scenario == "explicit-resume" || scenario == "resume-same-second" || scenario == "other-target" || scenario == "legacy-clean" || scenario == "valid-rotation"
			if scenario == "history-lost-after-arm" {
				must(e)
				removeTail()
			}
			blocker := ""
			if e != nil {
				blocker = e.Error()
			} else {
				a := v.(hqworkspace.IntentAttempt)
				blocker = replySourceHold(a, phase1CursorDatabase(t, s))
				if !allowed {
					oldCandidates := workspaceGoalCandidates
					t.Cleanup(func() { workspaceGoalCandidates = oldCandidates })
					workspaceGoalCandidates = func() []workspaceCandidate { t.Fatal("an ineligible target reached dispatch lookup"); return nil }
					must(pumpWorkspaceReplies(s, 111))
					d := phase1CursorDatabase(t, s)
					saved := d.IntentAttempts[p.Intent.ID]
					g := hqworkspace.GoalResponsibility(d, d.Matters[matter], 111)
					if saved.State != "armed" || blocker == "" || saved.Evidence != blocker || !strings.Contains(g.Reason, blocker) {
						t.Fatalf("lost blocker: %+v, %+v", saved, g)
					}
				}
				if blocker == "" && allowed {
					writes := 0
					for range 2 {
						_, e = s.DriveReply(a, 111, func(_ hqworkspace.Conversation, _ string, _ string, valid func() bool) hqworkspace.Delivery {
							if !valid() {
								t.Fatal("current action rejected")
							}
							writes++
							return hqworkspace.Delivery{State: "queued"}
						})
						must(e)
					}
					if writes != 1 {
						t.Fatal("writes", writes)
					}
				}
			}
			if allowed != (blocker == "") {
				t.Fatalf("allowed=%v blocker=%q", allowed, blocker)
			}
		})
	}
}
