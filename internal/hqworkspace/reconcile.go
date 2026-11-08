package hqworkspace

import (
	"errors"
	"reflect"
	"sort"
)

// Observation is immutable input, not permission to execute or change a goal.
// ID is namespaced by its producer; Ref points to the original durable record.
type Observation struct {
	ID             string       `json:"id"`
	Kind           string       `json:"kind"`
	MatterID       string       `json:"matter_id,omitempty"`
	MatterRevision uint64       `json:"matter_revision,omitempty"`
	Body           string       `json:"body,omitempty"`
	Ref            string       `json:"ref,omitempty"`
	Conversation   Conversation `json:"conversation,omitempty"`
	OccurredAt     int64        `json:"occurred_at"`
}

type Reconciliation struct {
	Observation Observation `json:"observation"`
	State       string      `json:"state"` // pending | leased | assessed | deferred | conflict
	ReceivedAt  int64       `json:"received_at"`
	Owner       string      `json:"owner,omitempty"`
	Generation  uint64      `json:"generation"`
	LeaseUntil  int64       `json:"lease_until,omitempty"`
	NextCheck   int64       `json:"next_check,omitempty"`
	ReceiptID   string      `json:"receipt_id,omitempty"`
}

type Assessment struct {
	ID            string `json:"id"`
	ObservationID string `json:"observation_id"`
	Owner         string `json:"owner"`
	Generation    uint64 `json:"generation"`
	MatterID      string `json:"matter_id,omitempty"`
	Revision      uint64 `json:"revision,omitempty"`
	Decision      string `json:"decision"` // noted | transition | deferred | conflict
	State         string `json:"state,omitempty"`
	Reason        string `json:"reason"`
	NextCheck     int64  `json:"next_check,omitempty"`
}

type AssessmentReceipt struct {
	Assessment     Assessment `json:"assessment"`
	At             int64      `json:"at"`
	BeforeRevision uint64     `json:"before_revision,omitempty"`
	AfterRevision  uint64     `json:"after_revision,omitempty"`
	QueueState     string     `json:"queue_state"`
}

func recordObservation(d *Database, o Observation, at int64) error {
	if o.ID == "" || len(o.ID) > 160 || o.Kind == "" || len(o.Kind) > 80 || len(o.Body) > 65536 || len(o.Ref) > 2048 {
		return errors.New("invalid observation")
	}
	if old, ok := d.Reconciliations[o.ID]; ok {
		if !reflect.DeepEqual(old.Observation, o) {
			return ErrConflict
		}
		return nil
	}
	d.Reconciliations[o.ID] = Reconciliation{Observation: o, State: "pending", ReceivedAt: at}
	changed(d, at, "observation", o.MatterID, o.ID, o.Kind)
	return nil
}

// Ingest commits observations and the producer checkpoint together. It never
// interprets an observation as a user answer, authorization or delivery request.
func (s *Store) Ingest(source string, expected, cursor int64, observations []Observation, at int64) error {
	if source == "" || cursor < 0 || cursor < expected {
		return errors.New("invalid observation checkpoint")
	}
	return s.transaction(func(d *Database) error {
		if d.ObservationCursors[source] != expected {
			return ErrConflict
		}
		for _, o := range observations {
			if err := recordObservation(d, o, at); err != nil {
				return err
			}
		}
		if _, ok := d.ObservationCursors[source]; !ok || cursor != expected {
			d.ObservationCursors[source] = cursor
			changed(d, at, "observation-checkpoint", "", source, "")
		}
		return nil
	})
}

func ReconciliationQueue(d Database, now int64) []Reconciliation {
	out := []Reconciliation{}
	for _, q := range d.Reconciliations {
		if q.State == "assessed" || q.State == "conflict" || q.State == "deferred" && q.NextCheck > now {
			continue
		}
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReceivedAt == out[j].ReceivedAt {
			return out[i].Observation.ID < out[j].Observation.ID
		}
		return out[i].ReceivedAt < out[j].ReceivedAt
	})
	return out
}

func (s *Store) ClaimObservation(id, owner string, at int64) (Reconciliation, error) {
	var out Reconciliation
	if owner == "" || len(owner) > 256 {
		return out, errors.New("verified assessor identity required")
	}
	err := s.transaction(func(d *Database) error {
		q, ok := d.Reconciliations[id]
		if !ok {
			return ErrNotFound
		}
		if q.State == "leased" && q.LeaseUntil > at {
			if q.Owner != owner {
				return ErrConflict
			}
			out = q
			return nil
		}
		if q.State == "assessed" || q.State == "conflict" || q.State == "deferred" && q.NextCheck > at {
			return ErrConflict
		}
		q.State = "leased"
		q.Owner = owner
		q.Generation++
		q.LeaseUntil = at + 300
		d.Reconciliations[id] = q
		out = q
		changed(d, at, "observation-claimed", q.Observation.MatterID, id, owner)
		return nil
	})
	return out, err
}

func assess(d *Database, a Assessment, at int64) (AssessmentReceipt, error) {
	if old, ok := d.Assessments[a.ID]; ok {
		if reflect.DeepEqual(old.Assessment, a) {
			return old, nil
		}
		return AssessmentReceipt{}, ErrConflict
	}
	var receipt AssessmentReceipt
	if !validID(a.ID) {
		return receipt, errors.New("assessment receipt ID required")
	}
	if _, err := text(a.Reason, 32768); err != nil {
		return receipt, err
	}
	q, ok := d.Reconciliations[a.ObservationID]
	if !ok {
		return receipt, ErrNotFound
	}
	if q.State != "leased" || q.Owner != a.Owner || q.Generation != a.Generation || q.LeaseUntil <= at {
		return receipt, ErrConflict
	}
	if q.Observation.MatterID != a.MatterID && CanonicalMatter(*d, q.Observation.MatterID) != a.MatterID {
		return receipt, ErrConflict
	}
	var m Matter
	if a.MatterID != "" {
		if a.Revision == 0 {
			return receipt, ErrConflict
		}
		var err error
		m, err = getMatter(d, a.MatterID, a.Revision)
		if err != nil {
			return receipt, err
		}
	}
	receipt = AssessmentReceipt{Assessment: a, At: at, BeforeRevision: m.Revision, AfterRevision: m.Revision}
	switch a.Decision {
	case "noted":
		q.State = "assessed"
	case "conflict":
		q.State = "conflict"
	case "deferred":
		if a.NextCheck <= at {
			return receipt, errors.New("deferred assessment requires a future next_check")
		}
		q.State = "deferred"
		q.NextCheck = a.NextCheck
	case "transition":
		// Unassigned/old-version evidence can be acknowledged, but cannot mutate a
		// newer target. HQ can issue a separate explicit transition after rereading it.
		if a.MatterID == "" || q.Observation.MatterID != a.MatterID || q.Observation.MatterRevision != a.Revision || a.State == "" {
			return receipt, ErrConflict
		}
		if err := validateTransition(m, a.State, a.Revision); err != nil {
			return receipt, err
		}
		m.State = a.State
		m.Assessment = a.Reason
		q.State = "assessed"
	default:
		return receipt, errors.New("invalid assessment decision")
	}
	if a.Decision != "transition" && a.State != "" {
		return receipt, errors.New("state requires transition decision")
	}
	if a.Decision != "deferred" && a.NextCheck != 0 {
		return receipt, errors.New("next_check requires deferred decision")
	}
	changed(d, at, "assessment", a.MatterID, a.ID, a.Decision)
	if a.Decision == "transition" {
		saveMatter(d, m, at)
		receipt.AfterRevision = d.Revision
	}
	q.LeaseUntil = 0
	q.ReceiptID = a.ID
	d.Reconciliations[a.ObservationID] = q
	receipt.QueueState = q.State
	d.Assessments[a.ID] = receipt
	return receipt, nil
}

// Assess has no delivery adapter. A dry run uses the same validation on a
// private snapshot; the transaction is the only authority for a committed result.
func (s *Store) Assess(a Assessment, at int64, dryRun bool) (AssessmentReceipt, error) {
	if dryRun {
		d, err := s.Read()
		if err != nil {
			return AssessmentReceipt{}, err
		}
		return assess(&d, a, at)
	}
	var out AssessmentReceipt
	err := s.transaction(func(d *Database) error { var err error; out, err = assess(d, a, at); return err })
	return out, err
}

func validateTransition(m Matter, state string, revision uint64) error {
	if revision == 0 || revision != m.Revision {
		return ErrConflict
	}
	if !validState(state) {
		return errors.New("invalid matter state")
	}
	if m.UserInput != nil && state != "needs-input" {
		return errors.New("resolve the current question with clear-input and its revision before changing state")
	}
	return nil
}

// MatchObservation never uses pane/title. Legacy inclusive interval endpoints
// are intentionally ambiguous at a shared second, rather than guessed away.
func MatchObservation(d Database, c Conversation, at int64) (string, uint64) {
	if c.Agent == "" || c.SessionID == "" {
		return "", 0
	}
	ids := map[string]bool{}
	for _, m := range d.Matters {
		for _, a := range m.Participants {
			if a.Role != "execution" || !a.Conversation.Same(c) || at < a.From || a.Until != 0 && at > a.Until || a.RemovedAt != 0 && at >= a.RemovedAt {
				continue
			}
			ids[CanonicalMatter(d, m.ID)] = true
		}
	}
	if len(ids) != 1 {
		return "", 0
	}
	for id := range ids {
		return id, d.Matters[id].Revision
	}
	return "", 0
}

// ActivateObservations establishes one prospective boundary for all producers.
func (s *Store) ActivateObservations(sequence, at int64) error {
	return s.transaction(func(d *Database) error {
		if _, ok := d.ObservationCursors["events"]; ok {
			return nil
		}
		d.ObservationCursors["events"] = sequence
		d.ObservationCursors["relay-start"] = at
		changed(d, at, "observation-activated", "", "", "")
		return nil
	})
}
