package app

import (
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

var workspaceGoalCandidates = workspaceCandidates

// Resolve by exact identity each time, never by the saved pane locator. The
// ordinary dispatcher retains its input lock, draft guard and native receipts.
var workspaceDeliverGoal = func(target hqworkspace.Conversation, payload, id string, current func() bool) hqworkspace.Delivery {
	return deliverWorkspaceIntent(target, payload, id, current, false)
}

func resolveWorkspaceGoal(target hqworkspace.Conversation) (workspaceCandidate, bool) {
	var found workspaceCandidate
	count := 0
	for _, c := range workspaceGoalCandidates() {
		if c.Same(target) {
			found = c
			count++
		}
	}
	return found, count == 1
}

func replyInputHold(c workspaceCandidate, io dispatch.IO) string {
	if c.Supervisor || c.Agent != "codex" || c.Pane == "" || c.Loc == "" || io.SubmitNative == nil {
		return "unsupported-reply-target"
	}
	if c.Status != "idle" {
		return "target-not-idle: " + c.Status
	}
	if why, ok := dispatch.Reach(io); !ok {
		return string(why)
	}
	return ""
}

func deliverWorkspaceIntent(target hqworkspace.Conversation, payload, id string, current func() bool, reply bool) hqworkspace.Delivery {
	live, ok := resolveWorkspaceGoal(target)
	if !ok {
		return hqworkspace.Delivery{State: "identity-unavailable", SafeHold: true, BeforeSubmit: true, ObservedEventSeq: events.CurrentSeq()}
	}
	io := workspaceDispatch(live.Pane, target.Agent)
	hold := func() string {
		if !current() {
			return "stale-premise"
		}
		c, ok := resolveWorkspaceGoal(target)
		if !ok || c.Pane != live.Pane || c.Loc != live.Loc {
			return "identity-changed"
		}
		if reply {
			return replyInputHold(c, io)
		}
		return workspaceInputHold(c.Status)
	}
	if why := hold(); why != "" {
		return hqworkspace.Delivery{State: why, SafeHold: true, BeforeSubmit: true, ObservedEventSeq: events.CurrentSeq()}
	}
	// A native outcome alone does not prove that the mutation was never
	// attempted. Remember a refusal we observed in our own pre-claim gate.
	enteredNative, passedNativeGate, refusedAtGate := false, false, false
	if reply && io.SubmitNative != nil {
		submit := io.SubmitNative
		io.SubmitNative = func(o dispatch.Opts, text string, current func() string) dispatch.Result {
			enteredNative = true
			return submit(o, text, func() string {
				why := current()
				passedNativeGate = passedNativeGate || why == ""
				refusedAtGate = !passedNativeGate && why != ""
				return why
			})
		}
	}
	io.Hold = hold
	opts := dispatchbridge.DeliverOpts(live.Pane, target.Agent, false, dispatch.LoadTuning())
	opts.IdleWakeInput = reply
	opts.DeliveryID = id
	opts.ExpectedSessionID = target.SessionID
	opts.ExpectedHomeID = target.HomeID
	result := dispatch.Deliver(io, opts, payload)
	events.AuditSend(live.Pane, string(result.State), payload, time.Now().Unix())
	if reply && result.State == dispatch.StateRefusedDraft {
		// The workspace needs the refusal reason, never the draft text.
		result.Evidence = "draft"
	}
	safe := result.State == dispatch.StateRefusedDraft || result.State == dispatch.StateRefusedWaiting && !strings.HasPrefix(result.Evidence, dispatch.EvidenceHeldBeforeEnter) && !strings.HasPrefix(result.Evidence, dispatch.EvidenceHeldBeforeRetry)
	delivery := hqworkspace.Delivery{State: string(result.State), Evidence: result.Evidence, SafeHold: safe}
	if reply && safe && !result.Delivered && result.Delivery == "" && result.Attempts == 0 && (!enteredNative || refusedAtGate) {
		delivery.BeforeSubmit = true
		delivery.ObservedEventSeq = events.CurrentSeq()
	}
	return delivery
}
