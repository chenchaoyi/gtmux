package hqnudge

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Old drains ignore this suffix. A bound briefing is always a standalone
// native batch and never lends its recipient or retry policy to another wake.
const rotationBriefSuffix = ".hqbrief"

type rotationBriefWake struct {
	Line string                `json:"line"`
	Bind hqinput.RotationBrief `json:"binding"`
}

type rotationBriefTarget struct {
	Home, Session, Stamp, Instance string
	PID                            int
}

func liveRotationBriefTarget(pane string) rotationBriefTarget {
	if hqpane.Find() != pane {
		return rotationBriefTarget{}
	}
	f, ok := resume.CodexFrontendForPane(pane)
	if !ok {
		return rotationBriefTarget{}
	}
	return rotationBriefTarget{
		Home: f.HomeID, Session: f.SessionID, Instance: f.Instance, PID: f.PID,
		Stamp: strings.TrimSpace(tmux.Display(pane, "#{pid}:#{session_created}:#{pane_pid}")),
	}
}

func DeliverRotationBrief(bind hqinput.RotationBrief, line string) {
	if !bind.Valid() || strings.TrimSpace(line) == "" {
		return
	}
	b, err := json.Marshal(rotationBriefWake{Line: line, Bind: bind})
	if err != nil {
		diag.ReportStoreFailure("hq-rotation-brief", err)
		return
	}
	enqueueWithSuffix(prod, line, string(b), rotationBriefSuffix)
	Drain(bind.Pane)
}

func isRotationBrief(name string) bool { return strings.HasSuffix(name, rotationBriefSuffix) }

func splitRotationBriefs(names []string) (ordinary []string, brief string) {
	ordinary = names[:0]
	for _, name := range names {
		if isRotationBrief(name) {
			if brief == "" {
				brief = name
			}
			continue
		}
		ordinary = append(ordinary, name)
	}
	return ordinary, brief
}

func drainRotationBrief(x io, pane, name string) {
	path := filepath.Join(queueDir(), name)
	claim := path + sendingSuffix
	if !takeQueued(path, claim) {
		return
	}

	b, err := readNativeBytes(claim)
	if err != nil {
		diag.ReportStoreFailure("hq-rotation-brief", err)
		// No receiver was called. Preserve the original claim on a storage error.
		return
	}
	var w rotationBriefWake
	if json.Unmarshal(b, &w) != nil || !w.Bind.Valid() || strings.TrimSpace(w.Line) == "" {
		refuseRotationBrief(claim, "rotation-brief-invalid")
		return
	}
	if x.rotationBriefTarget == nil {
		refuseRotationBrief(claim, "rotation-brief-target-unavailable")
		return
	}
	target := x.rotationBriefTarget(pane)
	hold := func() string {
		switch {
		case x.nowNano()/int64(time.Second) >= w.Bind.Deadline:
			return "rotation-brief-expired"
		case !hqinput.MatchesRotationBrief(w.Bind):
			return "rotation-brief-request-changed"
		case pane != w.Bind.Pane || target.Home != w.Bind.Home || target.Session != w.Bind.Session ||
			target.Stamp != w.Bind.Stamp || target.Instance != w.Bind.Instance || target.PID != w.Bind.ClientPID ||
			x.rotationBriefTarget(pane) != target:
			return "rotation-brief-target-changed"
		case wakeBusy(x, pane) || !emptyRecognizedWakeBox(x, pane):
			return "rotation-brief-composer-unavailable"
		case x.nowNano()/int64(time.Second) >= w.Bind.Deadline:
			return "rotation-brief-expired"
		default:
			return ""
		}
	}
	if reason := hold(); reason != "" {
		refuseRotationBrief(claim, reason)
		return
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(w.Bind.Deadline, 0))
	defer cancel()
	lastHold := ""
	submit := prepareNativeWakeFor(pane, dispatch.Opts{
		Context: ctx, ExpectedSessionID: target.Session, ExpectedHomeID: target.Home,
		ExpectedInstance: target.Instance, ExpectedClientPID: target.PID,
	}, func() string {
		lastHold = hold()
		return lastHold
	})
	if submit == nil {
		refuseRotationBrief(claim, "rotation-brief-native-unavailable")
		return
	}

	id := "#" + batchID([]string{claim}, w.Line)
	payload := w.Line + " · " + id
	intent, err := prepareNativeIntent(pane, id, payload, []string{claim})
	if err != nil {
		diag.ReportStoreFailure("hq-native-wake", err)
		recordIssue(issueRecoveryUnsafe, x.nowNano())
		return
	}
	wakeStage("act.wake.claimed", name, id, diag.OK, "rotation-brief")
	intent.SubmittedAt = x.nowNano()
	result, _ := submit(payload, id)
	if receipt := result.WakeReceipt; receipt != nil &&
		(receipt.Session != target.Session || receipt.Home != target.Home) {
		result = dispatch.Result{Submission: dispatch.SubmissionUnknown}
	}
	// An empty retry target preserves a definite refusal without scheduling
	// another attempt. A lost receipt still protects the original native claim.
	setNativeWakeOutcome(&intent, result, retryState{}, "", x.nowNano())
	if err := writeNativeIntent(intent); err != nil {
		diag.ReportStoreFailure("hq-native-wake", err)
		recordIssue(issueRecoveryUnsafe, x.nowNano())
		return
	}
	recordWakeOutcome(intent, x.nowNano())
	if intent.Outcome == "held" {
		if lastHold == "" {
			lastHold = nativeRefusalReason(result)
		}
		if lastHold == "" {
			lastHold = string(result.FailureCode)
		}
		if lastHold != "" {
			wakeStage("act.wake.rotation-brief.refusal", name, id, diag.Refused, lastHold)
		}
	}
	if err := settleNativeIntent(intent); err != nil {
		diag.ReportStoreFailure("hq-native-wake", err)
		recordIssue(issueRecoveryUnsafe, x.nowNano())
		return
	}
	switch intent.Outcome {
	case "accepted":
		finishAccepted(pane, payload, nil)
	case "delivered":
		finishDelivered(pane, payload, nil)
	}
}

func refuseRotationBrief(claim, reason string) {
	base := claimBase(claim)
	err := parkNativeClaimAt(claim, base+wakeRefusedSuffix)
	diag.ReportStoreFailure("hq-rotation-brief", err)
	if err == nil {
		wakeStage("act.wake.refused", base, "", diag.Refused, reason)
	}
}
