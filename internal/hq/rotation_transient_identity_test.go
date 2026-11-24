package hq

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/notify"
)

func TestSentRotationWaitsThroughTransientIdentityGaps(t *testing.T) {
	for _, mode := range []string{"recover", "settle-complete", "settle-incomplete", "settle-timeout", "session-timeout", "handoff-timeout", "wrong-pane", "reused-pane"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now().Truncate(time.Second)
			r := testRotationRequest(t, now)
			t.Setenv("GTMUX_HQ_BRIEF", "on")
			t.Setenv("CODEX_HOME", t.TempDir())
			r.TargetStamp = "17:300:19"
			if err := writeRotationFile(r, false); err != nil {
				t.Fatal(err)
			}
			stamp := r.TargetStamp
			previousStamp, previousBrief, previousNotice := rotationTargetStamp, rotationBootstrap, wakeDesktopNotice
			t.Cleanup(func() {
				rotationTargetStamp, rotationBootstrap, wakeDesktopNotice = previousStamp, previousBrief, previousNotice
			})
			rotationTargetStamp = func(string) string { return stamp }
			rotationBootstrap = func(hqinput.RotationBrief) { t.Error("unexpected briefing during identity gap") }
			var notices []notify.Options
			wakeDesktopNotice = func(o notify.Options) bool { notices = append(notices, o); return true }
			p := &spyPane{screen: box("")}
			advanceHQRotation(r, r.Pane, r.Agent, r.Retiring, p.io(), testBoundary("task_complete", now.Add(-time.Minute)), now)
			p.screen = box("user draft")
			advance := func(pane, agent, sid string, at time.Time, ready bool) {
				boundary := testBoundary("", time.Time{})
				if ready {
					boundary = testBoundary("task_started", now.Add(3*time.Second))
				}
				advanceHQRotation(r, pane, agent, sid, p.io(), boundary, at, "proof-expired")
			}
			checkPending := func(successor string, switchedAt int64, at time.Time) {
				t.Helper()
				pending, ok, err := readRotationRequest()
				if err != nil || !ok || pending.SentAt != now.Unix() || pending.Successor != successor ||
					pending.SwitchedAt != switchedAt || pending.BootstrapAt != 0 {
					t.Fatalf("pending state changed: %+v, %v, %v", pending, ok, err)
				}
				if window, err := hqinput.RotationWindow(r.Pane, at.Unix()); err != nil || window != hqinput.RotationHandoff {
					t.Fatalf("lost first-input reservation: %v, %v", window, err)
				}
			}
			advance(r.Pane, "", "", now.Add(time.Second), false)
			checkPending("", 0, now.Add(time.Second))
			deadline := now.Add(rotationSettleWait)
			successor, reason := "", "no new session ID appeared (HQ agent identity unavailable: proof-expired)"
			var switchedAt int64
			if mode == "recover" || mode == "handoff-timeout" {
				successor = "successor"
				switched := now.Add(2 * time.Second)
				switchedAt, deadline = switched.Unix(), switched.Add(rotationReadyWait)
				advance(r.Pane, r.Agent, successor, switched, false)
				advance(r.Pane, "", "", now.Add(50*time.Second), false)
				checkPending(successor, switchedAt, now.Add(50*time.Second))
				reason = "handoff/history was not confirmed (HQ agent identity unavailable: proof-expired)"
			}
			pane, agent, sid := r.Pane, "", ""
			switch mode {
			case "recover":
				writeRotationHistory(t, successor)
				agent, sid = r.Agent, successor
			case "settle-complete":
				successor = "successor"
				writeRotationHistory(t, successor)
				agent, sid = r.Agent, successor
			case "settle-incomplete":
				agent, sid, reason = r.Agent, "late-successor", "without recoverable history at the settle deadline"
			case "session-timeout":
				agent, reason = r.Agent, "HQ session identity unavailable: proof-expired"
			case "wrong-pane":
				pane, deadline, reason = "%7", now.Add(3*time.Second), "HQ pane or agent changed"
			case "reused-pane":
				stamp, deadline, reason = "17:300:20", now.Add(3*time.Second), "HQ target changed"
			}
			if mode != "wrong-pane" && mode != "reused-pane" {
				before := deadline.Add(-time.Second)
				advance(r.Pane, agent, "", before, mode == "recover")
				if switchedAt == 0 {
					checkPending("", 0, before)
				} else {
					checkPending(successor, switchedAt, before)
				}
			}
			confirmed := mode == "recover" || mode == "settle-complete"
			advance(pane, agent, sid, deadline, confirmed)
			advance(r.Pane, r.Agent, r.Retiring, deadline.Add(time.Second), false)
			if _, ok, err := readRotationRequest(); err != nil || ok || !slices.Equal(p.pasted, []string{"/new"}) || p.enters != 1 {
				t.Fatalf("terminal state or reset count was wrong: %v, %v, %+v", ok, err, p)
			}
			records := testRotationEvents(t)
			if len(records) != 2 {
				t.Fatalf("expected one outcome: %+v", records)
			}
			if confirmed {
				if records[1].Event != events.AuditEventRotate || records[1].AgentSession != successor || len(notices) != 0 {
					t.Fatalf("exact deadline evidence did not settle: %+v, %+v", records, notices)
				}
			} else if records[1].Event != events.AuditEventRotateFailed || !strings.Contains(records[1].Summary, reason) ||
				len(notices) != 1 || !strings.Contains(notices[0].Message, reason) {
				t.Fatalf("expected failure %q: %+v, %+v", reason, records, notices)
			}
		})
	}
}
