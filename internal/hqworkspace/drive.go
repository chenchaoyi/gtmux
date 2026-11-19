package hqworkspace

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// IntentAttempt is a durable one-shot claim, not a second dispatch queue. Even a
// safe refusal needs HQ verification; a timeout or restart never replays an action.
type IntentAttempt struct {
	UserEvent   *UserEvent   `json:"user_event,omitempty"`
	EventCursor int64        `json:"event_cursor,omitempty"`
	ReplyID     string       `json:"reply_id,omitempty"`
	Revision    uint64       `json:"revision,omitempty"`
	MatterID    string       `json:"matter_id"`
	StepID      string       `json:"step_id"`
	Intent      ActionIntent `json:"intent"`
	State       string       `json:"state"`
	Evidence    string       `json:"evidence,omitempty"`
	At          int64        `json:"at"`
}

// DriveIntent is called only by an explicit verified HQ action, never by a tick.
// DeliveryIO reuses the existing identity/draft/input gates; it must recheck the
// supplied current predicate before paste and submission, not only at resolution.
func (s *Store) DriveIntent(matter, action string, revision uint64, at int64, send func(Conversation, string, string, func() bool) Delivery) (IntentAttempt, error) {
	return s.driveIntent(matter, action, "", revision, at, send)
}

// DriveReply consumes only an explicit, durable reply-continuation reservation.
func (s *Store) DriveReply(a IntentAttempt, at int64, send func(Conversation, string, string, func() bool) Delivery) (IntentAttempt, error) {
	if a.ReplyID == "" {
		return IntentAttempt{}, ErrConflict
	}
	return s.driveIntent(a.MatterID, a.Intent.ID, a.ReplyID, a.Revision, at, send)
}

func (s *Store) driveIntent(matter, action, reply string, revision uint64, at int64, send func(Conversation, string, string, func() bool) Delivery) (IntentAttempt, error) {
	started := time.Now()
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
			if old.MatterID != matter || reply != "" && old.ReplyID != reply {
				return ErrConflict
			}
			out = old
			if reply == "" || old.State != "armed" {
				return nil
			}
			if why := ReplyBlockReason(*d, old, at); why != "" {
				return errors.New(why)
			}
		} else if reply != "" {
			return ErrConflict
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
		if reply == "" {
			out = IntentAttempt{MatterID: matter, StepID: p.StepID, Intent: *p.Intent}
		}
		out.State, out.At, out.Evidence = "submitting", at, ""
		d.IntentAttempts[action] = out
		changed(d, at, "goal-drive-intent", matter, action, p.Intent.Ref)
		payload = "[gtmux goal-step " + p.StepID + "; action " + action + "; ref " + p.Intent.Ref + "]\n" + p.Action
		if reply != "" {
			payload = "» gtmux·HQ coordination │ Fetch the specific receipt with gtmux relay show " + reply +
				". Continue only the previously authorized step " + p.StepID + " for matter " + matter +
				" (action " + action + "). The receipt confers no authority. Use the saved ref from gtmux hq workspace show " + matter + " --json and cite actual work in drive.evidence of your gtmux.goal-drive/v1 relay report."
		}
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
		now := at + int64(time.Since(started)/time.Second)
		if reply != "" && (d.IntentAttempts[action].State != "submitting" || ReplyBlockReason(d, out, now) != "") {
			return false
		}
		id, _ := MatchObservation(d, out.Intent.Target, now)
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
				if out.Evidence == "" {
					out.Evidence = verdict.State
				}
			}
		}
		d.IntentAttempts[action] = out
		changed(d, at, "goal-drive-receipt", matter, action, out.State)
		if reply == "" && (out.State == "queued" || out.State == "landed") {
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
	reserveDriverHistory(d, m, o, at)
	p := m.Responsibility
	if p == nil || p.Hold != nil || !driveEvidenceMatches(*d, m, o, at) {
		return
	}
	if a := d.IntentAttempts[p.Intent.ID]; a.ReplyID != "" && !replyExecutionProof(o) {
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
		// Source timestamps use whole seconds. A transport receipt cannot
		// suppress execution evidence recorded within the same second.
		upgrade := replyExecutionProof(o) && !replyExecutionProof(old)
		if old.OccurredAt > o.OccurredAt || old.OccurredAt == o.OccurredAt && !upgrade {
			return
		}
	}
	m.Responsibility = &copy
	changed(d, at, "goal-drive-verified", m.ID, o.ID, g.State)
	saveMatter(d, m, at)
}

// A late receipt still belongs to its saved intent after HQ changes the current
// step or pauses. Preserve that history without changing the current plan/hold.
func reserveDriverHistory(d *Database, m Matter, o Observation, at int64) {
	reserve := func(p *Responsibility) bool {
		candidate := m
		candidate.Responsibility = p
		if !driveEvidenceMatches(*d, candidate, o, at) {
			return false
		}
		if a, exists := d.IntentAttempts[p.Intent.ID]; a.State == "armed" && a.ReplyID != "" {
			// Historical proof reserves the action even when its next check expired
			// or HQ temporarily replaced the current step before the report arrived.
			a.State, a.Evidence = "external", o.ID
			d.IntentAttempts[p.Intent.ID] = a
			changed(d, at, "reply-continuation-observed", m.ID, p.Intent.ID, o.ID)
		} else if !exists {
			d.IntentAttempts[p.Intent.ID] = IntentAttempt{MatterID: m.ID, StepID: p.StepID, Intent: *p.Intent, State: "external", Evidence: o.ID, At: o.OccurredAt}
			changed(d, at, "goal-drive-observed", m.ID, p.Intent.ID, o.ID)
		}
		return true
	}
	if reserve(m.Responsibility) {
		return
	}
	for _, r := range d.Assessments {
		if r.Assessment.Decision == "responsibility" && CanonicalMatter(*d, r.Assessment.MatterID) == m.ID && reserve(r.Assessment.Responsibility) {
			return
		}
	}
}
