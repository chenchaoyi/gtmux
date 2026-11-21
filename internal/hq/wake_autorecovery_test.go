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
	if len(notifications) != 3 {
		t.Fatal("retained conflict or capacity observation was announced again")
	}
	watchWakeHealth(now+wakeNoticeInterval+2, hqnudge.Health{})
	WatchWakeDeadlines(now+wakeNoticeInterval+2, nil)
	if len(notifications) != 3 {
		t.Fatal("a verified resolution emitted another alarm")
	}
}

func TestWNR003WakeNoticeCoalescesNewCausesAndSkipsHistoricalInventory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	const now = 1700000000
	before := now - wakeNoticeInterval*2
	oldShown := before + 30
	path := filepath.Join(state.Dir(), "hqwake", "notices-channel.json")
	old := `{"observation-archived":1699998230,"input-refused":1699998230,"claim-conflict":1699998230}`
	if err := state.WritePrivateFile(path, []byte(old)); err != nil {
		t.Fatal(err)
	}
	prior := wakeDesktopNotice
	t.Cleanup(func() { wakeDesktopNotice = prior })
	var seen []notify.Options
	wakeDesktopNotice = func(options notify.Options) bool {
		seen = append(seen, options)
		return true
	}
	view := hqnudge.Health{Queued: 2, Uncertain: 320, Refused: 48, Issues: []hqnudge.WakeIssue{
		{Reason: "observation-archived", FirstAt: before, LastAt: oldShown - 1},
		{Reason: "input-refused", FirstAt: before, LastAt: oldShown - 1},
		{Reason: "claim-conflict", FirstAt: before, LastAt: now},
	}}
	watchWakeHealth(now, view)
	if len(seen) != 0 {
		t.Fatalf("retained reports were reannounced after a restart: %#v", seen)
	}
	view.Issues = append(view.Issues,
		hqnudge.WakeIssue{Reason: "queue-pressure", FirstAt: now + 1, LastAt: now + 1},
		hqnudge.WakeIssue{Reason: "stalled", FirstAt: now + 1, LastAt: now + 1})
	watchWakeHealth(now+1, view)
	rows, _ := events.ReadSince(0)
	if len(seen) != 1 || len(rows) != 1 ||
		!strings.Contains(seen[0].Message, "capacity was exceeded") ||
		!strings.Contains(seen[0].Message, "has not progressed") ||
		strings.Count(seen[0].Message, "320 uncertain") != 1 {
		t.Fatalf("simultaneous causes did not share one notification and journal entry: %#v, %#v", seen, rows)
	}
	again := now + wakeNoticeInterval + 2
	view.Issues[1].LastAt = again
	watchWakeHealth(again, view)
	if len(seen) != 2 || !strings.Contains(seen[1].Message, "could not be safely submitted") ||
		!strings.Contains(seen[1].Message, "has not progressed") ||
		strings.Contains(seen[1].Message, "active proof window") {
		t.Fatalf("new refusals or live stalls were hidden, or old archive repeated: %#v", seen)
	}
	WatchWakeDeadlines(again, []WakeDeadline{{FollowupID: "separate-followup", MatterID: "synthetic-matter", Due: now}})
	if len(seen) != 3 || !strings.Contains(seen[2].Message, "separate-followup") {
		t.Fatalf("channel changes masked a workspace deadline: %#v", seen)
	}
}
