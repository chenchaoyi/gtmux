package hqworkspace

import (
	"errors"
	"reflect"
	"strings"
)

// ArmReply reserves an already reviewed action ID. The ordinary drive command
// will return this reservation rather than issuing a competing submission.
func (s *Store) ArmReply(matter, action, reply string, revision uint64, at int64) (IntentAttempt, error) {
	var out IntentAttempt
	err := s.transaction(func(d *Database) error {
		if !validID(reply) || revision == 0 {
			return ErrConflict
		}
		if old, ok := d.IntentAttempts[action]; ok {
			if old.MatterID != matter || old.ReplyID != reply {
				return ErrConflict
			}
			out = old
			if old.Revision == revision {
				return nil
			}
			if old.State != "armed" {
				return ErrConflict
			}
			// HQ may revalidate an unsent reservation at a new current revision.
			out.Revision = revision
			out.EventCursor = d.ObservationCursors["events"]
			if why := ReplyBlockReason(*d, out, at); why != "" {
				return errors.New(why)
			}
			out.Evidence = ""
			d.IntentAttempts[action] = out
			changed(d, at, "reply-continuation-reviewed", matter, action, reply)
			return nil
		}
		m, err := getMatter(d, matter, revision)
		if err != nil {
			return err
		}
		p := m.Responsibility
		if p == nil || p.Intent == nil || p.Intent.ID != action {
			return errors.New("HQ must first save the reviewed responsibility and exact action")
		}
		for _, prior := range d.IntentAttempts {
			if prior.ReplyID == reply {
				return errors.New("this reply already has a reserved continuation")
			}
		}
		for _, prior := range d.Assessments {
			plan := prior.Assessment.Responsibility
			if prior.Assessment.MatterID == matter && plan != nil && plan.Intent != nil && plan.StepID == p.StepID && plan.AuthorityRef == p.AuthorityRef {
				if _, used := d.IntentAttempts[plan.Intent.ID]; used {
					return errors.New("reconcile the original attempt for this authorized step")
				}
			}
		}
		out = IntentAttempt{Intent: *p.Intent, StepID: p.StepID, MatterID: matter, ReplyID: reply, Revision: revision, EventCursor: d.ObservationCursors["events"], State: "armed", At: at}
		if why := ReplyBlockReason(*d, out, at); why != "" {
			return errors.New(why)
		}
		d.IntentAttempts[action] = out
		changed(d, at, "reply-continuation-armed", matter, action, reply)
		return nil
	})
	return out, err
}

// Revalidate source membership at request, reply and delivery, plus the saved
// user instruction. An unseen or superseding instruction requires a fresh review.
func ReplyBlockReason(d Database, a IntentAttempt, at int64) string {
	m := d.Matters[a.MatterID]
	p := m.Responsibility
	if m.State != "open" || m.UserInput != nil || m.MergedInto != "" || p == nil || p.Intent == nil || p.Hold != nil || p.ReviewRequired || !reflect.DeepEqual(*p.Intent, a.Intent) || p.StepID != a.StepID || m.Revision != a.Revision {
		return "HQ must review the changed matter, question or responsibility"
	}
	if p.Drive != nil {
		return "HQ must reconcile the existing drive receipt"
	}
	target := a.Intent.Target
	id, _ := MatchObservation(d, target, at)
	if target.Agent != "codex" || target.HomeID == "" || target.SessionID == "" || id != m.ID {
		return "HQ must verify the current exact execution member"
	}
	msg := d.Messages[strings.TrimPrefix(p.AuthorityRef, "message:")]
	if p.AuthorityRef != "message:"+msg.ID || msg.ID == "" || msg.Speaker != "user" || msg.Kind != "instruction" || msg.MatterID != m.ID || msg.ReplyTo != "" || msg.At > a.Intent.At || msg.Delivery == "cancelled" {
		return "HQ must verify the original user instruction for this matter"
	}
	for _, later := range d.Messages {
		if later.MatterID == m.ID && later.Speaker == "user" && later.Seq > msg.Seq {
			return "HQ must review the newer user instruction"
		}
	}
	o := d.Reconciliations["inbox:"+a.ReplyID].Observation
	request := d.Reconciliations["relay-"+a.ReplyID].Observation
	for _, fact := range []Observation{request, o} {
		member, _ := MatchObservation(d, target, fact.OccurredAt)
		if fact.Ref != "relay:"+a.ReplyID || !fact.Conversation.Same(target) || member != m.ID || fact.MatterID != m.ID || fact.OccurredAt > at {
			return "HQ must verify the inbox reply and its exact source"
		}
	}
	continuous := false
	for _, member := range m.Participants {
		if member.Role == "execution" && member.Conversation.Same(target) && member.From <= request.OccurredAt && (member.Until == 0 || member.Until >= at) && (member.RemovedAt == 0 || member.RemovedAt > at) {
			continuous = true
			break
		}
	}
	if !continuous {
		return "HQ must review the interrupted execution membership"
	}
	if o.Kind != "relay-inbox" || o.OccurredAt < request.OccurredAt {
		return "HQ must verify an eligible inbox reply"
	}
	return ""
}

// Remember a preflight blocker without consuming the one-shot submission.
func (s *Store) ReplyBlocker(a IntentAttempt, why string, at int64) error {
	return s.transaction(func(d *Database) error {
		old := d.IntentAttempts[a.Intent.ID]
		if old.State != "armed" || !reflect.DeepEqual(old, a) || old.Evidence == why {
			return nil
		}
		old.Evidence = why
		d.IntentAttempts[a.Intent.ID] = old
		changed(d, at, "reply-continuation-blocked", a.MatterID, a.Intent.ID, why)
		return nil
	})
}

func replyResponsibility(d Database, a IntentAttempt, at int64) GoalStatus {
	observationID := "inbox:" + a.ReplyID
	stage := "reply " + a.ReplyID + " awaits source fetch"
	if _, ok := d.Reconciliations["fetched:"+a.ReplyID]; ok {
		observationID = "fetched:" + a.ReplyID
		stage = "reply " + a.ReplyID + " fetched; step " + a.StepID + " awaits execution evidence"
	}
	reason := a.Evidence
	switch a.State {
	case "armed":
		if why := ReplyBlockReason(d, a, at); why != "" {
			reason = why
		}
		if reason == "" {
			reason = "awaiting guarded idle delivery"
		}
	case "submitting", "uncertain":
		reason = "HQ must reconcile the original submission before any further action"
	case "held":
		reason = "HQ must inspect the refused delivery: " + reason
	case "external":
		reason = "HQ must reconcile source receipt " + a.Evidence
	case "queued", "landed":
		reason = "reminder submitted; HQ must check the source receipt"
	}
	return GoalStatus{State: "owes-hq", Reason: stage + "; " + reason, ObservationID: observationID}
}

func replyExecutionProof(o Observation) bool {
	p := o.DriveProof
	return o.Kind == "relay-report" && p != nil && (p.State == "progress" || p.State == "native" || p.State == "background") && strings.TrimSpace(p.Evidence) != "" && len(p.Evidence) <= 2048
}

// Surface an unassessed receipt even before HQ has verified a continuation.
// Association identifies who owes a review, never a permission to send.
func pendingReply(d Database, m Matter) Observation {
	var latest Observation
	for _, q := range d.Reconciliations {
		o := q.Observation
		if o.MatterID != m.ID || (o.Kind != "relay-inbox" && o.Kind != "relay-fetched") || (q.State != "pending" && q.State != "leased" && q.State != "deferred") {
			continue
		}
		if o.OccurredAt > latest.OccurredAt || o.OccurredAt == latest.OccurredAt && o.ID > latest.ID {
			latest = o
		}
	}
	return latest
}
