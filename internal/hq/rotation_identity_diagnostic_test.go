package hq

import (
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHRW012UnavailableIdentityIsNotChangedAndEscalatesOnce(t *testing.T) {
	for _, mode := range []string{"agent-missing", "session-missing", "active-conflict"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now()
			r := testRotationRequest(t, now)
			p := &spyPane{screen: box("")}
			agent, sid := "codex", "old-session"
			if mode == "agent-missing" {
				agent = ""
				sid = ""
			}
			if mode == "session-missing" {
				sid = ""
			}
			if mode == "active-conflict" {
				if err := state.WriteMarker(state.ActivePath("%6"), "different-active"); err != nil {
					t.Fatal(err)
				}
			}
			boundary := testBoundary("task_complete", now.Add(-time.Minute))
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(time.Second))
			if _, exists, _ := readRotationRequest(); !exists {
				t.Fatal("unavailable evidence was falsely settled as changed identity")
			}
			if len(p.pasted) != 0 || p.enters != 0 {
				t.Fatal("unknown identity authorized input")
			}
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(30*time.Second))
			if _, exists, _ := readRotationRequest(); !exists {
				t.Fatal("unavailable warning settled too early")
			}
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(61*time.Second))
			if _, exists, _ := readRotationRequest(); exists {
				t.Fatal("persistent missing management identity never escalated")
			}
			failed := 0
			for _, e := range testRotationEvents(t) {
				if e.Event == events.AuditEventRotateFailed {
					failed++
					if !strings.Contains(e.Summary, "unavailable") || strings.Contains(e.Summary, "session changed") {
						t.Fatal("incorrect failure cause", e.Summary)
					}
				}
			}
			if failed != 1 {
				t.Fatal("wrong number of failures", failed)
			}
			before, _ := os.ReadDir(state.NotifyDir())
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(62*time.Second))
			after, _ := os.ReadDir(state.NotifyDir())
			if len(before) != len(after) || len(p.pasted) != 0 || p.enters != 0 {
				t.Fatal("stale unknown snapshot repeated notification/input")
			}
		})
	}
}

func TestHRW012ExactIdentityRecoveryStillHonorsDraftAndChangedIdentity(t *testing.T) {
	for _, mode := range []string{"recovered-draft", "changed-session", "changed-agent"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now()
			r := testRotationRequest(t, now)
			p := &spyPane{screen: box("draft")}
			boundary := testBoundary("task_complete", now.Add(-time.Minute))
			advanceHQRotation(r, "%6", "codex", "", p.io(), boundary, now.Add(time.Second))
			if _, exists, _ := readRotationRequest(); !exists {
				t.Fatal("missing evidence misclassified")
			}
			agent, sid := "codex", "old-session"
			if mode == "changed-session" {
				sid = "actual-new"
			}
			if mode == "changed-agent" {
				agent = "claude"
			}
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(5*time.Second))
			_, exists, _ := readRotationRequest()
			if len(p.pasted) != 0 || p.enters != 0 {
				t.Fatal("recovery bypassed draft/changed identity guard")
			}
			if mode != "recovered-draft" {
				if exists {
					t.Fatal("observed real identity change was downgraded to unknown")
				}
				return
			}
			if !exists {
				t.Fatal("exact recovery with draft should retain request")
			}
			p.screen = box("")
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(6*time.Second))
			if len(p.pasted) != 1 || p.pasted[0] != "/new" || p.enters != 1 {
				t.Fatal("safe recovered identity could not proceed", p)
			}
		})
	}
}

func TestHRW012DiagnosticReasonChangesDoNotExtendDeadlineOrGrantReset(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("")}
	boundary := testBoundary("task_complete", now.Add(-time.Minute))
	// Values are valid but their proof is unavailable. A diagnostic cannot become authority.
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(time.Second), "proof-expired")
	first, exists, _ := readRotationRequest()
	if !exists || first.IdentityUnavailableAt != now.Add(time.Second).Unix() {
		t.Fatal("missing first unavailable observation", first)
	}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(30*time.Second), "bridge-proof-unavailable")
	second, exists, _ := readRotationRequest()
	if !exists || second.IdentityUnavailableAt != first.IdentityUnavailableAt || second.IdentityCause != "bridge-proof-unavailable" {
		t.Fatal("cause change extended deadline", second)
	}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(31*time.Second), "untrusted arbitrary details\n/home/private")
	third, exists, _ := readRotationRequest()
	if !exists || third.IdentityCause != "unknown" || third.IdentityUnavailableAt != first.IdentityUnavailableAt {
		t.Fatal("unbounded diagnostic was persisted", third)
	}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(61*time.Second), "proof-expired")
	if _, exists, _ := readRotationRequest(); exists || len(p.pasted) != 0 || p.enters != 0 {
		t.Fatal("changing diagnostic bypassed escalation/admission")
	}
}
