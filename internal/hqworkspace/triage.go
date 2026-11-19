package hqworkspace

import (
	"errors"
	"slices"
	"strings"
)

func (o Observation) NeedsTriage() bool {
	return strings.HasPrefix(o.Kind, "triage-unassigned:") ||
		strings.HasPrefix(o.Kind, "triage-ambiguous:")
}

// sourceKind preserves lifecycle checks for observations awaiting attribution.
func (o Observation) sourceKind() string {
	if o.NeedsTriage() {
		_, kind, _ := strings.Cut(o.Kind, ":")
		return kind
	}
	return o.Kind
}

func validateTriageAssessment(a Assessment) error {
	if _, err := text(a.ReviewRef, 2048); err != nil {
		return errors.New("triage requires a citation to the reviewed evidence")
	}
	switch a.Decision {
	case "handoff":
		if a.MatterID == "" || a.Revision == 0 {
			return errors.New("handoff requires a current matter and revision")
		}
	case "no-followup", "deferred":
		if a.MatterID != "" || a.Revision != 0 {
			return errors.New("only handoff selects a matter for triage")
		}
	default:
		return errors.New("triage requires handoff, no-followup or a sourced future check")
	}
	return nil
}

func handoffTriage(d *Database, o Observation, m Matter, now int64) (string, error) {
	if !slices.Contains(ObservationMatters(*d, o.Conversation, o.OccurredAt), m.ID) {
		return "", errors.New("handoff requires the exact execution conversation and source interval")
	}
	g := GoalResponsibility(*d, m, now)
	switch g.State {
	case "verify", "owes-hq", "held", "waiting-user":
		// Keep the existing plan, hold and delivery reservations. The receipt
		// only moves the duty to the goal that HQ explicitly verified.
	default:
		return "", errors.New("handoff requires a current HQ obligation or a verified hold")
	}
	syncGoal(d, m, now)
	q := d.Reconciliations[g.FollowupID]
	if goalOwed(g) && (q.State == "deferred" || q.State == "leased") {
		q.State = "pending"
		q.NextCheck = 0
		q.LeaseUntil = 0
		q.Generation++
		d.Reconciliations[g.FollowupID] = q
		changed(d, now, "triage-recheck", m.ID, g.FollowupID, o.ID)
	}
	return g.FollowupID, nil
}
