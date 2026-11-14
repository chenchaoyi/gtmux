package hq

import (
	"testing"
	"time"
)

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
