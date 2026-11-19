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
	readOnly := strings.HasPrefix(scenario, "counter-read-only")
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
	} else if scenario == "counter-read-only-message" {
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
	messageAuthority := legacy || cold || scenario == "counter-read-only-message"
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
		setCap("0")
		events.Append(event(108, "UserPromptSubmit", c, "Private pause while recording is disabled"))
		if readOnly {
			if events.CurrentSeq() != 1 {
				t.Fatal("failed counter write changed watermark")
			}
			if err := events.AppendChecked(event(108, "UserPromptSubmit", c, "Private pause")); err == nil || !strings.Contains(err.Error(), "cannot advance") {
				t.Fatal("failed counter did not surface its write failure", err)
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
		} else {
			setCap("20")
		}
		must(events.AppendChecked(event(109, "Stop", other, "unrelated")))
		s = hqworkspace.New(home)
	}
	if readOnly && scenario != "counter-read-only-after-arm" && scenario != "counter-read-only-final" {
		recoverCounter()
	}
	if strings.HasPrefix(scenario, "off-on-") {
		blind()
		setCap("20")
	}
	resume := scenario == "off-on-explicit-resume" || scenario == "counter-read-only-resume" || scenario == "cold-message-resume"
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
	workspaceGoalCandidates = func() []workspaceCandidate { return []workspaceCandidate{visible} }
	writes := 0
	workspaceDispatch = func(string, ...string) dispatch.IO {
		return dispatch.IO{Capture: func() string { return "› \n\n  ? for shortcuts" }, Now: func() int64 { return 114 }, Sleep: func() {}, SubmitNative: func(o dispatch.Opts, body string, hold func() string) dispatch.Result {
			if scenario == "counter-read-only-final" {
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
	must(pumpWorkspaceReplies(s, 115))
	if allowed != (writes == 1) || writes > 1 {
		t.Fatalf("allowed=%v native writes=%d", allowed, writes)
	}
	if !allowed {
		return
	}
	if resume {
		rows[0].FetchedAt = 116
		must(ingestWorkspaceObservations(s, 116))
		result := filepath.Join(t.TempDir(), "result")
		must(os.WriteFile(result, []byte("original fixture executed"), 0600))
		b, e := os.ReadFile(result)
		must(e)
		proof, e := json.Marshal(map[string]any{"schema": "gtmux.goal-drive/v1", "matter": matter, "drive": hqworkspace.DriveProof{State: "progress", ActionID: p.Intent.ID, StepID: p.StepID, Ref: p.Intent.Ref, Evidence: string(b), NextCheck: 200}})
		must(e)
		rows = append(rows, relay.Request{ID: "work", Kind: "report", For: "hq", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: 117, Body: string(proof)})
		must(ingestWorkspaceObservations(s, 117))
		d := phase1CursorDatabase(t, s)
		g := hqworkspace.GoalResponsibility(d, current(), 118)
		if g.State != "driving" || g.ObservationID != "relay-work" {
			t.Fatal(g)
		}
	}
}
