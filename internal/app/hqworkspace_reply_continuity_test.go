package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

func persistReplyEvents(t *testing.T, rows ...events.Record) {
	t.Helper()
	for _, r := range rows {
		r.Seq = 0
		if err := events.AppendChecked(r); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReplyHistoryUsesCommittedOrderAndActiveJournal(t *testing.T) {
	for _, scenario := range []string{"legacy-crossing-cycle", "legacy-earlier-commit", "direct-active", "disabled-before-arm", "disabled-after-arm", "disabled-final-gate", "off-on-pause", "off-on-explicit-resume", "missing-direct-tail", "malformed-policy", "disabled-empty-store"} {
		t.Run(scenario, func(t *testing.T) { exerciseReplyCoverage(t, scenario) })
	}
}

func exerciseReplyCoverage(t *testing.T, scenario string) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMUX", "")
	originalTime := workspaceReplyTime
	t.Cleanup(func() { workspaceReplyTime = originalTime })
	delayed := strings.HasPrefix(scenario, "final-no-cursor") || strings.HasPrefix(scenario, "final-delayed") || scenario == "final-same-second"
	workspaceReplyTime = func() int64 {
		if delayed {
			return 118
		}
		return 114
	}
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	setCap := func(value string) {
		must(os.MkdirAll(filepath.Dir(usercfg.Path()), 0700))
		must(os.WriteFile(usercfg.Path(), []byte(`{"eventsCapMB":`+value+`}`), 0600))
	}
	if scenario == "disabled-empty-store" {
		setCap("0")
		events.Append(events.Record{Event: "UserPromptSubmit", Origin: events.OriginInstruction, Summary: "Private input"})
		if events.CurrentSeq() != 0 {
			t.Fatal("disabled telemetry created a counter")
		}
		for _, p := range []string{events.Path(), filepath.Join(filepath.Dir(events.Path()), "events.seq")} {
			if _, e := os.Stat(p); !os.IsNotExist(e) {
				t.Fatal(p, e)
			}
		}
		return
	}
	_, _, c := phase1CursorStore(t, &phase1CursorSource{}, true)
	workspaceReadEvents, workspaceEventSequence = events.ReadSince, events.CurrentSeq
	oldHQ, oldOwner, candidates, io := workspaceIsHQCaller, workspaceAssessmentOwner, workspaceGoalCandidates, workspaceDispatch
	t.Cleanup(func() {
		workspaceIsHQCaller, workspaceAssessmentOwner, workspaceGoalCandidates, workspaceDispatch = oldHQ, oldOwner, candidates, io
	})
	workspaceIsHQCaller = func() bool { return true }
	workspaceAssessmentOwner = func() string { return "reviewer" }
	home := t.TempDir()
	s := hqworkspace.New(home)
	const matter = "original-matter"
	must(s.ActivateObservations(0, 90))
	m, e := s.Register(matter, "Existing work", "Original fixture", "open", hqworkspace.Origin{Kind: "task", Ref: "task:fixture"}, 95)
	must(e)
	other := c
	other.SessionID = "other-session"
	otherMatter, e := s.Register("other", "Other work", "Other scope", "open", hqworkspace.Origin{Kind: "task", Ref: "task:other"}, 95)
	must(e)
	must(s.Associate(otherMatter.ID, hqworkspace.Association{ID: "different-task", Conversation: other, Role: "execution", From: 92}, otherMatter.Revision, 95))
	event := func(at int64, kind string, who hqworkspace.Conversation, body string) events.Record {
		return events.Record{Ts: at, Event: kind, Origin: events.OriginInstruction, Agent: "Codex", HomeID: who.HomeID, AgentSession: who.SessionID, Pane: "%test", Summary: body}
	}
	legacy := strings.HasPrefix(scenario, "legacy-")
	cold := strings.HasPrefix(scenario, "cold-")
	enabledFault := strings.HasPrefix(scenario, "enabled-counter")
	final := strings.HasPrefix(scenario, "final-")
	readOnly := strings.HasPrefix(scenario, "counter-read-only") || enabledFault
	if readOnly && scenario != "counter-read-only-remove-policy" {
		setCap("20")
	} else if readOnly {
		must(os.MkdirAll(filepath.Dir(usercfg.Path()), 0700))
	}
	if cold {
		switch scenario {
		case "cold-zero":
			_, e = s.WithInstructionCursor(0, events.RecordingPolicy()).Submit("legacy-user", matter, "Original assignment", 100)
		case "cold-legacy":
			_, e = s.Submit("legacy-user", matter, "Original assignment", 100)
		default:
			_, e = applyWorkspaceAction(s, workspaceAction{Op: "send", ID: "legacy-user", Matter: matter, Body: "Original assignment"}, 100)
		}
		must(e)
		setCap("0")
		events.Append(event(101, "UserPromptSubmit", c, "Private pause"))
		setCap("20")
		must(events.AppendChecked(event(102, "Stop", other, "unrelated")))
	} else if legacy {
		if scenario == "legacy-earlier-commit" {
			must(events.AppendChecked(event(96, "Stop", other, "earlier")))
			must(ingestWorkspaceObservations(s, 96))
		}
		_, e = s.Submit("legacy-user", matter, "Original assignment", 100)
		must(e)
		if scenario == "legacy-crossing-cycle" {
			must(events.AppendChecked(event(101, "UserPromptSubmit", c, "Pause")))
		}
		must(events.AppendChecked(event(102, "Stop", other, "unrelated")))
		must(ingestWorkspaceObservations(s, 99)) // cycle started before M, but committed after M
	} else if scenario == "counter-read-only-message" || scenario == "enabled-counter-message" {
		must(events.AppendChecked(event(96, "Stop", other, "initialized")))
		_, e = applyWorkspaceAction(s, workspaceAction{Op: "send", ID: "legacy-user", Matter: matter, Body: "Original assignment"}, 100)
		must(e)
	} else {
		must(events.AppendChecked(event(100, "UserPromptSubmit", c, "Run the original step")))
	}
	current := func() hqworkspace.Matter { return phase1CursorDatabase(t, s).Matters[matter] }
	must(s.Associate(m.ID, hqworkspace.Association{ID: "original-worker", Conversation: c, Role: "execution", From: 95}, current().Revision, 103))
	r := relay.Request{ID: "original-reply", Kind: "ask", For: "hq", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: 104, ReplyAt: 105, State: "replied", ReplyChannel: "inbox"}
	rows := []relay.Request{r}
	workspaceRelayRecords = func() []relay.Request { return rows }
	must(ingestWorkspaceObservations(s, 106))
	ref := "events:1"
	messageAuthority := legacy || cold || (scenario == "counter-read-only-message" || scenario == "enabled-counter-message")
	if messageAuthority {
		ref = "message:legacy-user"
	}
	p := hqworkspace.Responsibility{StepID: "original-step", AuthorityRef: ref, Action: "Run the fixture", Intent: &hqworkspace.ActionIntent{ID: "original-action", Ref: "task:fixture", Target: c, At: 104}}
	review := func(id string, at int64) {
		must(s.ScanGoals(at, 64))
		q, e := s.ClaimObservation(hqworkspace.GoalFollowupID(current()), "reviewer", at)
		must(e)
		_, e = s.Assess(hqworkspace.Assessment{ID: id, Owner: q.Owner, ObservationID: q.Observation.ID, Generation: q.Generation, MatterID: matter, Revision: current().Revision, Decision: "responsibility", Reason: "Confirmed saved scope", Responsibility: &p}, at, false)
		must(e)
	}
	review("saved-before-pause", 107)
	if scenario == "legacy-earlier-commit" {
		b, err := os.ReadFile(events.Path())
		must(err)
		lines := strings.SplitAfter(string(b), "\n")
		must(os.WriteFile(events.Path(), []byte(strings.Join(lines[1:], "")), 0600))
	}
	a := workspaceAction{Op: "resume-reply", ID: p.Intent.ID, ReplyID: r.ID, Matter: matter, Revision: current().Revision}
	if !messageAuthority {
		a.VerifiedUserEvent = ref
	}
	blind := func() {
		before, err := os.ReadFile(events.Path())
		must(err)
		counter := filepath.Join(filepath.Dir(events.Path()), "events.seq")
		restoredLink := false
		if readOnly {
			must(os.Chmod(counter, 0400))
			f, e := os.OpenFile(counter, os.O_RDWR, 0)
			if e == nil {
				f.Close()
				// Privileged test users can write mode 0400; the managed writer also rejects links.
				must(os.Rename(counter, counter+".saved"))
				must(os.Symlink(counter+".saved", counter))
				restoredLink = true
			}
		}
		fallback := strings.Contains(scenario, "fallback")
		coverage := filepath.Join(state.StatusDir(), "event-coverage")
		if fallback {
			must(os.Rename(coverage, coverage+".saved"))
			must(os.Symlink(coverage+".saved", coverage))
		}
		if !enabledFault {
			setCap("0")
		}
		events.Append(event(108, "UserPromptSubmit", c, "Private pause while recording is disabled"))
		if readOnly {
			if events.CurrentSeq() != 1 {
				t.Fatal("failed counter write changed watermark")
			}
			if err := events.AppendChecked(event(108, "UserPromptSubmit", c, "Private pause")); err == nil || (!enabledFault && !strings.Contains(err.Error(), "cannot advance")) {
				t.Fatal("failed counter did not surface its write failure", err)
			}
			if fallback {
				must(os.Rename(coverage+".saved", coverage))
				b, e := os.ReadFile(filepath.Join(state.HQHome(), "event-coverage-fault"))
				must(e)
				if len(b) > 64 || strings.Contains(string(b), "Private") {
					t.Fatal("fault witness contains payload", string(b))
				}
			}
			if restoredLink {
				must(os.Rename(counter+".saved", counter))
			}
			must(os.Chmod(counter, 0600))
		}
		after, err := os.ReadFile(events.Path())
		must(err)
		if string(before) != string(after) {
			t.Fatal("disabled journal wrote content")
		}
	}
	if scenario == "malformed-policy" {
		setCap("invalid")
	}
	if scenario == "disabled-before-arm" {
		blind()
	}
	if scenario == "missing-direct-tail" {
		must(events.AppendChecked(event(108, "UserPromptSubmit", c, "Pause")))
		b, e := os.ReadFile(events.Path())
		must(e)
		must(os.WriteFile(events.Path(), []byte(strings.SplitAfter(string(b), "\n")[0]), 0600))
	}
	recoverCounter := func() {
		blind()
		if scenario == "counter-read-only-remove-policy" {
			must(os.Remove(usercfg.Path()))
		} else if !enabledFault {
			setCap("20")
		}
		must(events.AppendChecked(event(109, "Stop", other, "unrelated")))
		s = hqworkspace.New(home)
	}
	if readOnly && scenario != "counter-read-only-after-arm" && scenario != "counter-read-only-final" && scenario != "enabled-counter-final" {
		recoverCounter()
	}
	if strings.HasPrefix(scenario, "off-on-") {
		blind()
		setCap("20")
	}
	resume := scenario == "off-on-explicit-resume" || scenario == "counter-read-only-resume" || scenario == "cold-message-resume" || (enabledFault && strings.HasSuffix(scenario, "-resume"))
	if resume {
		if v, err := applyWorkspaceAction(s, a, 110); err == nil && v.(hqworkspace.IntentAttempt).Evidence == "" {
			t.Fatal("original authority revived before fresh confirmation")
		}
		must(events.AppendChecked(event(110, "UserPromptSubmit", c, "Resume same saved action")))
		must(ingestWorkspaceObservations(s, 111))
		review("reverified-original-action", 112)
		a.AuthorityRef = "events:" + strconv.FormatInt(events.CurrentSeq(), 10)
		a.VerifiedUserEvent = a.AuthorityRef
		a.Revision = current().Revision
	}
	_, e = applyWorkspaceAction(s, a, 113)
	allowed := scenario == "legacy-earlier-commit" || scenario == "direct-active" || resume
	if e != nil {
		if allowed {
			t.Fatal(e)
		}
		return
	}
	if scenario == "counter-read-only-after-arm" {
		recoverCounter()
	}
	if scenario == "disabled-after-arm" {
		blind()
	}
	visible := workspaceCandidate{Conversation: c, Status: "idle"}
	visible.Pane = "%test"
	visible.Loc = "fixture:0.0"
	resolutions := 0
	workspaceGoalCandidates = func() []workspaceCandidate {
		resolutions++
		if scenario == "final-before-dispatch" && resolutions == 2 {
			must(events.AppendChecked(event(114, "UserPromptSubmit", c, "Pause at preflight")))
		}
		return []workspaceCandidate{visible}
	}
	counterPath := filepath.Join(state.Dir(), "events.seq")
	hiddenCounter := false
	writes, callbacks := 0, 0
	workspaceDispatch = func(string, ...string) dispatch.IO {
		return dispatch.IO{Capture: func() string { return "› \n\n  ? for shortcuts" }, Now: func() int64 { return 114 }, Sleep: func() {}, SubmitNative: func(o dispatch.Opts, body string, hold func() string) dispatch.Result {
			callbacks++
			if final && scenario != "final-before-dispatch" && callbacks == 1 {
				switch scenario {
				case "final-no-cursor-earlier", "final-no-cursor-later", "final-no-cursor-message":
					must(events.AppendChecked(event(116, "UserPromptSubmit", c, "Before the actual refusal")))
					must(os.Rename(counterPath, counterPath+".saved"))
					must(os.Mkdir(counterPath, 0700))
					hiddenCounter = true
					if why := hold(); why != "" {
						return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: why}
					}
					t.Fatal("unreadable sequence passed native admission")
				case "final-after-claim":
					if hold() != "" {
						t.Fatal("fixture failed initial native claim")
					}
					writes++
					must(events.AppendChecked(event(114, "UserPromptSubmit", c, "Pause after possible submit")))
					if why := hold(); why != "" {
						return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: why}
					}
					t.Fatal("later native hold missed new instruction")
				case "final-unknown":
					writes++
					return dispatch.Result{State: dispatch.StateFailed, Evidence: "native receipt unavailable"}
				case "final-accepted":
				case "final-same-authority":
					visible.Status = "working"
					why := hold()
					visible.Status = "idle"
					if why == "" {
						t.Fatal("working target passed the last hold")
					}
					return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: why}
				case "final-unproven":
					return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: "prior native claim"}
				default:
					must(events.AppendChecked(event(114, "UserPromptSubmit", c, "Pause before submission")))
					if why := hold(); why != "" {
						return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: why}
					}
					t.Fatal("last gate allowed a paused target")
				}
			}
			if scenario == "counter-read-only-final" || scenario == "enabled-counter-final" {
				recoverCounter()
			}
			if scenario == "disabled-final-gate" {
				blind()
			}
			if why := hold(); why != "" {
				return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: why}
			}
			if !o.IdleWakeInput || o.DeliveryID != p.Intent.ID || !strings.Contains(body, r.ID) {
				t.Fatal("wrong delivery", o)
			}
			writes++
			return dispatch.Result{State: dispatch.StateQueued}
		}}
	}
	must(pumpWorkspaceReplies(s, 114))
	if hiddenCounter {
		must(os.Remove(counterPath))
		must(os.Rename(counterPath+".saved", counterPath))
	}
	must(pumpWorkspaceReplies(s, 115))
	proofAt := int64(116)
	if final {
		s = hqworkspace.New(home)
		d := phase1CursorDatabase(t, s)
		old := d.IntentAttempts[p.Intent.ID]
		refused := scenario != "final-unknown" && scenario != "final-after-claim" && scenario != "final-accepted" && scenario != "final-unproven"
		initialCalls := 1
		if scenario == "final-before-dispatch" {
			initialCalls = 0
		}
		if callbacks != initialCalls || (refused && (old.State != "held" || writes != 0)) {
			t.Fatal("initial transport", old, callbacks, writes)
		}
		originalRef := a.VerifiedUserEvent
		switch scenario {
		case "final-message", "final-no-cursor-message":
			_, e := applyWorkspaceAction(s, workspaceAction{Op: "send", ID: "confirmed-after-refusal", Matter: matter, Body: "Continue the original fixture"}, 120)
			must(e)
			a.AuthorityRef, a.VerifiedUserEvent = "message:confirmed-after-refusal", ""
		case "final-same-authority":
			a.AuthorityRef = originalRef
		case "final-older-event", "final-no-cursor-earlier":
			a.AuthorityRef, a.VerifiedUserEvent = "events:2", "events:2"
		default:
			sourceTime := int64(116)
			switch scenario {
			case "final-no-cursor-later", "final-delayed-fresh":
				sourceTime = 119
			case "final-delayed-source":
				sourceTime = 117
			case "final-same-second":
				sourceTime = 118
			}
			must(events.AppendChecked(event(sourceTime, "UserPromptSubmit", c, "Fresh explicit confirmation")))
			a.AuthorityRef = "events:" + strconv.FormatInt(events.CurrentSeq(), 10)
			a.VerifiedUserEvent = a.AuthorityRef
		}
		must(ingestWorkspaceObservations(s, 117))
		review("review-after-final-refusal", 122)
		a.Revision = current().Revision
		_, armErr := applyWorkspaceAction(s, a, 123)
		resume = scenario == "final-no-cursor-message" || scenario == "final-delayed-fresh" || scenario == "final-event" || scenario == "final-message" || scenario == "final-before-dispatch"
		if (scenario == "final-same-authority" || scenario == "final-older-event" || delayed && !resume) && (armErr == nil || !strings.Contains(armErr.Error(), "after the recorded refusal")) {
			t.Fatal("missing fresh authorization interlock", armErr)
		}
		if resume {
			must(armErr)
		}
		must(pumpWorkspaceReplies(s, 124))
		must(pumpWorkspaceReplies(s, 125))
		d = phase1CursorDatabase(t, s)
		if refused {
			history := false
			for _, ch := range d.Changes {
				if ch.Kind == "reply-refused-before-submit" && ch.Target == p.Intent.ID && ch.Detail != "" {
					history = true
				}
			}
			if !history {
				t.Fatal("zero-submission receipt lost", d.IntentAttempts[p.Intent.ID])
			}
		}
		if !resume {
			if callbacks != 1 {
				t.Fatal("previous or unknown action replayed", callbacks, writes)
			}
			return
		}
		if callbacks != initialCalls+1 {
			t.Fatal("same action did not resume", callbacks, writes, d.IntentAttempts[p.Intent.ID])
		}
		allowed = true
		proofAt = 126
	}
	if allowed != (writes == 1) || writes > 1 {
		t.Fatalf("allowed=%v native writes=%d", allowed, writes)
	}
	if !allowed {
		return
	}
	if resume {
		rows[0].FetchedAt = proofAt
		must(ingestWorkspaceObservations(s, proofAt))
		result := filepath.Join(t.TempDir(), "result")
		must(os.WriteFile(result, []byte("original fixture executed"), 0600))
		b, e := os.ReadFile(result)
		must(e)
		proof, e := json.Marshal(map[string]any{"schema": "gtmux.goal-drive/v1", "matter": matter, "drive": hqworkspace.DriveProof{State: "progress", ActionID: p.Intent.ID, StepID: p.StepID, Ref: p.Intent.Ref, Evidence: string(b), NextCheck: 200}})
		must(e)
		rows = append(rows, relay.Request{ID: "work", Kind: "report", For: "hq", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: proofAt + 1, Body: string(proof)})
		must(ingestWorkspaceObservations(s, proofAt+1))
		d := phase1CursorDatabase(t, s)
		g := hqworkspace.GoalResponsibility(d, current(), proofAt+2)
		if g.State != "driving" || g.ObservationID != "relay-work" {
			t.Fatal(g)
		}
	}
}
