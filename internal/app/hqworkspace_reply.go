package app

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
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

// The general observation stream deliberately skips unenrolled conversations.
func workspaceMessageHistoryHold(d hqworkspace.Database, a hqworkspace.IntentAttempt) string {
	m := hqworkspace.ReplyAuthorityMessage(d, a)
	if m.ID == "" {
		return "HQ must verify the original workspace instruction"
	}
	floor := int64(0)
	policy := m.RecordingPolicy
	if m.EventCursor != nil {
		floor = *m.EventCursor
	} else {
		// Compare durable workspace revisions. A scan can start before the
		// user message and commit its later producer rows after the message.
		for _, change := range d.Changes {
			if change.Kind != "observation" || change.Seq >= m.Seq {
				continue
			}
			o := d.Reconciliations[change.Target].Observation
			seq, e := strconv.ParseInt(strings.TrimPrefix(o.Ref, "events:"), 10, 64)
			if e == nil && seq > floor && o.ID == "event-"+strconv.FormatInt(seq, 10) {
				floor = seq
				policy = o.RecordingPolicy
			}
		}
	}
	if floor <= 0 {
		return "HQ must verify a fresh user instruction with an initialized event journal"
	}
	rows, high, err := events.VerifiedSince(floor)
	if !events.RecordingPolicyCovers(policy, m.At) {
		return "HQ must verify a fresh user instruction after the recording policy changed"
	}
	if err != nil || high < a.EventCursor {
		return "HQ must reconcile the incomplete or unreadable target event history"
	}
	target := a.Intent.Target
	for _, r := range rows {
		if !hqworkspace.MessageHistoryAfter(m, r.Seq, r.Ts) || !replyTargetInstruction(r, target) {
			continue
		}
		return "HQ must reconcile the later or unordered instruction in the target conversation"
	}
	return ""
}

func replyTargetInstruction(r events.Record, target hqworkspace.Conversation) bool {
	if r.Event != "UserPromptSubmit" || r.Origin != events.OriginInstruction || r.AgentSession != "" && r.AgentSession != target.SessionID || r.HomeID != "" && r.HomeID != target.HomeID {
		return false
	}
	agent := agents.KeyForLabel(r.Agent)
	return agent == "" || agent == target.Agent
}

// A restored workspace snapshot is not a substitute for the live relay ledger.
func replySourceHold(a hqworkspace.IntentAttempt, db hqworkspace.Database) string {
	target := a.Intent.Target
	if a.UserEvent != nil {
		// The send audit can land after the prompt and after HQ arms the action.
		e, err := workspaceVerifiedUserEvent("events:" + strconv.FormatInt(a.UserEvent.Seq, 10))
		if err != nil || e.At != a.UserEvent.At || !e.Source.Same(a.UserEvent.Source) {
			return "HQ must reconcile the original user event and its authorship"
		}
	}
	if a.UserEvent == nil {
		if why := workspaceMessageHistoryHold(db, a); why != "" {
			return why
		}
	}
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
	liveReports := map[string]bool{}
	for _, r := range workspaceRelayRecords() {
		if r.Agent != target.Agent || r.HomeID != target.HomeID || r.ThreadID != target.SessionID {
			continue
		}
		if r.Kind == "report" {
			liveReports["relay:"+r.ID] = true
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
	// Legacy unassigned reports do not retain their recipient separately.
	// If the source record disappeared, do not guess it was safe to repeat.
	for _, q := range db.Reconciliations {
		o := q.Observation
		if o.Kind != "relay-report" || liveReports[o.Ref] || !o.Conversation.Same(target) || o.OccurredAt < a.Intent.At {
			continue
		}
		if p := workspaceRelayDriveProof(o.Body, a.MatterID); p != nil && p.ActionID == a.Intent.ID && p.StepID == a.StepID && p.Ref == a.Intent.Ref {
			return "HQ must reconcile the historical source report: " + o.Ref
		}
	}
	if !found {
		return "inbox source unavailable or changed"
	}
	return ""
}

// Re-read source metadata for reports predating enrollment. An unassigned raw
// observation alone does not say whether its recipient was HQ or the user.
func workspaceReplyReports(matter, action string) []hqworkspace.Observation {
	var out []hqworkspace.Observation
	for _, r := range workspaceRelayRecords() {
		if r.For == "user" || r.Kind != "report" {
			continue
		}
		proof := workspaceRelayDriveProof(r.Body, matter)
		if proof == nil || proof.ActionID != action {
			continue
		}
		out = append(out, hqworkspace.Observation{ID: "relay-" + r.ID, Kind: "relay-report", MatterID: matter, Ref: "relay:" + r.ID, Conversation: hqworkspace.Conversation{Agent: r.Agent, HomeID: r.HomeID, SessionID: r.ThreadID}, OccurredAt: r.CreatedAt, Body: r.Body, DriveProof: proof})
	}
	return out
}

// The operator attests authorship after inspecting the original source. The
// journal verifies event kind, exact source and time, and vetoes known sends.
// An unattributed prompt, by itself, is never user authorization.
func workspaceVerifiedUserEvent(ref string) (*hqworkspace.UserEvent, error) {
	seq, err := strconv.ParseInt(strings.TrimPrefix(ref, "events:"), 10, 64)
	if err != nil || seq <= 0 || ref != "events:"+strconv.FormatInt(seq, 10) {
		return nil, errors.New("verified_user_event requires an exact events:N reference")
	}
	rows, err := events.VerifiedInstructionHistory(seq)
	if err != nil {
		return nil, fmt.Errorf("HQ must reconcile the original user event history: %w", err)
	}
	for _, r := range rows {
		if r.Seq != seq {
			continue
		}
		if !events.RecordingPolicyCovers(r.RecordingPolicy, r.Ts) {
			return nil, errors.New("HQ must verify a fresh user instruction after the recording policy changed")
		}
		if r.Event != "UserPromptSubmit" || r.Origin != events.OriginInstruction || r.HomeID == "" || r.AgentSession == "" || r.Ts <= 0 || r.Actor != "" && r.Actor != "user" || events.AuthorOf(rows)[seq] != "" {
			break
		}
		agent := agents.KeyForLabel(r.Agent)
		if agent == "" {
			agent = r.Agent
		}
		for _, later := range rows {
			if (later.Seq > seq || later.Seq == 0 && (later.Ts <= 0 || later.Ts >= r.Ts)) && replyTargetInstruction(later, hqworkspace.Conversation{Agent: agent, HomeID: r.HomeID, SessionID: r.AgentSession}) {
				return nil, errors.New("HQ must verify the newer instruction in the target conversation")
			}
		}
		return &hqworkspace.UserEvent{Seq: seq, At: r.Ts, Source: hqworkspace.Conversation{Agent: agent, HomeID: r.HomeID, SessionID: r.AgentSession}}, nil
	}
	return nil, errors.New("original direct user event is missing, machine-delivered or lacks exact identity")
}
