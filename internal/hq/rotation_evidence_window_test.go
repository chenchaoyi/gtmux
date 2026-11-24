package hq

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestLegacyRotationWitnessRequiresMatchingGenerationAndSendOrder(t *testing.T) {
	r := rotationRequest{SentAt: 100, SubmittedAtNS: time.Unix(102, 100).UnixNano(), TargetStamp: "10:20:30"}
	for _, tc := range []struct {
		name, stamp string
		observed    int64
		valid       bool
	}{
		{"fresh", r.TargetStamp, 140, true},
		{"missing-generation", "", 140, false},
		{"previous-generation", "10:20:29", 140, false},
		{"same-second", r.TargetStamp, 102, false},
		{"between-attempt-and-submit", r.TargetStamp, 101, false},
		{"after-deadline", r.TargetStamp, 146, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proof := resume.HQObservation{Generation: tc.stamp, ObservedAt: tc.observed, ClientPID: 91, ClientBirth: "birth"}
			if got := legacyRotationObservation(proof, r, 147); got != tc.valid {
				t.Fatalf("legacy witness accepted=%v, want %v", got, tc.valid)
			}
		})
	}
	if legacyRotationObservation(resume.HQObservation{Generation: r.TargetStamp, ObservedAt: 140, ClientPID: 91, ClientBirth: "birth"}, r, 139) {
		t.Fatal("accepted a future observation")
	}
	if legacyRotationObservation(resume.HQObservation{Generation: r.TargetStamp, ObservedAt: 140}, r, 147) {
		t.Fatal("accepted an observation without the client incarnation")
	}
}

func TestLateTickOnlyConfirmsHistoryFromTheOriginalRotationWindow(t *testing.T) {
	for _, lateEvidence := range []bool{false, true} {
		name := "ready-before-deadline"
		if lateEvidence {
			name = "history-after-deadline"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Now().Truncate(time.Second)
			r := testRotationRequest(t, now)
			t.Setenv("CODEX_HOME", t.TempDir())
			r.SentAt = now.Unix()
			if err := writeRotationFile(r, false); err != nil {
				t.Fatal(err)
			}
			deadline := now.Add(rotationSettleWait)
			at := deadline.Add(-time.Second)
			if lateEvidence {
				at = deadline.Add(time.Second)
			}
			writeRotationHistory(t, "successor")
			p := &spyPane{screen: box("")}
			advanceHQRotation(r, r.Pane, r.Agent, "successor", p.io(), testBoundary("task_started", at), deadline.Add(2*time.Second))
			want := events.AuditEventRotate
			if lateEvidence {
				want = events.AuditEventRotateFailed
			}
			records := testRotationEvents(t)
			if len(records) != 2 || records[1].Event != want || len(p.pasted) != 0 || p.enters != 0 {
				t.Fatalf("want one %s with no keys: %+v, %+v", want, records, p)
			}
		})
	}
}
