package app

import (
	"errors"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

// Only explicit HQ reservations reach this pump. An ordinary reply, even one
// from an idle source with an open goal, has no path to dispatch.
func pumpWorkspaceReplies(s *hqworkspace.Store, now int64) error {
	d, err := s.Read()
	if err != nil {
		return err
	}
	for _, a := range d.IntentAttempts {
		if a.State != "armed" || a.ReplyID == "" {
			continue
		}
		why := hqworkspace.ReplyBlockReason(d, a, now)
		if why == "" {
			why = replySourceHold(a, d)
		}
		if why == "" {
			c, ok := resolveWorkspaceGoal(a.Intent.Target)
			if !ok {
				why = "identity-unavailable"
			} else {
				why = replyInputHold(c, workspaceDispatch(c.Pane, c.Agent))
			}
		}
		if why != "" {
			if err := s.ReplyBlocker(a, why, now); err != nil {
				return err
			}
			continue
		}
		_, err = s.DriveReply(a, now, func(c hqworkspace.Conversation, text, id string, current func() bool) hqworkspace.Delivery {
			return deliverWorkspaceIntent(c, text, id, func() bool { return current() && replySourceHold(a, d) == "" }, true)
		})
		if err != nil && !errors.Is(err, hqworkspace.ErrConflict) {
			return err
		}
	}
	return nil
}

func eligibleWorkspaceReply(r relay.Request) bool {
	return r.State == "replied" && r.ReplyChannel == "inbox" && r.For == "hq" && r.Agent == "codex" && r.HomeID != "" && r.ThreadID != "" && r.ReplyAt >= r.CreatedAt && r.ReplyAt > 0 && !r.Delivered && r.ReplyAttempt == ""
}

// A restored workspace snapshot is not a substitute for the live relay ledger.
func replySourceHold(a hqworkspace.IntentAttempt, db hqworkspace.Database) string {
	target := a.Intent.Target
	if seq := workspaceEventSequence(); seq != a.EventCursor {
		rows, gap := workspaceReadEvents(a.EventCursor)
		if gap || seq < a.EventCursor {
			return "HQ must reconcile the event history gap"
		}
		for _, r := range rows {
			if r.Event == "UserPromptSubmit" && r.Origin == events.OriginInstruction && r.AgentSession == target.SessionID && (r.HomeID == "" || r.HomeID == target.HomeID) {
				return "HQ must review the newer instruction in the target conversation"
			}
		}
	}
	found := false
	for _, r := range workspaceRelayRecords() {
		if r.Agent != target.Agent || r.HomeID != target.HomeID || r.ThreadID != target.SessionID {
			continue
		}
		if r.For == "user" && (r.State == "pending" || r.State == "claimed") && r.Blocking {
			return "awaiting user decision: " + r.ID
		}
		if r.Kind == "report" && r.For != "user" && r.CreatedAt >= a.Intent.At {
			if p := workspaceRelayDriveProof(r.Body, a.MatterID); p != nil && p.ActionID == a.Intent.ID && p.StepID == a.StepID && p.Ref == a.Intent.Ref {
				return "HQ must reconcile the original source drive receipt: " + r.ID
			}
		}
		if r.ID != a.ReplyID {
			continue
		}
		found = eligibleWorkspaceReply(r) && r.ReplyAt == db.Reconciliations["inbox:"+a.ReplyID].Observation.OccurredAt && r.CreatedAt == db.Reconciliations["relay-"+a.ReplyID].Observation.OccurredAt
		if r.Notice != nil && r.FetchedAt == 0 && r.Notice.State != "accepted" {
			return "HQ must reconcile the unconfirmed active-turn hint"
		}
	}
	if !found {
		return "inbox source unavailable or changed"
	}
	return ""
}
