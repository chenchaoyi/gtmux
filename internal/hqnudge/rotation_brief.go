package hqnudge

import (
	"context"
	"encoding/json"
	"errors"
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
const briefComposerUnavailable = "rotation-brief-composer-unavailable"
const briefTargetWaiting = "rotation-brief-target-proof-unavailable"
const briefRouteWaiting = "rotation-brief-native-route-unavailable"

type rotationBriefWake struct {
	Line string                `json:"line"`
	Bind hqinput.RotationBrief `json:"binding"`
}

type rotationBriefTarget struct {
	Home, Session, Stamp, Instance string
	PID                            int
	MissingProof                   bool
}

func liveRotationBriefTarget(pane string) rotationBriefTarget {
	if hqpane.Find() != pane {
		return rotationBriefTarget{}
	}
	f, err := resume.CodexFrontendForPaneReadiness(pane)
	stamp := tmux.PaneGeneration(pane)
	var missing *resume.FrontendBindingError
	target := rotationBriefTarget{
		Home: f.HomeID, Session: f.SessionID, Instance: f.Instance, PID: f.PID,
		Stamp: stamp, MissingProof: errors.As(err, &missing) && missing.Retryable,
	}
	if err != nil && !target.MissingProof {
		return rotationBriefTarget{} // a known conflicting observation is terminal
	}
	return target
}

func rotationBriefTargetHold(b hqinput.RotationBrief, target rotationBriefTarget) string {
	if target.Stamp != b.Stamp ||
		target.Home != "" && target.Home != b.Home || target.Session != "" && target.Session != b.Session ||
		target.Instance != "" && target.Instance != b.Instance || target.PID != 0 && target.PID != b.ClientPID {
		return "rotation-brief-target-changed"
	}
	if target.MissingProof {
		return briefTargetWaiting
	}
	if target.Home != b.Home || target.Session != b.Session || target.Instance != b.Instance || target.PID != b.ClientPID {
		return "rotation-brief-target-changed"
	}
	return ""
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
	if !emptyRecognizedWakeBox(x, pane) {
		return
	}
	path := filepath.Join(queueDir(), name)
	// Publish only the claim under the existing v2 journal format. The bound
	// envelope never becomes an ordinary pending text entry, even on rollback.
	claim := strings.TrimSuffix(path, rotationBriefSuffix) + ".txt" + sendingSuffix
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
	hold := func() string {
		switch {
		case x.nowNano()/int64(time.Second) >= w.Bind.Deadline:
			return "rotation-brief-expired"
		case !hqinput.MatchesRotationBrief(w.Bind):
			return "rotation-brief-request-changed"
		case pane != w.Bind.Pane:
			return "rotation-brief-target-changed"
		}
		if reason := rotationBriefTargetHold(w.Bind, x.rotationBriefTarget(pane)); reason != "" {
			return reason
		}
		if wakeBusy(x, pane) || !emptyRecognizedWakeBox(x, pane) {
			return briefComposerUnavailable
		}
		if x.nowNano()/int64(time.Second) >= w.Bind.Deadline {
			return "rotation-brief-expired"
		}
		return ""
	}
	settleHold := func(reason string) {
		if reason == briefComposerUnavailable || reason == briefTargetWaiting || reason == briefRouteWaiting {
			waitRotationBrief(claim, path, name, reason)
		} else {
			refuseRotationBrief(claim, reason)
		}
	}
	if reason := hold(); reason != "" {
		settleHold(reason)
		return
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(w.Bind.Deadline, 0))
	defer cancel()
	lastHold := ""
	opts := dispatch.Opts{
		Context: ctx, ExpectedSessionID: w.Bind.Session, ExpectedHomeID: w.Bind.Home,
		ExpectedInstance: w.Bind.Instance, ExpectedClientPID: w.Bind.ClientPID,
	}
	nativeHold := func() string {
		lastHold = hold()
		return lastHold
	}
	submit := prepareNativeWakeFor(pane, opts, nativeHold)
	if submit == nil {
		reason := hold()
		if reason == "" {
			// No native route was prepared, so this attempt cannot have submitted.
			reason = briefRouteWaiting
		}
		settleHold(reason)
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
	if result.Submission == dispatch.SubmissionNotSubmitted && result.Evidence == "codex-frontend-proof-unavailable" {
		switch hold() {
		case "", briefTargetWaiting, briefComposerUnavailable:
			lastHold = briefTargetWaiting
		}
	}
	if receipt := result.WakeReceipt; receipt != nil &&
		(receipt.Session != w.Bind.Session || receipt.Home != w.Bind.Home) {
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
	wait := intent.Outcome == "held" && result.Submission == dispatch.SubmissionNotSubmitted &&
		(lastHold == briefComposerUnavailable || lastHold == briefTargetWaiting || result.Evidence == "wake-loaded-idle-required" ||
			result.FailureCode == dispatch.InputDraftPresent || result.FailureCode == dispatch.InputQuestion)
	if !wait {
		recordWakeOutcome(intent, x.nowNano())
	}
	if intent.Outcome == "held" && !wait {
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
	if wait {
		// The native journal has settled a definite non-submission. If the
		// process stops before publication, retain its held claim, never infer
		// permission to resend from an orphan or an unknown result.
		reason := briefComposerUnavailable
		if lastHold == briefTargetWaiting {
			reason = lastHold
		}
		waitRotationBrief(finalClaimPath(intent, claim), path, name, reason)
		return
	}
	switch intent.Outcome {
	case "accepted":
		finishAccepted(pane, payload, nil)
	case "delivered":
		finishDelivered(pane, payload, nil)
	}
}

func waitRotationBrief(src, dst, name, reason string) {
	err := returnClaimAt(src, dst)
	diag.ReportStoreFailure("hq-rotation-brief", err)
	if err == nil {
		wakeStage("act.wake.waiting", name, "", diag.OK, reason)
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
