package hqworkspace

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// IntentAttempt is a durable one-shot claim, not a second dispatch queue. Even a
// safe refusal needs HQ verification; a timeout or restart never replays an action.
type IntentAttempt struct {
	MatterID string       `json:"matter_id"`
	StepID   string       `json:"step_id"`
	Intent   ActionIntent `json:"intent"`
	State    string       `json:"state"`
	Evidence string       `json:"evidence,omitempty"`
	At       int64        `json:"at"`
}

// DriveIntent is called only by an explicit verified HQ action, never by a tick.
// DeliveryIO reuses the existing identity/draft/input gates; it must recheck the
// supplied current predicate before paste and submission, not only at resolution.
func (s *Store) DriveIntent(matter, action string, revision uint64, at int64, send func(Conversation, string, string, func() bool) Delivery) (IntentAttempt, error) {
	var out IntentAttempt
	if err := state.MkdirAll(s.dir, 0700); err != nil {
		return out, err
	}
	lock, err := state.OpenFile(filepath.Join(s.dir, ".goal-drive-lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return out, err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return out, ErrConflict
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	var selected bool
	var payload string
	err = s.transaction(func(d *Database) error {
		m, e := getMatter(d, matter, 0)
		if e != nil {
			return e
		}
		if old, ok := d.IntentAttempts[action]; ok {
			if old.MatterID != matter {
				return ErrConflict
			}
			out = old
			return nil
		}
		if revision == 0 || m.Revision != revision {
			return ErrConflict
		}
		p := m.Responsibility
		if p == nil || p.Intent == nil || p.Intent.ID != action || p.Hold != nil || p.ReviewRequired || m.UserInput != nil || m.State != "open" {
			return ErrConflict
		}
		if e := validateResponsibility(*d, m, *p, at); e != nil {
			return e
		}
		g := GoalResponsibility(*d, m, at)
		if !goalOwed(g) {
			return errors.New("step already has drive coverage")
		}
		out = IntentAttempt{MatterID: matter, StepID: p.StepID, Intent: *p.Intent, State: "submitting", At: at}
		d.IntentAttempts[action] = out
		changed(d, at, "goal-drive-intent", matter, action, p.Intent.Ref)
		payload = "[gtmux goal-step " + p.StepID + "; action " + action + "; ref " + p.Intent.Ref + "]\n" + p.Action
		selected = true
		return nil
	})
	if err != nil || !selected {
		return out, err
	}
	current := func() bool {
		d, e := s.Read()
		if e != nil {
			return false
		}
		m := d.Matters[matter]
		p := m.Responsibility
		if m.Revision != revision || m.State != "open" || m.UserInput != nil || p == nil || p.Hold != nil || p.Intent == nil || !reflect.DeepEqual(*p.Intent, out.Intent) || p.StepID != out.StepID {
			return false
		}
		id, _ := MatchObservation(d, out.Intent.Target, at)
		return id == matter
	}
	verdict := Delivery{State: "stale-premise", SafeHold: true}
	if current() {
		verdict = send(out.Intent.Target, payload, action, current)
	}
	err = s.transaction(func(d *Database) error {
		old, ok := d.IntentAttempts[action]
		if !ok || old.State != "submitting" || !reflect.DeepEqual(old, out) {
			return ErrConflict
		}
		out.State = verdict.State
		out.Evidence = verdict.Evidence
		if len(out.Evidence) > 4096 {
			out.Evidence = out.Evidence[:4096]
		}
		if out.State != "queued" && out.State != "landed" {
			out.State = "uncertain"
			if verdict.SafeHold {
				out.State = "held"
			}
		}
		d.IntentAttempts[action] = out
		changed(d, at, "goal-drive-receipt", matter, action, out.State)
		if out.State == "queued" || out.State == "landed" {
			status := "queued"
			if out.State == "landed" {
				status = "received"
			}
			o := Observation{ID: "drive-" + action, Kind: "drive-receipt", MatterID: matter, Ref: "action:" + action, Conversation: out.Intent.Target, OccurredAt: at, DriveProof: &DriveProof{StepID: out.StepID, ActionID: action, Ref: out.Intent.Ref, State: status, NextCheck: at + 300}}
			if e := recordObservation(d, o, at); e != nil {
				return e
			}
			attachDrive(d, o, at)
		}
		syncGoal(d, d.Matters[matter], at)
		return nil
	})
	return out, err
}

func attachDrive(d *Database, o Observation, at int64) {
	m := d.Matters[o.MatterID]
	p := m.Responsibility
	if p == nil || p.Intent == nil || p.Hold != nil || o.DriveProof == nil {
		return
	}
	copy := *p
	copy.Drive = &DriveRef{ObservationID: o.ID}
	candidate := m
	candidate.Responsibility = &copy
	g := GoalResponsibility(*d, candidate, at)
	if g.State != "queued" && g.State != "driving" {
		return
	}
	// A late older receipt cannot overwrite a newer proof or resurrect an expired
	// native/background check. Source IDs remain immutable in the observation ledger.
	if p.Drive != nil {
		old := d.Reconciliations[p.Drive.ObservationID].Observation
		if old.OccurredAt >= o.OccurredAt {
			return
		}
	}
	// Observing an existing native/background driver reserves its action even
	// when no physical send ever happened here. Expiry removes live coverage,
	// never the durable fact that this work was already being driven.
	if _, exists := d.IntentAttempts[p.Intent.ID]; !exists {
		d.IntentAttempts[p.Intent.ID] = IntentAttempt{MatterID: m.ID, StepID: p.StepID, Intent: *p.Intent, State: "external", Evidence: o.ID, At: o.OccurredAt}
		changed(d, at, "goal-drive-observed", m.ID, p.Intent.ID, o.ID)
	}
	m.Responsibility = &copy
	changed(d, at, "goal-drive-verified", m.ID, o.ID, g.State)
	saveMatter(d, m, at)
}
