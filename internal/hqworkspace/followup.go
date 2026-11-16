package hqworkspace

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Responsibility is an explicit, reviewed next step. Citations never confer
// permissions. No scheduler in this package executes the recorded intent.
type Responsibility struct {
	ReviewRequired bool          `json:"review_required,omitempty"`
	StepID         string        `json:"step_id"`
	Action         string        `json:"action"`
	AuthorityRef   string        `json:"authority_ref"`
	Intent         *ActionIntent `json:"intent,omitempty"`
	Drive          *DriveRef     `json:"drive,omitempty"`
	Hold           *GoalHold     `json:"hold,omitempty"`
}

type ActionIntent struct {
	ID     string       `json:"id"`
	Ref    string       `json:"ref"` // exact task/message/native-goal identity
	Target Conversation `json:"target"`
	At     int64        `json:"at"`
}

type DriveRef struct {
	ObservationID string `json:"observation_id"`
}

// DriveProof is supplied by a source adapter, never synthesized from idle/working
// or an unstructured report. Ref and action bind receipts to one saved intent.
type DriveProof struct {
	StepID    string `json:"step_id"`
	ActionID  string `json:"action_id"`
	Ref       string `json:"ref"`
	State     string `json:"state"` // queued | received | progress | background | native
	NextCheck int64  `json:"next_check"`
}

type GoalHold struct {
	Kind      string `json:"kind"` // paused | refused | external
	SourceRef string `json:"source_ref"`
	Condition string `json:"condition"`
	NextCheck int64  `json:"next_check,omitempty"`
}

type GoalStatus struct {
	FollowupID    string `json:"followup_id"`
	State         string `json:"state"` // verify | owes-hq | queued | driving | waiting-user | held | terminal
	Reason        string `json:"reason"`
	ObservationID string `json:"observation_id,omitempty"`
	NextCheck     int64  `json:"next_check,omitempty"`
}

func goalKey(m Matter) string {
	step, action, authority := "verify", "", ""
	if p := m.Responsibility; p != nil {
		step, action, authority = p.StepID, p.Action, p.AuthorityRef
	}
	b, _ := json.Marshal([]string{m.ID, step, action, authority})
	return fmt.Sprintf("goal-%x", sha256.Sum256(b))
}

func GoalFollowupID(m Matter) string { return goalKey(m) }

func GoalResponsibility(d Database, m Matter, now int64) (out GoalStatus) {
	defer func() { out.FollowupID = goalKey(m) }()
	if m.MergedInto != "" || m.State == "completed" || m.State == "closed" {
		return GoalStatus{State: "terminal", Reason: "explicit terminal state"}
	}
	if q := m.UserInput; q != nil && m.State == "needs-input" && q.AnswerID == "" {
		return GoalStatus{State: "waiting-user", Reason: q.Prompt}
	}
	if m.State == "needs-input" && m.UserInput == nil {
		return GoalStatus{State: "verify", Reason: "HQ must clarify the legacy unstructured question"}
	}
	p := m.Responsibility
	if p == nil {
		return GoalStatus{State: "verify", Reason: "HQ must verify authority and next responsibility; legacy records do not authorize execution"}
	}
	if h := p.Hold; h != nil {
		if h.Kind == "paused" || h.Kind == "refused" {
			return GoalStatus{State: "held", Reason: h.Kind + ": " + h.Condition}
		}
		if h.Kind == "external" && h.NextCheck > now {
			return GoalStatus{State: "held", Reason: h.Condition, NextCheck: h.NextCheck}
		}
		return GoalStatus{State: "owes-hq", Reason: "HQ must recheck the external condition; elapsed time is not permission"}
	}
	if p.ReviewRequired {
		return GoalStatus{State: "verify", Reason: "restored responsibility requires explicit HQ review before any action"}
	}
	if m.UserInput != nil {
		return GoalStatus{State: "owes-hq", Reason: "HQ must verify the saved answer and clear the exact question"}
	}
	if p.Intent == nil || p.Drive == nil {
		return GoalStatus{State: "owes-hq", Reason: "next step has no verified drive; check existing intent before any delivery"}
	}
	if a, ok := d.IntentAttempts[p.Intent.ID]; ok && (a.State == "uncertain" || a.State == "submitting" || a.State == "held") && p.Drive.ObservationID == "drive-"+p.Intent.ID {
		return GoalStatus{State: "verify", Reason: "submission outcome is unknown; verify original receipts before any new action"}
	}
	q, ok := d.Reconciliations[p.Drive.ObservationID]
	o := q.Observation
	proof := o.DriveProof
	i := p.Intent
	if !ok || !driveEvidenceMatches(d, m, o, now) {
		return GoalStatus{State: "verify", Reason: "drive evidence does not cover this step, action and exact conversation"}
	}
	current, _ := MatchObservation(d, i.Target, now)
	if current != m.ID {
		return GoalStatus{State: "verify", Reason: "drive membership or interval is unavailable or ambiguous"}
	}
	if proof.NextCheck <= now {
		return GoalStatus{State: "verify", Reason: "drive check is due; missing probes never authorize replay", ObservationID: o.ID}
	}
	switch proof.State {
	case "queued":
		return GoalStatus{State: "queued", Reason: "reliably queued, awaiting receipt; do not resend", ObservationID: o.ID, NextCheck: proof.NextCheck}
	case "received", "progress", "background", "native":
		return GoalStatus{State: "driving", Reason: "verified " + proof.State + " receipt for this step", ObservationID: o.ID, NextCheck: proof.NextCheck}
	}
	return GoalStatus{State: "verify", Reason: "unknown drive verdict"}
}

// Historical attribution is independent of today's probe deadline and membership.
// A late report can prove this action happened without proving it is still active.
func driveEvidenceMatches(d Database, m Matter, o Observation, now int64) bool {
	p := m.Responsibility
	proof := o.DriveProof
	if p == nil || p.Intent == nil || proof == nil {
		return false
	}
	i := p.Intent
	if proof.StepID != p.StepID || proof.ActionID != i.ID || proof.Ref != i.Ref || !o.Conversation.Same(i.Target) || CanonicalMatter(d, o.MatterID) != m.ID || o.OccurredAt < i.At || o.OccurredAt > now || proof.NextCheck <= o.OccurredAt {
		return false
	}
	switch proof.State {
	case "queued", "received", "progress", "background", "native":
	default:
		return false
	}
	id, _ := MatchObservation(d, o.Conversation, o.OccurredAt)
	return id == m.ID
}

func goalOwed(g GoalStatus) bool { return g.State == "verify" || g.State == "owes-hq" }

func validateResponsibility(d Database, m Matter, p Responsibility, now int64) error {
	if !validID(p.StepID) {
		return errors.New("stable step_id required")
	}
	for _, v := range []string{p.Action, p.AuthorityRef} {
		if _, e := text(v, 2048); e != nil {
			return e
		}
	}
	if p.Hold != nil && (p.Intent != nil || p.Drive != nil) {
		return errors.New("hold and drive intent are mutually exclusive")
	}
	if i := p.Intent; i != nil {
		if !validID(i.ID) || i.Target.Agent == "" || i.Target.SessionID == "" || i.At <= 0 || i.At > now {
			return errors.New("intent needs stable action ID, exact target and timestamp")
		}
		if _, e := text(i.Ref, 2048); e != nil {
			return e
		}
		id, _ := MatchObservation(d, i.Target, now)
		if id != m.ID {
			return errors.New("intent target is not unambiguously associated with this matter")
		}
	}
	if p.Drive != nil && p.Intent == nil {
		return errors.New("drive requires a previously saved intent")
	}
	old := m.Responsibility
	if p.Intent != nil {
		if old != nil && old.Intent != nil && old.Intent.ID != p.Intent.ID && !goalOwed(GoalResponsibility(d, m, now)) {
			return errors.New("existing queued or running intent must be reconciled before replacing it")
		}
		if attempt, ok := d.IntentAttempts[p.Intent.ID]; ok && (attempt.MatterID != m.ID || attempt.StepID != p.StepID || !reflect.DeepEqual(attempt.Intent, *p.Intent)) {
			return ErrConflict
		}
		// An action ID remains immutable even across step revisions and failed sends.
		for _, r := range d.Assessments {
			if x := r.Assessment.Responsibility; CanonicalMatter(d, r.Assessment.MatterID) == m.ID && sameIntentWork(x, &p) && x.Intent.ID != p.Intent.ID {
				return errors.New("this work already has a saved action identity; a new step label does not authorize replay")
			}
			if x := r.Assessment.Responsibility; r.Assessment.MatterID == m.ID && x != nil && x.Intent != nil && x.StepID == p.StepID && x.Action == p.Action && x.AuthorityRef == p.AuthorityRef && x.Intent.ID != p.Intent.ID {
				return errors.New("the material step already has a saved action ID; verify its original outcome instead of replacing it")
			}
			if x := r.Assessment.Responsibility; x != nil && x.Intent != nil && x.Intent.ID == p.Intent.ID && (!reflect.DeepEqual(x.Intent, p.Intent) || x.StepID != p.StepID || x.Action != p.Action || x.AuthorityRef != p.AuthorityRef) {
				return ErrConflict
			}
		}
	}
	if p.Drive != nil {
		if old == nil || old.Intent == nil || !reflect.DeepEqual(old.Intent, p.Intent) || old.StepID != p.StepID || old.Action != p.Action || old.AuthorityRef != p.AuthorityRef {
			return errors.New("save intent before attaching a receipt")
		}
		candidate := m
		candidate.Responsibility = &p
		g := GoalResponsibility(d, candidate, now)
		if g.State != "queued" && g.State != "driving" {
			return errors.New(g.Reason)
		}
	}
	if h := p.Hold; h != nil {
		if h.Kind != "paused" && h.Kind != "refused" && h.Kind != "external" {
			return errors.New("invalid hold kind")
		}
		for _, v := range []string{h.SourceRef, h.Condition} {
			if _, e := text(v, 2048); e != nil {
				return e
			}
		}
		if h.Kind == "external" && h.NextCheck <= now {
			return errors.New("external hold requires a future condition check")
		}
		if h.Kind != "external" && h.NextCheck != 0 {
			return errors.New("user pause/refusal cannot expire")
		}
	}
	return nil
}

// ScanGoals resumes a bounded persistent sweep even when there are no new events.
// It only records HQ debt. It cannot send, spawn, answer a question or resume work.
func (s *Store) ScanGoals(now int64, limit int) error {
	if limit < 1 || limit > 64 {
		limit = 64
	}
	return s.transaction(func(d *Database) error {
		ids := []string{}
		for id := range d.Matters {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		start := sort.SearchStrings(ids, d.GoalScanAfter)
		for start < len(ids) && ids[start] <= d.GoalScanAfter {
			start++
		}
		if start == len(ids) {
			start = 0
		}
		end := min(start+limit, len(ids))
		for _, id := range ids[start:end] {
			syncGoal(d, d.Matters[id], now)
		}
		cursor := ""
		if end < len(ids) && end > start {
			cursor = ids[end-1]
		}
		if d.GoalScanAfter != cursor {
			d.GoalScanAfter = cursor
			changed(d, now, "goal-scan", "", cursor, "")
		}
		return nil
	})
}

func syncGoal(d *Database, m Matter, now int64) {
	key := goalKey(m)
	g := GoalResponsibility(*d, m, now)
	for id, q := range d.Reconciliations {
		if q.Observation.Kind != "goal-followup" || q.Observation.MatterID != m.ID || id == key || q.State == "superseded" {
			continue
		}
		q.State = "superseded"
		q.LeaseUntil = 0
		q.Generation++
		d.Reconciliations[id] = q
		changed(d, now, "goal-superseded", m.ID, id, key)
	}
	q, exists := d.Reconciliations[key]
	if !exists {
		if g.State == "terminal" {
			return
		}
		_ = recordObservation(d, Observation{ID: key, Kind: "goal-followup", MatterID: m.ID, MatterRevision: m.Revision, Ref: "matter:" + m.ID, Body: "HQ must verify the current next responsibility; observation acknowledgement is not goal execution.", OccurredAt: now}, now)
		if !goalOwed(g) {
			q := d.Reconciliations[key]
			q.State = "assessed"
			d.Reconciliations[key] = q
		}
		return
	}
	if q.State == "leased" && q.LeaseUntil > now {
		if q.GoalPremise == goalPremise(*d, m, now) {
			return
		}
		q.State = "pending"
		if !goalOwed(g) {
			q.State = "assessed"
		}
		q.LeaseUntil = 0
		q.Generation++
		q.GoalPremise = ""
		d.Reconciliations[key] = q
		changed(d, now, "goal-premise-changed", m.ID, key, g.State)
		return
	}
	if !goalOwed(g) {
		if q.State != "assessed" {
			q.State = "assessed"
			q.LeaseUntil = 0
			q.Generation++
			d.Reconciliations[key] = q
			changed(d, now, "goal-covered", m.ID, key, g.State)
		}
	} else if q.State == "assessed" || q.State == "conflict" || q.State == "superseded" {
		q.State = "pending"
		q.LeaseUntil = 0
		q.Generation++
		d.Reconciliations[key] = q
		changed(d, now, "goal-uncovered", m.ID, key, g.State)
	}
}

// RemindGoals coalesces goal reminders across process restarts. A reminder is
// never action coverage; repeated misses are explicitly HQ coordination debt.
func (s *Store) RemindGoals(now int64) (count int, repeated bool, err error) {
	err = s.transaction(func(d *Database) error {
		for _, q := range ReconciliationQueue(*d, now) {
			if q.Observation.Kind != "goal-followup" {
				continue
			}
			m, ok := d.Matters[q.Observation.MatterID]
			if !ok || !goalOwed(GoalResponsibility(*d, m, now)) || q.ReminderAt != 0 && now < q.ReminderAt+300 {
				continue
			}
			q.ReminderAt = now
			q.ReminderCount++
			count++
			repeated = repeated || q.ReminderCount >= 3
			d.Reconciliations[q.Observation.ID] = q
			changed(d, now, "goal-reminder", m.ID, q.Observation.ID, "HQ coordination remains due")
		}
		return nil
	})
	return
}

// Step labels, citation wording and pane hints do not change the physical work.
// The work reference plus the normalized instruction bind a recovery to
// its original action; business changes still need an explicit new instruction.
func sameIntentWork(a, b *Responsibility) bool {
	return a != nil && b != nil && a.Intent != nil && b.Intent != nil && strings.TrimSpace(a.Intent.Ref) == strings.TrimSpace(b.Intent.Ref) && strings.Join(strings.Fields(a.Action), " ") == strings.Join(strings.Fields(b.Action), " ")
}

// A claim snapshots only action/hold/question/coverage facts. Routine scans and
// observation reads do not renew or revoke it; time matters only when coverage
// actually expires. Matter revision independently fences all explicit edits.
func goalPremise(d Database, m Matter, now int64) string {
	b, _ := json.Marshal(struct {
		State          string
		Responsibility *Responsibility
		Question       *UserInput
		Coverage       GoalStatus
	}{m.State, m.Responsibility, m.UserInput, GoalResponsibility(d, m, now)})
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
