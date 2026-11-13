package hq

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHRW006TimeoutAlertsIndependentlyAndStaleSnapshotIsInert(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("unsent user draft")}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), testBoundary("task_started", now), now.Add(rotationQueueLimit))
	records := testRotationEvents(t)
	if len(records) != 2 || records[1].Event != events.AuditEventRotateFailed || records[1].Severity != events.SevRoutine {
		t.Fatal("original audit contract changed", records)
	}
	notifications, _ := os.ReadDir(state.NotifyDir())
	if runtime.GOOS == "darwin" && len(notifications) != 1 {
		t.Fatal("timeout lacked independent owner notification", len(notifications))
	}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), testBoundary("task_complete", now.Add(-time.Minute)), now.Add(rotationQueueLimit+time.Second))
	again := testRotationEvents(t)
	notifications2, _ := os.ReadDir(state.NotifyDir())
	if len(again) != len(records) || len(notifications2) != len(notifications) || len(p.pasted) != 0 || p.enters != 0 {
		t.Fatal("stale timeout snapshot repeated alert/input", again, p)
	}
}
