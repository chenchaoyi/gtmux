package hqworkspace

import (
	"errors"
	"reflect"
	"strings"
)

// UserInput is a concrete human request, separate from goal state and reports.
// Choices are proposed replies, never permissions or executable actions.
type UserInput struct {
	AnswerID string   `json:"answer_id,omitempty"`
	ID       string   `json:"id"`
	Kind     string   `json:"kind"` // decision | manual | information
	Prompt   string   `json:"prompt"`
	Why      string   `json:"why"`
	Impact   string   `json:"impact"`
	Choices  []string `json:"choices,omitempty"`
	At       int64    `json:"at"`
}

func (q UserInput) validate() error {
	if q.AnswerID != "" {
		return errors.New("a request cannot supply a user answer")
	}
	if !validID(q.ID) || (q.Kind != "decision" && q.Kind != "manual" && q.Kind != "information") {
		return errors.New("invalid human request ID or kind")
	}
	for _, field := range []struct {
		value string
		limit int
	}{{q.Prompt, 360}, {q.Why, 600}, {q.Impact, 1200}} {
		if _, err := text(field.value, field.limit); err != nil {
			return errors.New("human request needs a short prompt, reason and impact")
		}
	}
	if len(q.Choices) > 4 {
		return errors.New("human request supports at most four choices")
	}
	seen := map[string]bool{}
	for _, choice := range q.Choices {
		if _, err := text(choice, 240); err != nil || seen[strings.TrimSpace(choice)] {
			return errors.New("invalid or duplicate human request choice")
		}
		seen[strings.TrimSpace(choice)] = true
	}
	return nil
}
func (q UserInput) body() string {
	parts := []string{q.Prompt, q.Why, q.Impact}
	for _, c := range q.Choices {
		parts = append(parts, "• "+c)
	}
	return strings.Join(parts, "\n\n")
}

// RequestInput commits the question and its audit message together. A later
// progress report cannot replace this question with operational details.
func (s *Store) RequestInput(matter string, q UserInput, revision uint64, at int64) error {
	if err := q.validate(); err != nil {
		return err
	}
	q.At = at
	return s.transaction(func(d *Database) error {
		if old, ok := d.Messages[q.ID]; ok {
			expected := q
			if old.UserInput != nil {
				expected.At = old.UserInput.At
			}
			if old.MatterID == matter && reflect.DeepEqual(old.UserInput, &expected) {
				return nil
			}
			return ErrConflict
		}
		m, err := getMatter(d, matter, revision)
		if err != nil {
			return err
		}
		changed(d, at, "hq-decision", matter, q.ID, "")
		d.Messages[q.ID] = Message{ID: q.ID, MatterID: matter, Speaker: "hq", Kind: "decision", MatterState: "needs-input", Body: q.body(), At: at, Seq: d.Revision, UserInput: &q}
		m.State = "needs-input"
		m.UserInput = &q
		saveMatter(d, m, at)
		return nil
	})
}

// ClearInput withdraws only the identified question, never completes its goal.
// A verified reply/superseding fact must be cited in the recorded explanation.
func (s *Store) ClearInput(matter, requestID, messageID, reason string, revision uint64, at int64) error {
	if !validID(requestID) || !validID(messageID) {
		return errors.New("question and receipt IDs required")
	}
	if _, err := text(reason, 1200); err != nil {
		return err
	}
	return s.transaction(func(d *Database) error {
		if old, ok := d.Messages[messageID]; ok {
			if old.Speaker == "hq" && old.Kind == "progress" && old.MatterID == matter && old.ReplyTo == requestID && old.Body == reason {
				return nil
			}
			return ErrConflict
		}
		m, err := getMatter(d, matter, revision)
		if err != nil {
			return err
		}
		if m.UserInput == nil || m.UserInput.ID != requestID {
			return ErrConflict
		}
		changed(d, at, "hq-progress", matter, messageID, "")
		d.Messages[messageID] = Message{ID: messageID, MatterID: matter, Speaker: "hq", Kind: "progress", Body: reason, At: at, Seq: d.Revision, ReplyTo: requestID}
		m.UserInput = nil
		if m.State == "needs-input" {
			m.State = "open"
		}
		m.Assessment = reason
		saveMatter(d, m, at)
		return nil
	})
}
