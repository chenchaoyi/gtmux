package hq

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqsurface"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const wakeNoticeInterval = int64((15 * time.Minute) / time.Second)

type WakeDeadline struct {
	FollowupID string
	MatterID   string
	Due        int64
}

var wakeDesktopNotice = notify.TrySend

func watchWakeHealth(now int64, view hqnudge.Health) {
	alerts := make(map[string]string)
	for _, issue := range view.Issues {
		why, action := wakeIssueAdvice(issue.Reason)
		if issue.Reason == "stalled" {
			if cause := wakeHeldCause(); cause != "" {
				why = cause
				action = wakeNotifyMessage(cause)
			}
		}
		alerts[issue.Reason] = fmt.Sprintf("%s. %d queued, %d uncertain, %d retained refusals. First observed %s. %s",
			why, view.Queued, view.Uncertain, view.Refused, wakeObservedAt(issue.FirstAt), action)
	}
	publishWakeNotices("channel", "HQ wake delivery needs attention", alerts, now)
}

// WatchWakeDeadlines is independent of the pane, wake transport and quiet mode.
// The workspace owns the due obligation and supplies only currently owed cases.
func WatchWakeDeadlines(now int64, due []WakeDeadline) {
	const detailedLimit = 32
	alerts := make(map[string]string, min(len(due), detailedLimit)+1)
	for _, d := range due[:min(len(due), detailedLimit)] {
		alerts[fmt.Sprintf("%s@%d", d.FollowupID, d.Due)] = fmt.Sprintf("Workspace deadline %s for %s is overdue (follow-up %s). The scheduler keeps checking the original matter. Open it to verify the result or decide on a new deadline.",
			wakeObservedAt(d.Due), d.MatterID, d.FollowupID)
	}
	if extra := len(due) - detailedLimit; extra > 0 {
		alerts["other-overdue"] = fmt.Sprintf("%d additional workspace deadlines remain overdue. Review the overdue matters in the workspace.", extra)
	}
	publishWakeNotices("deadline", "HQ workspace deadline overdue", alerts, now)
}

func wakeObservedAt(at int64) string {
	if at <= 0 {
		return "at this check"
	}
	return time.Unix(at, 0).Format(time.RFC3339)
}

func wakeIssueAdvice(reason string) (string, string) {
	switch reason {
	case "claim-conflict":
		return "A wake generation conflicts with retained evidence", "Independent wakes can continue. Verify the original receipts before deciding whether to retire the conflicting evidence."
	case "recovery-evidence":
		return "Wake recovery evidence could not be safely read", "Automatic dispatch is held. Inspect the local diagnostics and recover the original evidence before making a new attempt."
	case "queue-io":
		return "Wake queue I/O failed", "Check the reported state path or available space. Use the original workspace matter for urgent work."
	case "queue-pressure":
		return "Wake queue capacity was exceeded", "The scheduler keeps higher-priority work. Open the workspace to inspect due matters and verify which still need a report."
	case "retry-exhausted":
		return "A wake reached its safe retry limit", "The original is retained. Check whether the workspace matter still needs action before deciding on a new attempt."
	case "target-unavailable":
		return "The HQ pane identity could not be verified", "Automatic input remains queued. Reopen the original HQ conversation or verify the current attachment."
	case "input-refused":
		return "A wake could not be safely submitted to its original target", "Verify the HQ identity and check the original workspace matter before requesting another attempt."
	case "observation-archived":
		return "Some accepted wakes exceeded the active proof window", "Receiver acknowledgements are retained in the local wake archive. Review the affected workspace matter before deciding on a new attempt."
	case "accepted-unverified":
		return "The receiver accepted a wake without independent start or delivery evidence", "An automatic replay would risk duplication. Check the original matter and the exact receiver receipt."
	case "submission-unknown", "terminal-submission-unknown":
		return "A submitted wake has no confirmed outcome", "The original attempt is retained. Verify its receipt before deciding whether any manual action is needed."
	default:
		return "HQ wake delivery has not progressed", "Automatic drains continue to respect input guards. Verify the HQ pane and check the workspace for urgent work."
	}
}

func publishWakeNotices(scope, title string, alerts map[string]string, now int64) {
	path := filepath.Join(state.Dir(), "hqwake", "notices-"+scope+".json")
	previous, err := readWakeNotices(path)
	if err != nil {
		diag.ReportStoreFailure("hq-wake-notices", err)
		// Continue through the durable diagnostic and desktop channel.
	}
	next := make(map[string]int64, len(alerts))
	changed := len(previous) != len(alerts)
	for key, message := range alerts {
		last := previous[key]
		if last == 0 || now < last || now-last >= wakeNoticeInterval {
			journalDegradation(hqsurface.ControlWakeDegraded, message, now)
			queued := wakeDesktopNotice(notify.Options{Kind: "attention", Title: title, Subtitle: "gtmux", Message: message})
			outcome := diag.OK
			if !queued {
				outcome = diag.Refused
			}
			diag.For("hq").Act("act.wake.notice", "system", scope, outcome, "HQ independent notice",
				"reason", key, "desktop_queued", queued)
			last = now
			changed = true
		}
		next[key] = last
	}
	if !changed {
		return
	}
	data, err := json.Marshal(next)
	if err == nil {
		err = state.WritePrivateFile(path, data)
	}
	diag.ReportStoreFailure("hq-wake-notices", err)
}

func readWakeNotices(path string) (map[string]int64, error) {
	f, err := state.OpenPrivateRead(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const budget = 128 << 10
	data, err := io.ReadAll(io.LimitReader(f, budget+1))
	if err != nil {
		return nil, err
	}
	if len(data) > budget {
		return nil, errors.New("wake notice state exceeds the size limit")
	}
	var previous map[string]int64
	err = json.Unmarshal(data, &previous)
	return previous, err
}
