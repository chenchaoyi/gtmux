package hq

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHWA004WakeNoticesRespondToChangedCauseAndOverdueMatter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const now = 1700000000
	if err := state.WriteMarker(filepath.Join(state.Dir(), "hqwakedegraded"), "down"); err != nil {
		t.Fatal(err)
	}
	prior := wakeDesktopNotice
	t.Cleanup(func() { wakeDesktopNotice = prior })
	var notifications []notify.Options
	wakeDesktopNotice = func(options notify.Options) bool {
		notifications = append(notifications, options)
		return false // independent journal still carries the failure
	}

	view := hqnudge.Health{Queued: 200, Uncertain: 320, Issues: []hqnudge.WakeIssue{{Reason: "claim-conflict", FirstAt: now, LastAt: now}}}
	watchWakeHealth(now, view)
	watchWakeHealth(now+10, view)
	view.Issues = append(view.Issues, hqnudge.WakeIssue{Reason: "queue-pressure", FirstAt: now + 11, LastAt: now + 11})
	watchWakeHealth(now+11, view)
	due := []WakeDeadline{{FollowupID: "goal-synthetic", MatterID: "synthetic-morning", Due: now}}
	WatchWakeDeadlines(now+65, due)
	WatchWakeDeadlines(now+66, due)
	if len(notifications) != 3 || !strings.Contains(notifications[2].Message, "goal-synthetic") {
		t.Fatalf("changed cause or independent deadline was masked or repeated: %#v", notifications)
	}
	if hqnudge.Pending() {
		t.Fatal("notice attempted to use the broken wake queue")
	}
	rows, _ := events.ReadSince(0)
	important := 0
	for _, r := range rows {
		if r.Event == "gtmux:wake-degraded" && r.Severity == events.SevImportant {
			important++
		}
	}
	if important != 3 {
		t.Fatalf("desktop unavailability suppressed independent diagnostic records: %d", important)
	}
	watchWakeHealth(now+wakeNoticeInterval+1, view)
	if len(notifications) != 4 {
		t.Fatal("continuing cause was not rate-limited per reason")
	}
	watchWakeHealth(now+wakeNoticeInterval+2, hqnudge.Health{})
	WatchWakeDeadlines(now+wakeNoticeInterval+2, nil)
	if len(notifications) != 4 {
		t.Fatal("a verified resolution emitted another alarm")
	}
}
