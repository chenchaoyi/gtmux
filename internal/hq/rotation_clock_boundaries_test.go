package hq

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestResetAttemptAndCompletedKeysHaveSeparateClocks(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	current := now
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("")}
	terminal := p.io()
	enter := terminal.Enter
	terminal.Enter = func() error {
		pending, ok, err := readRotationRequest()
		if err != nil || !ok || pending.SentAt != now.Unix() || pending.SubmittedAtNS != 0 {
			t.Fatalf("wrong state before Enter: %+v, %v, %v", pending, ok, err)
		}
		current = now.Add(2*time.Second + 100*time.Millisecond)
		return enter()
	}
	complete := testBoundary("task_complete", now.Add(-time.Minute))
	advanceHQRotationWithClock(r, r.Pane, r.Agent, r.Retiring, terminal, complete, func() time.Time { return current })
	sent, ok, err := readRotationRequest()
	if err != nil || !ok || sent.SentAt != now.Unix() || sent.SubmittedAtNS != current.UnixNano() || p.enters != 1 {
		t.Fatalf("wrong state after Enter: %+v, %v, %v, keys=%d", sent, ok, err, p.enters)
	}
	sent.TargetStamp = "10:20:30"
	proof := resume.HQObservation{Generation: sent.TargetStamp, ObservedAt: now.Add(time.Second).Unix(), ClientPID: 91, ClientBirth: "birth"}
	if legacyRotationObservation(proof, sent, current.Unix()) {
		t.Fatal("observation between recording the attempt and Enter proved a successor")
	}
	proof.ObservedAt = now.Add(3 * time.Second).Unix()
	if !legacyRotationObservation(proof, sent, proof.ObservedAt) {
		t.Fatal("post-Enter observation did not qualify")
	}
	sent.SubmittedAtNS = 0
	if legacyRotationObservation(proof, sent, proof.ObservedAt) {
		t.Fatal("an attempt with no completion fence proved legacy identity")
	}
	advanceHQRotation(r, r.Pane, "", "", p.io(), complete, current)
	if len(p.pasted) != 1 || p.enters != 1 {
		t.Fatal("an old snapshot repeated the reset")
	}
}

func TestIncompleteHistoryProbeCannotOpenAHandoffAfterSettleDeadline(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	r := testRotationRequest(t, now)
	t.Setenv("CODEX_HOME", t.TempDir())
	r.SentAt = now.Unix()
	if err := writeRotationFile(r, false); err != nil {
		t.Fatal(err)
	}
	deadline := now.Add(rotationSettleWait)
	current := deadline.Add(-time.Second)
	p := &spyPane{screen: box("")}
	advanceHQRotationWithClock(r, r.Pane, r.Agent, "successor", p.io(), func(string) (string, time.Time) {
		current = deadline
		return "", time.Time{}
	}, func() time.Time { return current })
	if _, ok, err := readRotationRequest(); err != nil || ok {
		t.Fatalf("settle timeout incorrectly opened a handoff: %v, %v", ok, err)
	}
	if records := testRotationEvents(t); len(records) != 2 || records[1].Event != events.AuditEventRotateFailed ||
		!strings.Contains(records[1].Summary, "without recoverable history at the settle deadline") || len(p.pasted) != 0 || p.enters != 0 {
		t.Fatalf("expected one settle failure with no input: %+v, pane=%+v", records, p)
	}
}

func TestBriefingTargetProbeCannotCrossItsOriginalDeadline(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	r := testRotationRequest(t, now)
	t.Setenv("GTMUX_HQ_BRIEF", "on")
	t.Setenv("CODEX_HOME", t.TempDir())
	r.SentAt, r.SwitchedAt, r.Successor = now.Unix(), now.Add(time.Second).Unix(), "successor"
	if err := writeRotationFile(r, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Unix(r.SwitchedAt, 0).Add(rotationReadyWait)
	current := deadline.Add(-time.Second)
	prevFront, prevBrief, prevNotice := rotationBriefFrontend, rotationBootstrap, wakeDesktopNotice
	t.Cleanup(func() { rotationBriefFrontend, rotationBootstrap, wakeDesktopNotice = prevFront, prevBrief, prevNotice })
	rotationBootstrap = func(hqinput.RotationBrief) { t.Error("slow target probe enqueued a late briefing") }
	notices := 0
	wakeDesktopNotice = func(notify.Options) bool { notices++; return true }
	rotationBriefFrontend = func(string) (resume.CodexFrontend, bool) {
		current = deadline
		return resume.CodexFrontend{SessionID: r.Successor, HomeID: transcript.CodexHomeID(), Instance: "frontend",
			CodexBindingTarget: resume.CodexBindingTarget{PID: 19}}, true
	}
	p := &spyPane{screen: box("")}
	advanceHQRotationWithClock(r, r.Pane, r.Agent, r.Successor, p.io(), testBoundary("", time.Time{}), func() time.Time { return current })
	if _, ok, err := readRotationRequest(); err != nil || ok {
		t.Fatalf("expired handoff stayed pending: %v, %v", ok, err)
	}
	records := testRotationEvents(t)
	if len(records) != 2 || records[1].Event != events.AuditEventRotateFailed ||
		!strings.Contains(records[1].Summary, "handoff/history was not confirmed") || notices != 1 || len(p.pasted) != 0 || p.enters != 0 {
		t.Fatalf("expected one failure with no input: %+v, notices=%d, pane=%+v", records, notices, p)
	}
}
