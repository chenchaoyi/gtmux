package hq

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestLegacyRotationWitnessOrdersSameSecondPrecisely(t *testing.T) {
	completed := time.Unix(105, 270)
	r := rotationRequest{SentAt: 104, SubmittedAtNS: completed.UnixNano(), TargetStamp: "10:20:30"}
	proof := resume.HQObservation{Generation: r.TargetStamp, ClientPID: 21, ClientBirth: "birth"}
	for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
		proof.ObservedAtNS = completed.Add(offset).UnixNano()
		if got := legacyRotationObservation(proof, r, completed.Add(time.Second)); got != (offset > 0) {
			t.Fatalf("observation offset %v accepted=%v", offset, got)
		}
	}
}

func TestSentRotationWithNoCompletedKeyFenceFailsWithoutInput(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	r := testRotationRequest(t, now)
	t.Setenv("CODEX_HOME", t.TempDir())
	r.SentAt, r.ReserveBrief = now.Unix(), true
	if err := writeRotationFile(r, false); err != nil {
		t.Fatal(err)
	}
	writeRotationHistory(t, "successor")
	p := &spyPane{screen: box("")}
	boundary := testBoundary("task_started", now.Add(time.Second))
	advanceHQRotation(r, r.Pane, r.Agent, "successor", p.io(), boundary, now.Add(2*time.Second))
	if saved, ok, err := readRotationRequest(); err != nil || !ok || saved != r {
		t.Fatalf("came to a conclusion without the reset fence: %+v, %v, %v", saved, ok, err)
	}
	advanceHQRotation(r, r.Pane, r.Agent, "successor", p.io(), boundary, now.Add(rotationSettleWait))
	advanceHQRotation(r, r.Pane, r.Agent, r.Retiring, p.io(), boundary, now.Add(rotationSettleWait+time.Second))
	if records := testRotationEvents(t); len(records) != 2 || records[1].Event != events.AuditEventRotateFailed ||
		!strings.Contains(records[1].Summary, "key completion was not recorded") || len(p.pasted) != 0 || p.enters != 0 {
		t.Fatalf("missing proof must fail once with no input: %+v, pane=%+v", records, p)
	}
}
