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

type ReplyReview struct {
	MessageHold  func(Database, IntentAttempt) string
	AuthorityRef string
	UserEvent    *UserEvent
	Reports      []Observation
}

// ArmReply binds an existing action. A prior transport/external attempt can
// gain a reply reference for reconciliation, but can never become armed again.
func (s *Store) ArmReply(matter, action, reply string, revision uint64, at int64, evidence ReplyReview) (IntentAttempt, error) {
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
			if evidence.AuthorityRef != "" && evidence.AuthorityRef != old.AuthorityRef {
				return ErrConflict
			}
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
		if used && old.ReplyID == "" {
			out.PriorDelivery = true
		}
		// A later user confirmation can supplement a saved action without
		// changing its identity, original scope or any past delivery result.
		out.ReviewedAt = review.At
		if evidence.AuthorityRef != "" {
			out.AuthorityRef = evidence.AuthorityRef
			out.UserEvent = nil
			if msg, ok := d.Messages[strings.TrimPrefix(evidence.AuthorityRef, "message:")]; ok && msg.Seq > review.BeforeRevision {
				return errors.New("HQ must review the current user confirmation for the original action")
			}
		}
		out.ReplyID, out.Revision, out.EventCursor = reply, revision, *review.ReviewedEventCursor
		if evidence.UserEvent != nil {
			out.UserEvent = evidence.UserEvent
		}
		if old.State == "held" && old.Unsent != nil && !old.PriorDelivery {
			if !confirmedAfterRefusal(*d, evidence, *old.Unsent) {
				return errors.New("HQ must verify a new user instruction after the recorded refusal")
			}
			out.State, out.Unsent = "armed", nil
		}
		if why := ReplyBlockReason(*d, out, at); why != "" {
			return errors.New(why)
		}
		if out.State == "armed" {
			out.Evidence = ""
			if out.UserEvent == nil && evidence.MessageHold != nil {
				out.Evidence = evidence.MessageHold(*d, out)
			}
		}
		if d.Reconciliations["relay-"+reply].Observation.MatterID == "" || d.Reconciliations["inbox:"+reply].Observation.MatterID == "" {
			id := "attribution:" + reply
			if prior, ok := d.Reconciliations[id]; ok {
				if prior.Observation.MatterID != matter || !prior.Observation.Conversation.Same(out.Intent.Target) {
					return ErrConflict
				}
			} else {
				observation := Observation{ID: id, Kind: "relay-attribution", MatterID: matter, MatterRevision: revision, Ref: "relay:" + reply, Conversation: out.Intent.Target, OccurredAt: at, Body: "HQ verified the original request and reply against the execution membership interval."}
				if err := recordObservation(d, observation, at); err != nil {
					return err
				}
			}
		}
		d.IntentAttempts[action] = out
		changed(d, at, "reply-continuation-bound", matter, action, reply)
		// A report may already exist when the intent or its source membership
		// becomes known. Reserve the same action before it can reach the pump.
		for _, o := range evidence.Reports {
			raw := d.Reconciliations[o.ID].Observation
			if raw.Kind != "relay-report" || raw.Ref != o.Ref || raw.Body != o.Body || raw.OccurredAt != o.OccurredAt || !raw.Conversation.Same(o.Conversation) || !driveEvidenceMatches(*d, m, o, at) {
				continue
			}
			if raw.MatterID == "" {
				o.ID = "attributed:" + strings.TrimPrefix(raw.Ref, "relay:")
				if err := recordObservation(d, o, at); err != nil {
					return err
				}
			} else if raw.MatterID != matter || !reflect.DeepEqual(raw.DriveProof, o.DriveProof) {
				continue
			}
			attachDrive(d, o, at)
		}
		out = d.IntentAttempts[action]
		return nil
	})
	return out, err
}

func confirmedAfterRefusal(d Database, review ReplyReview, refusal UnsentReceipt) bool {
	if review.AuthorityRef == "" || review.AuthorityRef == refusal.AuthorityRef {
		return false
	}
	if e := review.UserEvent; e != nil {
		if refusal.EventCursor > 0 {
			return e.Seq > refusal.EventCursor && e.At >= refusal.At
		}
		return e.At > refusal.At
	}
	msg := d.Messages[strings.TrimPrefix(review.AuthorityRef, "message:")]
	return msg.Seq > refusal.Revision
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

func replyAuthorityRef(m Matter, a IntentAttempt) string {
	if a.AuthorityRef != "" {
		return a.AuthorityRef
	}
	return m.Responsibility.AuthorityRef
}

func ReplyAuthorityMessage(d Database, a IntentAttempt) Message {
	return d.Messages[strings.TrimPrefix(replyAuthorityRef(d.Matters[a.MatterID], a), "message:")]
}

// A legacy timestamp cannot order two instructions in the same second.
func MessageHistoryAfter(m Message, seq, at int64) bool {
	if m.EventCursor != nil && seq > 0 {
		return seq > *m.EventCursor
	}
	return at <= 0 || at >= m.At
}

func replyAuthority(d Database, m Matter, a IntentAttempt) bool {
	ref := replyAuthorityRef(m, a)
	latest := a.Intent.At
	if a.AuthorityRef != "" {
		latest = a.ReviewedAt
	}
	msg := d.Messages[strings.TrimPrefix(ref, "message:")]
	if e := a.UserEvent; e != nil {
		member, _ := MatchObservation(d, e.Source, e.At)
		if ref != "events:"+strconv.FormatInt(e.Seq, 10) || e.Seq <= 0 || e.Seq > a.EventCursor || e.At <= 0 || e.At > latest || !e.Source.Same(a.Intent.Target) || member != m.ID {
			return false
		}
	} else if msg.ID == "" || ref != "message:"+msg.ID || msg.Speaker != "user" || msg.Kind != "instruction" || msg.MatterID != m.ID || msg.At > latest || msg.Delivery == "cancelled" || msg.ReplyTo != "" && !clearedAnswer(d, msg) {
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
		if seq > a.EventCursor || a.UserEvent == nil && MessageHistoryAfter(ReplyAuthorityMessage(d, a), seq, o.OccurredAt) {
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
		if fact.Ref != "relay:"+a.ReplyID || !fact.Conversation.Same(target) || member != m.ID || fact.MatterID != "" && fact.MatterID != m.ID || fact.OccurredAt > at {
			return "HQ must verify the inbox reply and its exact source"
		}
	}
	from := min(request.OccurredAt, a.Intent.At)
	if a.UserEvent != nil {
		from = min(from, a.UserEvent.At)
	}
	if !uniqueReplyInterval(d, m, target, from, at) {
		return "HQ must review the interrupted or ambiguous execution membership"
	}
	if o.Kind != "relay-inbox" || o.OccurredAt < request.OccurredAt {
		return "HQ must verify an eligible inbox reply"
	}
	return ""
}

// A competing membership can begin and end between the request and the final
// probe. Sampling only their endpoints would miss that use of the same target.
func uniqueReplyInterval(d Database, m Matter, target Conversation, from, to int64) bool {
	covered := false
	for _, candidate := range d.Matters {
		owner := CanonicalMatter(d, candidate.ID)
		for _, member := range candidate.Participants {
			if member.Role != "execution" || !member.Conversation.Same(target) {
				continue
			}
			if owner == m.ID {
				if candidate.ID == m.ID && member.From <= from && (member.Until == 0 || member.Until >= to) && (member.RemovedAt == 0 || member.RemovedAt > to) {
					covered = true
				}
				continue
			}
			first := max(from, member.From)
			if first <= to && (member.Until == 0 || member.Until >= first) && (member.RemovedAt == 0 || member.RemovedAt > first) {
				return false
			}
		}
	}
	return covered
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
		if a.Unsent != nil {
			reason = "HQ must verify fresh user authority after the zero-submission refusal: " + reason
		} else {
			reason = "HQ must inspect the refused delivery: " + reason
		}
	case "external":
		reason = "HQ must reconcile source receipt " + a.Evidence
	case "queued", "landed":
		reason = "reminder submitted; HQ must check the source receipt"
		if a.PriorDelivery {
			reason = "HQ must reconcile the original " + a.State + " drive and check its source receipt"
		}
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
		if o.MatterID == "" {
			o.MatterID = d.Reconciliations["attribution:"+id].Observation.MatterID
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
		if (o.Kind != "relay-inbox" && o.Kind != "relay-fetched") || q.State == "superseded" {
			continue
		}
		id := strings.TrimPrefix(o.Ref, "relay:")
		if o.MatterID == "" {
			// An unassigned raw receipt can nominate a current historical match
			// for HQ review. Only ArmReply records the explicit attribution.
			request := d.Reconciliations["relay-"+id].Observation
			owner, _ := MatchObservation(d, o.Conversation, request.OccurredAt)
			if owner != m.ID {
				continue
			}
			o.MatterID, _ = MatchObservation(d, o.Conversation, o.OccurredAt)
		}
		if o.MatterID != m.ID || replyDischarged(d, id) {
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
