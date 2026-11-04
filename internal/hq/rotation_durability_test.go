package hq

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func writeRotationHistory(t *testing.T, id string) {
	t.Helper()
	dir := filepath.Join(transcript.CodexHome(), "sessions/2026/01/01")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-time-"+id+".jsonl"), []byte(`{"type":"session_meta","payload":{"id":"`+id+`","history_mode":"paginated"}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRotationDoesNotConfirmAnUnsavedNewThread(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	r.SentAt = now.Unix()
	if err := writeRotationFile(r, false); err != nil {
		t.Fatal(err)
	}
	p := &spyPane{screen: box("")}
	advanceHQRotation(r, "%6", "codex", "empty-thread", p.io(), testBoundary("", time.Time{}), now.Add(time.Second))
	saved, exists, _ := readRotationRequest()
	if !exists || saved.Successor != "empty-thread" || saved.SwitchedAt == 0 {
		t.Fatal(saved, exists)
	}
	advanceHQRotation(r, "%6", "codex", "empty-thread", p.io(), testBoundary("", time.Time{}), now.Add(50*time.Second))
	if _, exists, _ := readRotationRequest(); !exists {
		t.Fatal("45s identifier timeout discarded a known successor")
	}
	for _, event := range testRotationEvents(t) {
		if event.Event == events.AuditEventRotate {
			t.Fatal("empty thread marked successful")
		}
	}
	writeRotationHistory(t, "empty-thread")
	advanceHQRotation(r, "%6", "codex", "empty-thread", p.io(), testBoundary("task_started", now.Add(time.Second)), now.Add(time.Minute))
	if _, exists, _ := readRotationRequest(); exists {
		t.Fatal("durable working successor was not confirmed")
	}
	if len(p.pasted) != 0 {
		t.Fatal("confirmation typed input", p.pasted)
	}
}

func TestRotationQueuesOneGuardedHandoffAfterReleasingInput(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	t.Setenv("GTMUX_HQ_BRIEF", "on")
	r.SentAt = now.Unix()
	if err := writeRotationFile(r, false); err != nil {
		t.Fatal(err)
	}
	prior := rotationBootstrap
	t.Cleanup(func() { rotationBootstrap = prior })
	calls := 0
	rotationBootstrap = func(pane, sid, stamp string) {
		calls++
		if pane != "%6" || sid != "empty-thread" {
			t.Fatal(pane, sid)
		}
		release, ok, err := hqinput.Acquire()
		if err != nil || !ok {
			t.Fatal("bootstrap raced the shared input lock")
		}
		release()
	}
	p := &spyPane{screen: box("user draft")}
	boundary := testBoundary("", time.Time{})
	advanceHQRotation(r, "%6", "codex", "empty-thread", p.io(), boundary, now.Add(time.Second))
	if calls != 0 || len(p.pasted) != 0 {
		t.Fatal("draft triggered bootstrap")
	}
	p.screen = box("")
	advanceHQRotation(r, "%6", "codex", "empty-thread", p.io(), boundary, now.Add(2*time.Second))
	advanceHQRotation(r, "%6", "codex", "empty-thread", p.io(), boundary, now.Add(3*time.Second))
	if calls != 1 || len(p.pasted) != 0 {
		t.Fatal("handoff repeated or reset was retyped", calls, p.pasted)
	}
}

func TestRotationDoesNotTrustReissuedPaneNumbers(t *testing.T) {
	for _, current := range []string{"", "23:400:19", "17:301:19", "17:300:20"} {
		if rotationTargetMatches("17:300:19", current) {
			t.Fatal("new server/session/process accepted", current)
		}
	}
	if rotationTargetMatches("", "") || rotationTargetMatches("::", "::") {
		t.Fatal("unknown identity accepted")
	}
	if !rotationTargetMatches("17:300:19", "17:300:19") {
		t.Fatal("same runtime target rejected")
	}
}
