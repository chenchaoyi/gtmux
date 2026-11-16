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
	resolve := func() (hqworkspace.Conversation, bool) {
		var found hqworkspace.Conversation
		count := 0
		for _, c := range workspaceGoalCandidates() {
			if c.Same(target) {
				found = c.Conversation
				count++
			}
		}
		return found, count == 1
	}
	live, ok := resolve()
	if !ok {
		return hqworkspace.Delivery{State: "identity-unavailable", SafeHold: true}
	}
	hold := func() string {
		if !current() {
			return "stale-premise"
		}
		c, ok := resolve()
		if !ok || c.Pane != live.Pane || c.Loc != live.Loc {
			return "identity-changed"
		}
		for _, candidate := range workspaceGoalCandidates() {
			if candidate.Same(target) {
				return workspaceInputHold(candidate.Status)
			}
		}
		return "identity-unavailable"
	}
	if why := hold(); why != "" {
		return hqworkspace.Delivery{State: why, SafeHold: true}
	}
	io := workspaceDispatch(live.Pane, target.Agent)
	io.Hold = hold
	opts := dispatchbridge.DeliverOpts(live.Pane, target.Agent, false, dispatch.LoadTuning())
	opts.DeliveryID = id
	opts.ExpectedSessionID = target.SessionID
	opts.ExpectedHomeID = target.HomeID
	result := dispatch.Deliver(io, opts, payload)
	events.AuditSend(live.Pane, string(result.State), payload, time.Now().Unix())
	safe := result.State == dispatch.StateRefusedDraft || result.State == dispatch.StateRefusedWaiting && !strings.HasPrefix(result.Evidence, dispatch.EvidenceHeldBeforeEnter) && !strings.HasPrefix(result.Evidence, dispatch.EvidenceHeldBeforeRetry)
	return hqworkspace.Delivery{State: string(result.State), Evidence: result.Evidence, SafeHold: safe}
}
