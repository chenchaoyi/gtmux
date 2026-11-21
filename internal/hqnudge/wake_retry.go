package hqnudge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const nativeAttemptLimit = 3
const wakeRefusedSuffix = ".refused"

type retryState struct {
	Number int
	At     int64
	Target string
}

func parseRetry(name string) (retryState, bool) {
	base := strings.TrimSuffix(filepath.Base(name), ".txt")
	idx := strings.LastIndex(base, ".r")
	if idx < 0 {
		return retryState{}, true
	}
	fields := strings.Split(base[idx+2:], "-")
	if len(fields) != 3 {
		return retryState{}, false
	}
	n, nErr := strconv.Atoi(fields[0])
	at, atErr := strconv.ParseInt(fields[1], 10, 64)
	if nErr != nil || atErr != nil || n < 1 || n >= nativeAttemptLimit || at <= 0 || !validWakeTarget(fields[2]) {
		return retryState{}, false
	}
	return retryState{Number: n, At: at, Target: fields[2]}, true
}

func validWakeTarget(target string) bool {
	return len(target) == 32 && strings.Trim(target, "0123456789abcdef") == ""
}

func retryName(name string, retry retryState) string {
	return fmt.Sprintf("%s.r%d-%d-%s.txt", identityOf(name), retry.Number, retry.At, retry.Target)
}

func dueForAttempt(name string) int64 {
	retry, _ := parseRetry(name)
	return max(dueOf(name), retry.At)
}

func nativeAttemptID(id string, retry retryState) string {
	if retry.Number == 0 {
		return id // original native delivery ID is backward compatible
	}
	return fmt.Sprintf("%s-r%d-%s", id, retry.Number, retry.Target[:12])
}

// Keep budgets per generation. A newly arrived entry never inherits the retry
// count from another wake, or changes the composition of a retry group.
func sameRetryGroup(a, b retryState) bool {
	return a == b
}

func wakeIdentity(x io, pane string) string {
	if x.identity == nil {
		return ""
	}
	return x.identity(pane)
}

func liveWakeIdentity(pane string) string {
	if hqpane.Find() != pane {
		return ""
	}
	target := tmux.Display(pane, "#{pid}:#{session_created}:#{pane_pid}")
	parts := strings.Split(target, ":")
	if len(parts) != 3 {
		return ""
	}
	for _, part := range parts {
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || n <= 0 {
			return ""
		}
	}
	var identity string
	if f, ok := resume.CodexFrontendForPane(pane); ok {
		identity = fmt.Sprintf("%s:%s:%s:%d:%s", f.HomeID, f.SessionID, f.Instance, f.BackendPID, f.BackendStart)
	} else {
		loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
		r, ok := resume.LoadLive(loc)
		if !ok || r.SessionID == "" || r.Agent == "" || r.Agent != tmux.Display(pane, "#{pane_current_command}") {
			return ""
		}
		identity = r.Agent + ":" + r.SessionID
	}
	hash := sha256.Sum256([]byte(target + ":" + identity))
	return hex.EncodeToString(hash[:16])
}

func nativeRefusalReason(result dispatch.Result) string {
	switch result.Evidence {
	case "wake-loaded-idle-required", "local-proxy-unavailable", "input-spool-full", "input-deadline":
		return result.Evidence
	}
	switch result.FailureCode {
	case dispatch.InputDraftPresent, dispatch.InputQuestion, dispatch.InputSpoolFull:
		return string(result.FailureCode)
	}
	return ""
}

func setNativeWakeOutcome(intent *nativeIntent, result dispatch.Result, retry retryState, target string, now int64) {
	switch {
	case result.Submission == dispatch.SubmissionUnknown:
		intent.Outcome = "uncertain"
		intent.Reason = issueUnknownInput
	case result.State == dispatch.StateQueued:
		intent.Outcome = "accepted"
		if result.WakeReceipt != nil && validWakeReceipt(*result.WakeReceipt) {
			intent.WakeReceipt = result.WakeReceipt
		}
	case result.State == dispatch.StateLanded:
		intent.Outcome = "delivered"
	case result.Submission != dispatch.SubmissionNotSubmitted:
		intent.Outcome = "uncertain"
		intent.Reason = issueUnknownInput
	default:
		intent.Outcome = "held"
		intent.Reason = issueInputRefused
		if nativeRefusalReason(result) == "" || !validWakeTarget(target) {
			return
		}
		if retry.Number+1 >= nativeAttemptLimit {
			intent.Reason = issueRetryExhausted
			return
		}
		intent.Outcome = "refused"
		delay := 5 * time.Second
		if retry.Number > 0 {
			delay = 15 * time.Second
		}
		intent.Retry = retryState{Number: retry.Number + 1, At: now + int64(delay), Target: target}
		intent.Reason = nativeRefusalReason(result)
	}
}

func recordWakeOutcome(intent nativeIntent, now int64) {
	var stage string
	status := diag.OK
	switch intent.Outcome {
	case "accepted":
		stage = "act.wake.accepted"
	case "delivered":
		stage = "act.wake.delivered"
	case "refused":
		stage, status = "act.wake.retry.scheduled", diag.Refused
	case "held":
		stage, status = "act.wake.refused", diag.Refused
		recordIssue(intent.Reason, now)
	case "uncertain":
		stage, status = "act.wake.outcome.unknown", diag.Failed
		recordIssue(issueUnknownInput, now)
	default:
		return
	}
	for _, entry := range intent.Entries {
		wakeStage(stage, claimBase(entry.Name), intent.ID, status, intent.Reason)
	}
}
