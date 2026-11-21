package hqnudge

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestHWA012AcceptanceWaitsForIndependentProof(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Unix(1700000000, 0)
	const session = "synthetic-thread"
	const turn = "ack-turn"
	const home = "synthetic-home"
	f := &fake{}
	x := f.io()
	x.nowNano = func() int64 { return now.UnixNano() }
	var attempted string
	x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
		attempted = payload
		return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
			Session: session, Turn: turn, Home: home, Request: "receipt-request",
		}}, true
	}
	priorHome, priorWitness := wakeTranscriptHome, wakeWitness
	t.Cleanup(func() { wakeTranscriptHome, wakeWitness = priorHome, priorWitness })
	wakeTranscriptHome = func() string { return home }
	provideProof := false
	wakeWitness = func(s, r, p string, since time.Time) transcript.WakeWitness {
		if s != session || r != turn || p != attempted || !since.Equal(now) {
			t.Fatal("observer asked for unrelated evidence", s, r, p, since)
		}
		if !provideProof {
			return transcript.WakeWitness{}
		}
		return transcript.WakeWitness{Started: now.Add(time.Second), Delivered: now.Add(2 * time.Second)}
	}
	enqueue(x, "private synthetic source")
	drain(x, "%hq")
	ObserveWakeEvidence(now.Add(3 * time.Minute))
	if len(auditRecords(t, events.AuditEventWakeAccepted)) != 1 || len(auditRecords(t, events.AuditEventWakeDelivered)) != 0 {
		t.Fatal("a receiver ack was confused with delivery")
	}
	issues := ReadHealth(now.Add(3 * time.Minute).Unix()).Issues
	if len(issues) != 1 || issues[0].Reason != issueAcceptedUnverified {
		t.Fatal("unverified receiver result was silent", issues)
	}
	provideProof = true
	ObserveWakeEvidence(now.Add(4 * time.Minute))
	ObserveWakeEvidence(now.Add(4*time.Minute + time.Second))
	rows := auditRecords(t, events.AuditEventWakeDelivered)
	if len(rows) != 1 || !strings.Contains(rows[0].Summary, idOf(t, attempted)) || rows[0].Ts != now.Add(2*time.Second).Unix() {
		t.Fatal("proved delivery did not record the original ID and timestamp exactly once", rows)
	}
	for _, issue := range ReadHealth(now.Add(4 * time.Minute).Unix()).Issues {
		if issue.Reason == issueAcceptedUnverified {
			t.Fatal("exact evidence did not clear its notice")
		}
	}
	if f.pastes != 0 || f.enters != 0 {
		t.Fatal("observer must never mutate the terminal")
	}
}
