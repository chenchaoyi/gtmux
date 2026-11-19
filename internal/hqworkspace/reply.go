package hqworkspace

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
)

// UserEvent joins HQ's explicit review of the original user instruction to a
// retained journal record. No caller-supplied prompt body participates in the join.
type UserEvent struct {
	Seq    int64        `json:"seq"`
	At     int64        `json:"at"`
	Source Conversation `json:"source"`
}

// ArmReply binds an existing action. A prior transport/external attempt can
// gain a reply reference for reconciliation, but can never become armed again.
func (s *Store) ArmReply(matter, action, reply string, revision uint64, at int64, event *UserEvent) (IntentAttempt, error) {
	var out IntentAttempt
	err := s.transaction(func(d *Database) error {
		if !validID(reply) || revision == 0 {
			return ErrConflict
		}
		old, used := d.IntentAttempts[action]
		if used && (old.MatterID != matter || old.ReplyID != "" && old.ReplyID != reply) {
			return ErrConflict
		}
		if used && old.ReplyID == reply && old.Revision == revision {
			out = old
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
		for id, prior := range d.IntentAttempts {
			if id != action && prior.ReplyID == reply {
				return errors.New("this reply already has a reserved continuation")
			}
		}
		var review AssessmentReceipt
		for _, r := range d.Assessments {
			plan := r.Assessment.Responsibility
			if r.Assessment.MatterID != matter || plan == nil || plan.Intent == nil || plan.StepID != p.StepID || plan.AuthorityRef != p.AuthorityRef {
				continue
			}
			if plan.Intent.ID != action {
				if _, exists := d.IntentAttempts[plan.Intent.ID]; exists {
					return errors.New("reconcile the original attempt for this authorized step")
				}
			} else if reflect.DeepEqual(*plan.Intent, *p.Intent) && r.AfterRevision > review.AfterRevision {
				review = r
			}
		}
		if review.ReviewedEventCursor == nil {
			return errors.New("HQ must review the original intent against the current event history")
		}
		out = old
		if !used {
			out = IntentAttempt{Intent: *p.Intent, StepID: p.StepID, MatterID: matter, State: "armed", At: at}
		} else if old.State == "submitting" {
			return errors.New("HQ must reconcile the original in-flight submission")
		}
		out.ReplyID, out.Revision, out.EventCursor = reply, revision, *review.ReviewedEventCursor
		if event != nil {
			out.UserEvent = event
		}
		if why := ReplyBlockReason(*d, out, at); why != "" {
			return errors.New(why)
		}
		if out.State == "armed" {
			out.Evidence = ""
		}
		d.IntentAttempts[action] = out
		changed(d, at, "reply-continuation-bound", matter, action, reply)
		return nil
	})
	return out, err
}

func clearedAnswer(d Database, answer Message) bool {
	question := d.Messages[answer.ReplyTo]
	if question.UserInput == nil || question.MatterID != answer.MatterID || question.Seq >= answer.Seq {
		return false
	}
	for _, clear := range d.Messages {
		// ClearInput cites the HQ question. Ordinary report replies may only
		// cite a user message, and carry a pending-reconciliation projection.
		if clear.MatterID == answer.MatterID && clear.Speaker == "hq" && clear.Kind == "progress" && clear.Projection == "" && clear.ReplyTo == question.ID && clear.TargetRevision >= question.UserInput.Revision && clear.Seq > answer.Seq {
			return true
		}
	}
	return false
}

func replyAuthority(d Database, m Matter, a IntentAttempt) bool {
	ref := m.Responsibility.AuthorityRef
	msg := d.Messages[strings.TrimPrefix(ref, "message:")]
	if e := a.UserEvent; e != nil {
		member, _ := MatchObservation(d, e.Source, e.At)
		if ref != "events:"+strconv.FormatInt(e.Seq, 10) || e.Seq <= 0 || e.Seq > a.EventCursor || e.At <= 0 || e.At > a.Intent.At || !e.Source.Same(a.Intent.Target) || member != m.ID {
			return false
		}
	} else if msg.ID == "" || ref != "message:"+msg.ID || msg.Speaker != "user" || msg.Kind != "instruction" || msg.MatterID != m.ID || msg.At > a.Intent.At || msg.Delivery == "cancelled" || msg.ReplyTo != "" && !clearedAnswer(d, msg) {
		return false
	}
	for _, later := range d.Messages {
		if later.MatterID != m.ID || later.Speaker != "user" {
			continue
		}
		if a.UserEvent != nil && later.At >= a.UserEvent.At || a.UserEvent == nil && later.Seq > msg.Seq {
			return false
		}
	}
	return true
}

// Revalidate source membership at request, reply and delivery, plus the saved
// user instruction. An unseen or superseding instruction requires a fresh review.
func ReplyBlockReason(d Database, a IntentAttempt, at int64) string {
	m := d.Matters[a.MatterID]
	p := m.Responsibility
	if m.State != "open" || m.UserInput != nil || m.MergedInto != "" || p == nil || p.Intent == nil || p.Hold != nil || p.ReviewRequired || !reflect.DeepEqual(*p.Intent, a.Intent) || p.StepID != a.StepID || m.Revision != a.Revision {
		return "HQ must review the changed matter, question or responsibility"
	}
	if p.Drive != nil && (a.State == "armed" || a.State == "submitting") {
		return "HQ must reconcile the existing drive receipt"
	}
	target := a.Intent.Target
	id, _ := MatchObservation(d, target, at)
	if target.Agent != "codex" || target.HomeID == "" || target.SessionID == "" || id != m.ID {
		return "HQ must verify the current exact execution member"
	}
	if !replyAuthority(d, m, a) {
		return "HQ must verify the latest user instruction or cleared answer for this matter"
	}
	for _, row := range d.Reconciliations {
		o := row.Observation
		if o.Kind != "UserPromptSubmit" || !o.Conversation.Same(target) {
			continue
		}
		seq, _ := strconv.ParseInt(strings.TrimPrefix(o.Ref, "events:"), 10, 64)
		if seq > a.EventCursor || a.UserEvent == nil && o.OccurredAt > d.Messages[strings.TrimPrefix(p.AuthorityRef, "message:")].At {
			return "HQ must review the newer instruction in the target conversation"
		}
	}
	if replyDischarged(d, a.ReplyID) {
		return "HQ recorded that this receipt needs no follow-up"
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

func replyDischarged(d Database, id string) bool {
	for _, stage := range []string{"inbox:", "fetched:"} {
		q := d.Reconciliations[stage+id]
		if d.Assessments[q.ReceiptID].Assessment.Decision == "no-followup" {
			return true
		}
	}
	return false
}

// ReplyObservations preserves each concrete stage even after HQ merely notes
// the receipt. The action ID, if assigned, remains in the intent ledger.
func ReplyObservations(d Database) map[string]Observation {
	out := map[string]Observation{}
	for _, q := range d.Reconciliations {
		o := q.Observation
		if o.Kind != "relay-inbox" {
			continue
		}
		id := strings.TrimPrefix(o.Ref, "relay:")
		if replyDischarged(d, id) {
			continue
		}
		if f, ok := d.Reconciliations["fetched:"+id]; ok {
			o = f.Observation
		}
		out[id] = o
	}
	return out
}

// Surface a receipt even before HQ has verified a continuation.
// Association identifies who owes a review, never a permission to send.
func pendingReply(d Database, m Matter) Observation {
	var latest Observation
	for _, q := range d.Reconciliations {
		o := q.Observation
		if o.MatterID != m.ID || (o.Kind != "relay-inbox" && o.Kind != "relay-fetched") || q.State == "superseded" {
			continue
		}
		id := strings.TrimPrefix(o.Ref, "relay:")
		if replyDischarged(d, id) {
			continue
		}
		if p := m.Responsibility; p != nil && p.Intent != nil && (!o.Conversation.Same(p.Intent.Target) || o.OccurredAt < p.Intent.At) {
			continue
		}
		if o.OccurredAt > latest.OccurredAt || o.OccurredAt == latest.OccurredAt && o.ID > latest.ID {
			latest = o
		}
	}
	return latest
}
