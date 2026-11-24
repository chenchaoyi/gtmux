package hq

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqsurface"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const wakeNoticeInterval = int64((15 * time.Minute) / time.Second)
const wakeNoticeLedgerLimit = 128 << 10

var errWakeNoticeLedgerTooLarge = errors.New("wake notice state exceeds the size limit")

type WorkspaceAttention struct {
	MatterID   string
	EvidenceID string
	NeedsUser  bool
}

var wakeDesktopNotice = notify.TrySend
var writeWakeNoticeLedger = state.WritePrivateFile

type wakeNotice struct {
	message           string
	desktopID         string
	observedAt        int64 // zero for conditions without an occurrence timestamp
	workspaceDecision bool
}

func watchWakeHealth(now int64, view hqnudge.Health) {
	alerts := make(map[string]wakeNotice)
	for _, issue := range view.Issues {
		why, action := wakeIssueAdvice(issue.Reason)
		timing := fmt.Sprintf("First observed %s", wakeObservedAt(issue.FirstAt))
		if issue.Reason == "accepted-unverified" {
			timing = fmt.Sprintf("Latest proof deadline %s", wakeObservedAt(issue.LastAt))
		}
		if issue.Reason == "stalled" {
			if cause := wakeHeldCause(); cause != "" {
				why = cause
				action = wakeNotifyMessage(cause)
			}
		}
		alerts[issue.Reason] = wakeNotice{
			message:    fmt.Sprintf("%s. %s. %s", why, timing, action),
			observedAt: wakeNoticeOccurrence(issue),
		}
	}
	summary := fmt.Sprintf("%d queued, %d uncertain, %d retained refusals.", view.Queued, view.Uncertain, view.Refused)
	publishWakeNotices("channel", "HQ wake delivery needs attention", alerts, now, len(alerts), summary)
}

func wakeNoticeOccurrence(issue hqnudge.WakeIssue) int64 {
	switch issue.Reason {
	case "claim-conflict":
		// A rescan can keep seeing the same conflict.
		return issue.FirstAt
	case "accepted-unverified", "queue-pressure", "input-refused", "retry-exhausted", "observation-archived",
		"submission-unknown", "terminal-submission-unknown":
		// These timestamps advance for recorded incidents, not retained file counts.
		return issue.LastAt
	default:
		return 0
	}
}

// WatchWorkspaceAttention projects actionable workbench state independently of
// the pane and wake transport. Routine recheck clocks stay in the workspace.
func WatchWorkspaceAttention(now int64, rows []WorkspaceAttention) {
	alerts := make(map[string]wakeNotice, len(rows))
	for _, row := range rows {
		title := "Review the saved attempt before retrying"
		reason := "blocked"
		if row.NeedsUser {
			title = "Answer the pending question"
			reason = "decision"
		}
		key := row.MatterID + "@" + reason + "@" + row.EvidenceID
		alerts[key] = wakeNotice{
			desktopID:         "workspace/" + row.MatterID,
			message:           fmt.Sprintf("%s in the HQ workspace for %s.", title, row.MatterID),
			workspaceDecision: row.NeedsUser,
		}
	}
	publishWakeNotices("workspace", "HQ workspace needs attention", alerts, now, 8, "")
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

func publishWakeNotices(scope, title string, alerts map[string]wakeNotice, now int64, maxNew int, summary string) {
	desktopScope := scope
	if scope == "relay-continuation" {
		desktopScope = "relay"
	}
	if desktopScope == "workspace" || desktopScope == "relay" {
		var active []string
		for _, notice := range alerts {
			active = append(active, notice.desktopID)
		}
		if err := notify.SyncAttention(desktopScope, active); err != nil {
			// Do not publish an orphan request or start cooldown. A later scan
			// can present the still-active condition after the store recovers.
			diag.ReportStoreFailure("hq-attention-notice", err)
			return
		}
	}
	path := state.StorePath("hqwake", "notices-"+scope+".json")
	previous, readErr := readWakeNotices(path)
	workspace := scope == "workspace"
	if workspace {
		previous, readErr = reconcileWorkspaceNoticeLedger(path, previous, alerts, readErr)
	}
	diag.ReportStoreFailure("hq-wake-notices", readErr)
	ledgerWritable := !workspace || readErr == nil
	next := make(map[string]int64, len(alerts))
	type candidate struct {
		key  string
		last int64
	}
	var ready []candidate
	for key, notice := range alerts {
		if notice.workspaceDecision && !ledgerWritable {
			continue // an unreadable record cannot authorize another attempt
		}
		last, attempted := previous[key]
		if workspace && !notice.workspaceDecision {
			if recent, ok := workspaceNoticeDelivery.blockers[key]; ok {
				last, attempted = recent, true
			}
		}
		if notice.workspaceDecision {
			marker := workspaceNoticePendingPrefix + key
			if at, pending := previous[marker]; pending {
				next[marker] = at
				if attempted {
					next[key] = last
				}
				continue
			}
		}
		if attempted {
			next[key] = last
			if notice.workspaceDecision {
				// Legacy positive timestamps are reused across upgrade.
				if last >= 0 {
					continue
				}
				last = -last // a recorded queue refusal can retry after cooldown
			}
			if notice.observedAt > 0 && notice.observedAt <= last && now >= last {
				continue
			}
		}
		if !attempted || now < last || now-last >= wakeNoticeInterval {
			ready = append(ready, candidate{key: key, last: last})
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		a, b := ready[i], ready[j]
		if a.last == b.last {
			return a.key < b.key
		}
		return a.last < b.last
	})
	changed := !maps.Equal(next, previous)
	selected := ready[:min(maxNew, len(ready))]
	if workspace {
		selected = ready // Skipped reservations do not use a sending slot.
	}
	queued := false
	if summary != "" && len(selected) > 0 {
		lines := []string{summary}
		for _, item := range selected {
			lines = append(lines, alerts[item.key].message)
		}
		queued = sendWakeNotice(title, strings.Join(lines, "\n"), "", now)
	}
	sent := 0
	for _, item := range selected {
		if sent >= maxNew {
			break
		}
		key := item.key
		notice := alerts[key]
		if notice.workspaceDecision {
			if !ledgerWritable {
				continue
			}
			old, exists := next[key]
			reserveWorkspaceNotice(next, key, now)
			if err := saveWakeNotices(path, next); err != nil {
				repair := workspaceNoticeRepair{key: key, reservedAt: now, value: old, keep: exists}
				repair.apply(next)
				if !errors.Is(err, errWakeNoticeLedgerTooLarge) {
					workspaceNoticeDelivery.repair = &repair
				}
				diag.ReportStoreFailure("hq-wake-notices", err)
				ledgerWritable = false
				continue
			}
		}
		if summary == "" {
			queued = sendWakeNotice(title, notice.message, notice.desktopID, now)
		}
		outcome := diag.OK
		if !queued {
			outcome = diag.Refused
		}
		diag.For("hq").Act("act.wake.notice", "system", scope, outcome, "HQ independent notice",
			"reason", key, "desktop_queued", queued)
		sent++
		if !notice.workspaceDecision {
			next[key] = now
			changed = true
			if workspace {
				workspaceNoticeDelivery.blockers[key] = now
			}
			continue
		}
		result := workspaceNoticeRepair{key: key, reservedAt: now, value: now, keep: true}
		if !queued {
			result.value = -now // a known refusal keeps the original retry time
		}
		result.apply(next)
		if err := saveWakeNotices(path, next); err != nil {
			workspaceNoticeDelivery.repair = &result
			diag.ReportStoreFailure("hq-wake-notices", err)
			ledgerWritable = false
		}
		changed = false // the decision write includes any earlier blocker attempts
	}
	if !changed || !ledgerWritable {
		return
	}
	diag.ReportStoreFailure("hq-wake-notices", saveWakeNotices(path, next))
}

func saveWakeNotices(path string, notices map[string]int64) error {
	data, err := json.Marshal(notices)
	if err != nil {
		return err
	}
	if len(data) > wakeNoticeLedgerLimit {
		return errWakeNoticeLedgerTooLarge
	}
	return writeWakeNoticeLedger(path, data)
}

func sendWakeNotice(title, message, identity string, now int64) bool {
	journalDegradation(hqsurface.ControlWakeDegraded, message, now)
	return wakeDesktopNotice(notify.Options{Kind: "attention", Title: title, Subtitle: "gtmux", Message: message, AttentionID: identity})
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
	data, err := io.ReadAll(io.LimitReader(f, wakeNoticeLedgerLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > wakeNoticeLedgerLimit {
		return nil, errWakeNoticeLedgerTooLarge
	}
	var previous map[string]int64
	err = json.Unmarshal(data, &previous)
	return previous, err
}

// RelayAttention identifies a durable source waiting for an HQ claim. A
// reservation records eligibility and says nothing about delivery.
type RelayAttention struct {
	ID         string
	Since      int64
	ReservedAt int64
	Unverified bool
}

func WatchRelayAttention(now int64, rows []RelayAttention) {
	alerts := make(map[string]wakeNotice, len(rows))
	for _, row := range rows {
		reason, detail := "unreserved", "HQ has not reserved a delivery attempt."
		if row.ReservedAt > 0 {
			reason, detail = "unclaimed", "HQ has not claimed this report."
		}
		if row.Unverified {
			reason, detail = "unverified", "HQ could not verify who has claimed this report."
		}
		key := fmt.Sprintf("%s@%d@%s", row.ID, row.Since, reason)
		alerts[key] = wakeNotice{desktopID: "relay/" + row.ID, message: fmt.Sprintf(
			"%s Review the original if it needs action: gtmux relay show %s", detail, row.ID)}
	}
	publishWakeNotices("relay-continuation", "HQ report still needs attention", alerts, now, 8, "")
}
