package app

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const replayStopSequence = 42

func TestWorkspaceContinuationIncidentReplay(t *testing.T) {
	if os.Getenv("GTMUX_CONTINUATION_FIXTURE") == "1" {
		workspaceContinuationFixture(t)
		return
	}
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux unavailable")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"idle", "quiet", "lost-wake", "unclaimed", "draft"} {
		t.Run(mode, func(t *testing.T) {
			home, err := os.MkdirTemp("/tmp", "hq-cont-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(home) })
			for k, v := range map[string]string{"HOME": home, "CODEX_HOME": filepath.Join(home, ".codex"), "CODEX_THREAD_ID": "", "CODEX_SESSION_ID": "", "TMUX": "", "TMUX_PANE": ""} {
				t.Setenv(k, v)
			}
			wrapper := filepath.Join(home, "tmux")
			config := filepath.Join(home, "tmux.conf")
			if err := os.WriteFile(config, []byte("set -g remain-on-exit on\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec "+shellQuote(bin)+" -S "+shellQuote(filepath.Join(home, "sock"))+" -f "+shellQuote(config)+" \"$@\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			oldBin := tmux.Bin
			tmux.Bin = wrapper
			t.Cleanup(func() { _ = tmux.OK("kill-server"); tmux.Bin = oldBin })
			if err := state.MkdirAll(state.HQHome(), 0700); err != nil {
				t.Fatal(err)
			}
			// Also resolve the private tmux server in the CLI subprocess.
			t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
			registerWorkspaceReplayProbe()
			paneOut, err := tmux.Run("new-session", "-d", "-x", "180", "-y", "35", "-s", "fixture",
				"-c", state.HQHome(), "-P", "-F", "#{pane_id}", "-e", "HOME="+home,
				"-e", "GTMUX_CONTINUATION_FIXTURE=1", "-e", "GTMUX_CONTINUATION_FIXTURE_MODE="+mode, "exec "+shellQuote(exe)+" -test.run=^TestWorkspaceContinuationIncidentReplay$")
			if err != nil {
				t.Fatal("private tmux", err, paneOut)
			}
			pane := strings.TrimSpace(paneOut)
			if err := state.WriteMarker(state.ActivePath(pane), "hq-fixture"); err != nil {
				t.Fatal(err)
			}
			wait := func(limit time.Duration, ready func() bool) {
				t.Helper()
				deadline := time.Now().Add(limit)
				for !ready() {
					if time.Now().After(deadline) {
						t.Fatalf("timeout; HQ screen: %q", tmux.CapturePane(pane))
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			wait(3*time.Second, func() bool { return strings.Contains(tmux.CapturePane(pane), "❯") })
			if mode == "quiet" {
				if code, out := goalCLI(t, "", "quiet", "on"); code != 0 {
					t.Fatal(code, out)
				}
			}
			if mode == "draft" {
				if !tmux.OK("send-keys", "-t", pane, "-l", "user draft") {
					t.Fatal("draft setup")
				}
				wait(time.Second, func() bool { return strings.Contains(tmux.CapturePane(pane), "❯ user draft") })
			}
			s := workspaceStore()
			now := time.Now().Unix()
			m, err := s.Create("review-matter", "Finish reviewed changes", now-100)
			if err != nil {
				t.Fatal(err)
			}
			c := hqworkspace.Conversation{Agent: "codex", HomeID: "review-home", SessionID: "review-thread"}
			if err := s.Associate(m.ID, hqworkspace.Association{ID: "reviewer", Conversation: c, Role: "execution", From: now - 100}, 0, now-100); err != nil {
				t.Fatal(err)
			}
			if err := s.ActivateObservations(replayStopSequence-1, now-90); err != nil {
				t.Fatal(err)
			}
			if err := s.ScanGoals(now-80, 64); err != nil {
				t.Fatal(err)
			}
			read := func() hqworkspace.Database {
				t.Helper()
				d, err := s.Read()
				if err != nil {
					t.Fatal(err)
				}
				return d
			}
			m = read().Matters[m.ID]
			g := hqworkspace.GoalFollowupID(m)
			q, err := s.ClaimObservation(g, "::hq-fixture", now-70)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Assess(hqworkspace.Assessment{ID: "wait-for-review", MatterID: m.ID, Revision: m.Revision, ObservationID: g, Owner: q.Owner, Generation: q.Generation, Decision: "deferred", Reason: "Await independent review", NextCheck: now + 600}, now-69, false); err != nil {
				t.Fatal(err)
			}
			// The synthetic formal report is durable before Stop.
			stop := events.Record{Seq: replayStopSequence, Event: "Stop", Ts: now, Pane: "%9", Agent: "Codex", HomeID: c.HomeID, AgentSession: c.SessionID}
			report := relay.Request{ID: "formal-review", Kind: "report", For: "hq", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, CreatedAt: now - 10, Body: "Two verified findings need fixes."}
			oldRead, oldSeq, oldRelay, oldObserve := workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceObserveEvent
			oldNext, oldWakeAt, oldStamp, oldWake := workspaceReconciliationNext, workspaceReconciliationWakeAt, workspaceGoalCheckStamp, workspaceReconcileWake
			t.Cleanup(func() {
				wait(5*time.Second, func() bool {
					return !workspaceReconciliationRunner.running.Load() && !workspaceBackground.running.Load()
				})
				workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceObserveEvent = oldRead, oldSeq, oldRelay, oldObserve
				workspaceReconciliationNext, workspaceReconciliationWakeAt, workspaceGoalCheckStamp, workspaceReconcileWake = oldNext, oldWakeAt, oldStamp, oldWake
			})
			workspaceReadEvents = func(after int64) ([]events.Record, bool) {
				if after < stop.Seq {
					return []events.Record{stop}, false
				}
				return nil, false
			}
			workspaceEventSequence = func() int64 { return stop.Seq }
			workspaceRelayRecords = func() []relay.Request { return []relay.Request{report} }
			workspaceObserveEvent = func(string, string, string, string, int64, ...string) bool { return true }
			attempted := make(chan time.Time, 4)
			workspaceReconcileWake = func(line string) {
				if !(mode == "lost-wake" && len(attempted) == 0) {
					enqueueWorkspaceReconciliationWake(line)
				}
				attempted <- time.Now()
			}
			workspaceReconciliationNext, workspaceReconciliationWakeAt = now+29, now+300
			if err := hqworkspace.RequestGoalCheck(state.HQHome()); err != nil {
				t.Fatal(err)
			}
			workspaceGoalCheckStamp = hqworkspace.GoalCheckStamp(state.HQHome())
			edge := time.Now()
			if err := hqworkspace.RequestGoalCheck(state.HQHome()); err != nil {
				t.Fatal(err)
			}
			if workspaceGoalCheckStamp >= hqworkspace.GoalCheckStamp(state.HQHome()) {
				t.Fatal("existing marker was not refreshed")
			}

			tick := func() {
				workspaceServeFastTick()
				wait(5*time.Second, func() bool { return !workspaceReconciliationRunner.running.Load() })
			}
			// Follow the actual serve cadence, with a real guarded wake drain.
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			<-ticker.C
			tick()
			if read().Reconciliations[g].State != "pending" || len(attempted) != 1 {
				t.Fatal("new report and Stop did not bypass both old deadlines")
			}
			q = read().Reconciliations["relay-formal-review"]
			if q.ClaimedAt != 0 || len(read().IntentAttempts) != 0 {
				t.Fatal("enqueue was mistaken for HQ or source execution")
			}
			if mode == "draft" {
				hq.DrainHQNudges()
				if q := read().Reconciliations["relay-formal-review"]; q.ClaimedAt != 0 || !strings.Contains(tmux.CapturePane(pane), "❯ user draft") {
					t.Fatal("wake disturbed the draft")
				}
				if !tmux.OK("send-keys", "-t", pane, "C-u") {
					t.Fatal("clear fixture draft")
				}
			}
			timeout := time.After(39 * time.Second)
			for q.State != "leased" {
				select {
				case <-ticker.C:
					tick()
					hq.DrainHQNudges()
					if tmux.Display(pane, "#{pane_dead}") == "1" {
						t.Fatal("HQ fixture exited", tmux.CapturePane(pane))
					}
					end := time.Now()
					wait(time.Second, func() bool {
						q = read().Reconciliations["relay-formal-review"]
						return q.State == "leased" || time.Since(end) > 400*time.Millisecond
					})
				case <-timeout:
					t.Fatalf("HQ never started; ledger: %+v; screen: %q", q, tmux.CapturePane(pane))
				}
			}
			first := <-attempted
			t.Logf("edge to initial enqueue=%s, edge to observed HQ claim=%s, owner=%q generation=%d", first.Sub(edge), time.Since(edge), q.Owner, q.Generation)
			if q.Owner != "::hq-fixture" || q.ClaimedAt <= 0 || q.Generation != 1 {
				t.Fatal("missing exact HQ claim from terminal CLI", q)
			}
			journal, _ := events.ReadSince(0)
			delivered := map[string]bool{}
			for _, row := range journal {
				if row.Event == events.AuditEventWakeDelivered && row.Pane == pane && strings.Contains(row.Summary, workspaceContinuationWake) {
					fields := strings.Fields(row.Summary)
					batch := fields[len(fields)-1]
					if delivered[batch] {
						t.Fatal("separate wake attempts reused one delivery id", batch)
					}
					delivered[batch] = true
					t.Logf("confirmed wake batch: %s", row.Summary)
				}
			}
			wantDelivered := 1
			if mode == "unclaimed" {
				wantDelivered = 2
			}
			if len(delivered) != wantDelivered {
				t.Fatalf("got %v confirmed wake batches, want %d", delivered, wantDelivered)
			}
			if (mode == "lost-wake" || mode == "unclaimed") && len(attempted) != 1 {
				t.Fatal("expected one periodic recovery for unclaimed evidence")
			}
			if keep, _ := workspaceReconciliationProbe(workspaceContinuationWake); keep {
				t.Fatal("claimed queue still eligible for wake")
			}
			d := read()
			if len(d.IntentAttempts) != 0 || hqworkspace.GoalResponsibility(d, d.Matters[m.ID], time.Now().Unix()).State != "verify" {
				t.Fatal("HQ pickup erased debt or authorized source input")
			}
		})
	}
}

func workspaceContinuationFixture(t *testing.T) {
	input := bufio.NewScanner(os.Stdin)
	withholdClaim := os.Getenv("GTMUX_CONTINUATION_FIXTURE_MODE") == "unclaimed"
	for {
		fmt.Print("❯ ")
		if !input.Scan() {
			return
		}
		if !strings.Contains(input.Text(), "workspace reconciliation follow-up") {
			t.Fatalf("unexpected submission %q", input.Text())
		}
		if withholdClaim {
			withholdClaim = false
			fmt.Println("First wake received without a claim")
			continue
		}
		for _, id := range []string{fmt.Sprintf("event-%d", replayStopSequence), "relay-formal-review"} {
			var claim hqworkspace.Reconciliation
			workspaceFixtureAction(t, workspaceAction{Op: "claim", ObservationID: id}, &claim)
			fmt.Printf("Claimed %s at generation %d\n", id, claim.Generation)
		}
	}
}
