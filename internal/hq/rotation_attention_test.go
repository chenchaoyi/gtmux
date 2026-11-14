package hq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHRW011RotationFailureUsesNonWaitingAttention(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("unsent user draft")}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), testBoundary("task_started", now), now.Add(rotationQueueLimit))
	entries, err := os.ReadDir(state.NotifyDir())
	if runtime.GOOS != "darwin" {
		if len(entries) != 0 {
			t.Fatal("headless host queued desktop request")
		}
		return
	}
	if err != nil || len(entries) != 1 {
		t.Fatal("missing single notification", err, len(entries))
	}
	b, err := os.ReadFile(filepath.Join(state.NotifyDir(), entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var req struct {
		Kind string `json:"kind"`
		Pane string `json:"pane"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(b, &req); err != nil {
		t.Fatal(err)
	}
	t.Log("HRW011_NOTIFICATION_JSON=" + string(b))
	if req.Kind != "attention" || req.Pane != "%6" || req.Body == "" {
		t.Fatalf("rotation failure must not masquerade as waiting or done: %s", b)
	}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), testBoundary("task_complete", now), now.Add(rotationQueueLimit+time.Second))
	next, _ := os.ReadDir(state.NotifyDir())
	if len(next) != 1 || len(p.pasted) != 0 || p.enters != 0 {
		t.Fatal("old snapshot repeated notification or input")
	}
}
