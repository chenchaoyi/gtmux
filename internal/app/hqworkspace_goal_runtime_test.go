package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func goalCLI(t *testing.T, body string, args ...string) (int, string) {
	t.Helper()
	in, e := os.CreateTemp(t.TempDir(), "stdin")
	if e != nil {
		t.Fatal(e)
	}
	defer in.Close()
	if _, e = in.WriteString(body); e != nil {
		t.Fatal(e)
	}
	_, _ = in.Seek(0, 0)
	out, e := os.CreateTemp(t.TempDir(), "stdout")
	if e != nil {
		t.Fatal(e)
	}
	defer out.Close()
	priorIn, priorOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = in, out
	defer func() { os.Stdin, os.Stdout = priorIn, priorOut }()
	code := Run(args)
	if _, e = out.Seek(0, 0); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(out.Name())
	if e != nil {
		t.Fatal(e)
	}
	return code, string(b)
}

func TestGoalRuntimeCLIStopServeCallbackAndGuardedDispatch(t *testing.T) {
	for _, scenario := range []string{"queued", "draft", "reused-sid"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			t.Setenv("TMUX", "")
			t.Setenv("TMUX_TMPDIR", t.TempDir())
			t.Setenv("TMUX_PANE", "%199")
			t.Setenv("GTMUX_QUIET", "")
			t.Setenv("GTMUX_SURFACE_TIER", "")
			oldBin := tmux.Bin
			tmux.Bin = ""
			t.Cleanup(func() { tmux.Bin = oldBin })
			oldHQ, oldOwner, oldCandidates, oldDispatch := workspaceIsHQCaller, workspaceAssessmentOwner, workspaceGoalCandidates, workspaceDispatch
			oldEvents, oldSeq, oldRelays, oldWake := workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceReconcileWake
			oldNext, oldWakeAt, oldStamp := workspaceReconciliationNext, workspaceReconciliationWakeAt, workspaceGoalCheckStamp
			t.Cleanup(func() {
				workspaceIsHQCaller, workspaceAssessmentOwner, workspaceGoalCandidates, workspaceDispatch = oldHQ, oldOwner, oldCandidates, oldDispatch
				workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceReconcileWake = oldEvents, oldSeq, oldRelays, oldWake
				workspaceReconciliationNext, workspaceReconciliationWakeAt, workspaceGoalCheckStamp = oldNext, oldWakeAt, oldStamp
			})
			workspaceIsHQCaller = func() bool { return true }
			workspaceAssessmentOwner = func() string { return "synthetic-hq" }
			workspaceReadEvents = func(int64) ([]events.Record, bool) { return nil, false }
			workspaceEventSequence = func() int64 { return 0 }
			workspaceRelayRecords = func() []relay.Request { return nil }
			if code, out := goalCLI(t, "", "--lang=en", "quiet", "on"); code != 0 || !strings.Contains(out, "quiet = on") {
				t.Fatal("actual quiet entry failed", code, out)
			}
			now := time.Now().Unix()
			s := workspaceStore()
			m, e := s.Create("goal-runtime", "Runtime fixture", now-10)
			if e != nil {
				t.Fatal(e)
			}
			c := hqworkspace.Conversation{Agent: "codex", HomeID: "synthetic-home", SessionID: "synthetic-worker", Pane: "%199", Loc: "synthetic:0.0"}
			if e = s.Associate(m.ID, hqworkspace.Association{ID: "runtime-worker", Conversation: c, Role: "execution", From: now - 10}, 0, now-9); e != nil {
				t.Fatal(e)
			}
			if e = s.ActivateObservations(0, now-8); e != nil {
				t.Fatal(e)
			}
			if code, _ := goalCLI(t, `{"session_id":"synthetic-hook","turn_id":"fixture","assistant":"stage ended"}`, "hook", "--agent", "kimi", "--detached", "Stop"); code != 0 {
				t.Fatal("actual Stop entry failed", code)
			}
			if hqworkspace.GoalCheckStamp(state.HQHome()) == 0 {
				t.Fatal("Stop entry did not request a goal check")
			}
			wake := make(chan string, 1)
			workspaceReconcileWake = func(line string) { wake <- line }
			workspaceReconciliationNext = now + 3600
			workspaceReconciliationWakeAt = 0
			workspaceGoalCheckStamp = 0
			workspaceServeFastTick() // The exact workspace callback invoked by serve, not a test copy.
			var line string
			select {
			case line = <-wake:
			case <-time.After(3 * time.Second):
				t.Fatal("no-event quiet callback lost goal debt")
			}
			deadline := time.Now().Add(3 * time.Second)
			for workspaceReconciliationRunner.running.Load() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if workspaceReconciliationRunner.running.Load() {
				t.Fatal("workspace callback did not settle")
			}
			if code, out := goalCLI(t, "", "hq", "workspace", "reconcile", "--json"); code != 0 || !strings.Contains(out, "goal-followup") {
				t.Fatal("actual reconcile CLI lost obligation", code, out)
			}
			act := func(a workspaceAction) string {
				t.Helper()
				b, _ := json.Marshal(a)
				code, out := goalCLI(t, string(b), "hq", "workspace", "act", "--stdin", "--json")
				if code != 0 {
					t.Fatal("actual action entry failed", a.Op, code, out)
				}
				return out
			}
			d, _ := s.Read()
			m = d.Matters[m.ID]
			var q hqworkspace.Reconciliation
			if e = json.Unmarshal([]byte(act(workspaceAction{Op: "claim", ObservationID: hqworkspace.GoalFollowupID(m)})), &q); e != nil {
				t.Fatal(e)
			}
			p := hqworkspace.Responsibility{StepID: "runtime-step", Action: "Finish the synthetic fixture", AuthorityRef: "user:synthetic-scope", Intent: &hqworkspace.ActionIntent{ID: "runtime-action", Ref: "message:runtime-action", Target: c, At: now - 1}}
			act(workspaceAction{Op: "assess", ID: "runtime-plan", ObservationID: q.Observation.ID, Generation: q.Generation, Matter: m.ID, Revision: m.Revision, Decision: "responsibility", Body: "Verified existing synthetic authority", Responsibility: &p})
			visible := c
			if scenario == "reused-sid" {
				visible.SessionID = "different-worker"
			}
			workspaceGoalCandidates = func() []workspaceCandidate { return []workspaceCandidate{{Conversation: visible, Status: "idle"}} }
			draft := ""
			if scenario == "draft" {
				draft = "human draft"
			}
			submitted := 0
			fakeNow := now
			workspaceDispatch = func(string, ...string) dispatch.IO {
				return dispatch.IO{
					Capture: func() string { return "• Working (esc to interrupt)\n\n› " + draft + "\n\n  ? for shortcuts" },
					Paste:   func(string) error { t.Fatal("native goal fell back to pane paste"); return nil }, Enter: func() error { t.Fatal("native goal fell back to pane Enter"); return nil },
					Now: func() int64 { return fakeNow }, Sleep: func() { fakeNow++ },
					SubmitNative: func(opts dispatch.Opts, _ string, hold func() string) dispatch.Result {
						if opts.Force || opts.ClobberDraft || opts.DeliveryID != p.Intent.ID || opts.ExpectedSessionID != c.SessionID || opts.ExpectedHomeID != c.HomeID || hold() != "" {
							t.Fatal("address or input gates changed", opts)
						}
						submitted++
						return dispatch.Result{State: dispatch.StateQueued, Evidence: "synthetic durable queue receipt"}
					},
				}
			}
			d, _ = s.Read()
			a := workspaceAction{Op: "drive", ID: p.Intent.ID, Matter: m.ID, Revision: d.Matters[m.ID].Revision}
			var receipt hqworkspace.IntentAttempt
			if e = json.Unmarshal([]byte(act(a)), &receipt); e != nil {
				t.Fatal(e)
			}
			want := "held"
			calls := 0
			if scenario == "queued" {
				want = "queued"
				calls = 1
			}
			if receipt.State != want || submitted != calls {
				t.Fatal("actual adapter/dispatch gate mismatch", receipt, submitted)
			}
			act(a)
			if submitted != calls {
				t.Fatal("retry reached native transport twice")
			}
			d, _ = s.Read()
			for _, row := range hqworkspace.ReconciliationQueue(d, time.Now().Unix()) {
				if row.Observation.Kind == "goal-followup" {
					continue
				}
				var claimed hqworkspace.Reconciliation
				_ = json.Unmarshal([]byte(act(workspaceAction{Op: "claim", ObservationID: row.Observation.ID})), &claimed)
				current, _ := s.Read()
				act(workspaceAction{Op: "assess", ID: "noted-" + row.Observation.ID, ObservationID: row.Observation.ID, Generation: claimed.Generation, Matter: m.ID, Revision: current.Matters[m.ID].Revision, Decision: "noted", Body: "Source checked; goal responsibility remains separate"})
			}
			d, _ = s.Read()
			act(workspaceAction{Op: "update", Matter: m.ID, Revision: d.Matters[m.ID].Revision, State: "completed", Body: "Verified synthetic whole-goal result"})
			if keep, _ := workspaceReconciliationProbe(line); keep {
				t.Fatal("actual premise probe kept an obsolete reminder")
			}
		})
	}
}
