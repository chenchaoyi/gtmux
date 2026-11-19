package dispatchbridge

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

var codexInputTarget = resume.CodexFrontendForPane
var codexQueueInputs = codexrpc.QueueInputs
var codexImageInputs = codexrpc.ImageInputs
var codexUserInputs = codexrpc.UserInputs
var codexIdleWakeInputs = codexrpc.IdleWakeInputs

func codexNativeSubmit(pane string) func(dispatch.Opts, string, func() string) dispatch.Result {
	return func(opts dispatch.Opts, text string, hold func() string) (result dispatch.Result) {
		if agentclient.PriorDeliveryUnknown(pane, opts.DeliveryID) {
			defer func() { result = result.WithUnverifiedPriorDelivery() }()
		}
		if opts.IdleWakeInput && (opts.DirectUserInput || opts.ActiveImageInput) {
			return codexInputRefusal(dispatch.StateRefusedWaiting, "conflicting-native-input-policy", "conflicting-native-input-policy")
		}
		f, ok := codexInputTarget(pane)
		if !ok {
			return codexInputRefusal(dispatch.StateRefusedWaiting, "codex-frontend-unavailable", "codex-frontend-unavailable")
		}
		if !opts.MatchesTarget(f.SessionID, f.HomeID, f.Instance, f.PID) {
			return codexInputRefusal(dispatch.StateRefusedWaiting, "expected-conversation-changed", "expected-conversation-changed")
		}
		a := agentclient.Attachment{Version: agentclient.ProtocolVersion, Agent: "codex", Client: "codex", Pane: pane,
			PID: f.PID, Instance: f.Instance, HomeID: f.HomeID, SessionID: f.SessionID, Cwd: f.Cwd, ServerEpoch: f.RootStart,
			BindingEpoch: f.StartedAt, StartedAt: f.StartedAt, UpdatedAt: time.Now().UnixMilli(), InputReady: true, NativeSession: true}
		intent, err := agentclient.StageInput(a, text, opts.DeliveryID, opts.Force && opts.DeliveryID == "", time.Now())
		if err != nil {
			return codexPreparationFailure(err)
		}
		r := intent.Request
		if receipt, known := agentclient.ReadReceipt(r); known {
			return codexRecordedResult(receipt)
		}
		if !r.MatchesTarget(a) {
			return codexInputRefusal(dispatch.StateRefusedWaiting, "codex-input-target-changed", "codex-input-target-changed")
		}
		ctx := opts.Context
		if ctx == nil {
			ctx = context.Background()
		}
		// Share the RPC's overall deadline with the final durable publication.
		ctx, cancel := context.WithTimeout(ctx, codexrpc.InputTimeout)
		defer cancel()
		claimed := false
		var claimErr, publishErr error
		check := func() bool {
			if ctx.Err() != nil {
				return false
			}
			current, valid := codexInputTarget(pane)
			if !r.MatchesTarget(a) || !valid || current.Instance != f.Instance || current.HomeID != f.HomeID || current.SessionID != f.SessionID || current.PID != f.PID || current.StartedAt != f.StartedAt || current.RootStart != f.RootStart || current.BackendPID != f.BackendPID || current.BackendStart != f.BackendStart || (hold != nil && hold() != "") {
				return false
			}
			if claimed {
				// An explicit unsupported method can retry via queue, but
				// cannot carry its claim into another generation.
				claimErr = r.CheckInputStore()
				return claimErr == nil
			}
			published, err := intent.PublishContext(ctx, time.Now())
			if err != nil {
				publishErr = err
				return false
			}
			r = published
			claimErr = agentclient.ClaimNative(r)
			if claimErr != nil {
				return false
			}
			claimed = true
			return true
		}
		inputs := codexInputs(text)
		submit := codexQueueInputs
		if opts.IdleWakeInput {
			submit = codexIdleWakeInputs
		} else if opts.DirectUserInput {
			submit = codexUserInputs
		} else if opts.ActiveImageInput && len(inputs) > 1 {
			submit = codexImageInputs
		}
		outcome := submit(ctx, codexrpc.Target{HomeID: f.HomeID, ThreadID: f.SessionID, BackendPID: f.BackendPID, BackendStart: f.BackendStart}, r.ID, inputs, check)
		if publishErr != nil {
			if errors.Is(publishErr, context.Canceled) || errors.Is(publishErr, context.DeadlineExceeded) {
				return codexInputRefusal(dispatch.StateRefusedWaiting, "input-deadline", "input-deadline")
			}
			if errors.Is(publishErr, agentclient.ErrInputUncertain) {
				if receipt, ok := agentclient.ReadReceipt(r); ok {
					return codexRecordedResult(receipt)
				}
			}
			return codexPreparationFailure(publishErr)
		}
		if claimErr != nil {
			if receipt, ok := agentclient.ReadReceipt(r); ok {
				return codexRecordedResult(receipt)
			}
			if !outcome.Attempted && errors.Is(claimErr, agentclient.ErrInputStoreUnavailable) {
				return codexPreparationFailure(claimErr)
			}
			return codexInputUnknown("codex-input-receipt-unconfirmed", outcome.Attempted)
		}
		if outcome.Accepted {
			delivery, evidence := "", "codex-queue-accepted:"+outcome.QueueID
			if outcome.ActiveTurnID != "" {
				delivery, evidence = "active-turn", "codex-active-accepted:"+outcome.ActiveTurnID
			}
			if outcome.StartedTurnID != "" {
				delivery, evidence = "started-turn", "codex-turn-accepted:"+outcome.StartedTurnID
			}
			if outcome.AcknowledgedTurnID != "" {
				delivery, evidence = "wake-accepted", "codex-wake-accepted:"+outcome.AcknowledgedTurnID
			}
			if err := agentclient.FinishNativeDelivery(r, "accepted", outcome.AcknowledgedTurnID, delivery); err != nil {
				return codexInputUnknown("codex-input-receipt-unconfirmed", true)
			}
			return dispatch.Result{State: dispatch.StateQueued, JudgedBy: dispatch.JudgedByDriver, Delivery: delivery, Evidence: evidence, Attempts: 1}
		}
		if claimed {
			verdict := "uncertain"
			if !outcome.Attempted {
				verdict = "refused"
			}
			if err := agentclient.FinishNative(r, verdict, outcome.Reason); err != nil {
				return codexInputUnknown("codex-input-receipt-unconfirmed", outcome.Attempted)
			}
			if outcome.Attempted {
				return codexInputUnknown("codex-input-uncertain:"+outcome.Reason, true)
			}
			if opts.IdleWakeInput {
				return codexInputRefusal(dispatch.StateRefusedWaiting, outcome.Reason, outcome.Reason)
			}
			return codexInputRefusal(dispatch.StateFailed, outcome.Reason, "codex-input-refused:"+outcome.Reason)
		}
		if outcome.Attempted {
			return codexInputUnknown("codex-input-uncertain:"+outcome.Reason, true)
		}
		return codexInputRefusal(dispatch.StateRefusedWaiting, outcome.Reason, outcome.Reason)
	}
}

func codexInputRefusal(state dispatch.State, reason, evidence string) dispatch.Result {
	code := dispatch.InputUnavailable
	switch reason {
	case "input-spool-full":
		code = dispatch.InputSpoolFull
	case "input-awaiting-confirmation":
		code = dispatch.InputQuestion
	case "codex-frontend-unavailable":
		code = dispatch.InputAssociationMissing
	case "expected-conversation-changed", "codex-input-target-changed", "target-changed", "backend-changed", "server-home-mismatch":
		code = dispatch.InputTargetChanged
	case "input-thread-not-loaded":
		code = dispatch.InputSessionNotLoaded
	}
	return dispatch.Result{State: state, Evidence: evidence, FailureCode: code,
		Submission: dispatch.SubmissionNotSubmitted, NativeQuestion: reason == "input-awaiting-confirmation"}
}

func codexInputUnknown(evidence string, attempted bool) dispatch.Result {
	result := dispatch.Result{State: dispatch.StateFailed, Evidence: evidence,
		FailureCode: dispatch.InputOutcomeUnconfirmed, Submission: dispatch.SubmissionUnknown}
	if attempted {
		result.Attempts = 1
	}
	return result
}

func codexPreparationFailure(err error) dispatch.Result {
	var refused *agentclient.InputRefused
	switch {
	case errors.Is(err, agentclient.ErrInputFull):
		return codexInputRefusal(dispatch.StateFailed, "input-spool-full", "input-spool-full")
	case errors.Is(err, agentclient.ErrInputUncertain):
		return codexInputUnknown("codex-input-receipt-unconfirmed", false)
	case errors.Is(err, agentclient.ErrInputTargetChanged):
		return codexInputRefusal(dispatch.StateFailed, "target-changed", "codex-input-retry-held: "+err.Error())
	case errors.As(err, &refused):
		return codexInputRefusal(dispatch.StateFailed, refused.Reason, "codex-input-refused:"+refused.Reason)
	default:
		return codexInputRefusal(dispatch.StateFailed, "", "codex-input-retry-held: "+err.Error())
	}
}

func codexRecordedResult(receipt agentclient.Receipt) dispatch.Result {
	switch receipt.State {
	case "accepted":
		kind := "queue"
		switch receipt.Delivery {
		case "wake-accepted":
			kind = "wake"
		case "started-turn":
			kind = "turn"
		case "active-turn":
			kind = "active"
		}
		return dispatch.Result{State: dispatch.StateQueued, JudgedBy: dispatch.JudgedByDriver, Delivery: receipt.Delivery, Evidence: "codex-" + kind + "-already-accepted"}
	case "refused":
		return codexPreparationFailure(&agentclient.InputRefused{Reason: receipt.Error})
	default:
		return codexPreparationFailure(agentclient.ErrInputUncertain)
	}
}

// Uploads already authorized as attachments retain their image semantics. Other
// files stay text paths. Never infer an image from arbitrary prose or follow a
// file symlink outside the private uploads directory.
func codexInputs(text string) []codexrpc.Input {
	inputs := []codexrpc.Input{{Type: "text", Text: text}}
	root := filepath.Join(state.Dir(), "uploads")
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		path := strings.TrimSpace(line)
		rel, err := filepath.Rel(root, path)
		if err != nil || !filepath.IsAbs(path) || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || seen[path] || len(inputs) >= 17 {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		file.Close()
		kind := http.DetectContentType(buf[:n])
		if kind != "image/png" && kind != "image/jpeg" && kind != "image/gif" && kind != "image/webp" {
			continue
		}
		seen[path] = true
		inputs = append(inputs, codexrpc.Input{Type: "localImage", Path: path})
	}
	return inputs
}
