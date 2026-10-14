package hqworkspace

import (
	"errors"
	"sort"
)

func (s *Store) Create(id, title string, at int64) (Matter, error) {
	var out Matter
	if !validID(id) {
		return out, errors.New("invalid matter ID")
	}
	title, err := text(title, 512)
	if err != nil {
		return out, err
	}
	err = s.transaction(func(d *Database) error {
		if m, exists := d.Matters[id]; exists {
			if m.Title != title {
				return ErrConflict
			}
			out = m
			return nil
		}
		out = Matter{ID: id, Title: title, State: "open", CreatedAt: at, Participants: []Association{}}
		changed(d, at, "create", id, "", "")
		saveMatter(d, out, at)
		out = d.Matters[id]
		return nil
	})
	return out, err
}

func (s *Store) Update(id, title, state, assessment string, revision uint64, at int64) error {
	if title != "" {
		if _, err := text(title, 512); err != nil {
			return err
		}
	}
	if state != "" && !validState(state) {
		return errors.New("invalid matter state")
	}
	if len(assessment) > 32768 {
		return errors.New("assessment too long")
	}
	return s.transaction(func(d *Database) error {
		m, err := getMatter(d, id, revision)
		if err != nil {
			return err
		}
		if title != "" {
			m.Title = title
		}
		if state != "" {
			m.State = state
		}
		if assessment != "" {
			m.Assessment = assessment
		}
		changed(d, at, "update", id, "", state)
		saveMatter(d, m, at)
		return nil
	})
}

func validateAssociation(a Association, at int64) error {
	if !validID(a.ID) || a.Conversation.Agent == "" || a.Conversation.SessionID == "" {
		return errors.New("conversation identity required")
	}
	if a.Role != "execution" && a.Role != "reference" {
		return errors.New("invalid association role")
	}
	if a.From <= 0 || a.From > at || a.Until < 0 || a.Until > at || a.Until != 0 && a.Until < a.From {
		return errors.New("invalid conversation interval")
	}
	if a.Role == "reference" && a.Until == 0 {
		return errors.New("historical reference requires an end time")
	}
	if a.RemovedAt != 0 {
		return errors.New("cannot add a removed association")
	}
	return nil
}

func (s *Store) Associate(id string, a Association, revision uint64, at int64) error {
	if err := validateAssociation(a, at); err != nil {
		return err
	}
	return s.transaction(func(d *Database) error {
		m, err := getMatter(d, id, revision)
		if err != nil {
			return err
		}
		for _, old := range m.Participants {
			if old.ID == a.ID {
				if old.RemovedAt == 0 && old.Conversation.Same(a.Conversation) && old.Role == a.Role && old.From == a.From && old.Until == a.Until {
					return nil
				}
				return ErrConflict
			}
		}
		m.Participants = append(m.Participants, a)
		changed(d, at, "associate", id, a.ID, a.Role)
		saveMatter(d, m, at)
		return nil
	})
}

// Move changes metadata only. A removed association remains available for provenance.
func (s *Store) Move(from, to, association string, revision uint64, at int64) error {
	if from == to {
		return errors.New("choose a different matter")
	}
	return s.transaction(func(d *Database) error {
		m, err := getMatter(d, from, revision)
		if err != nil {
			return err
		}
		var target Matter
		if to != "" {
			target, err = getMatter(d, to, 0)
			if err != nil {
				return err
			}
		}
		for i, a := range m.Participants {
			if a.ID != association || a.RemovedAt != 0 {
				continue
			}
			m.Participants[i].RemovedAt = at
			if to != "" {
				for _, old := range target.Participants {
					if old.ID == a.ID && old.RemovedAt == 0 {
						return ErrConflict
					}
				}
				target.Participants = append(target.Participants, a)
			}
			kind := "unlink"
			if to != "" {
				kind = "move"
			}
			changed(d, at, kind, from, association, to)
			saveMatter(d, m, at)
			if to != "" {
				saveMatter(d, target, at)
			}
			return nil
		}
		return ErrNotFound
	})
}

func (s *Store) Interval(id, association string, from, until int64, revision uint64, at int64) error {
	return s.transaction(func(d *Database) error {
		m, err := getMatter(d, id, revision)
		if err != nil {
			return err
		}
		for i, a := range m.Participants {
			if a.ID != association || a.RemovedAt != 0 {
				continue
			}
			a.From, a.Until = from, until
			if err := validateAssociation(a, at); err != nil {
				return err
			}
			m.Participants[i] = a
			changed(d, at, "interval", id, association, "")
			saveMatter(d, m, at)
			return nil
		}
		return ErrNotFound
	})
}

func (s *Store) Merge(from, into string, revision uint64, at int64) error {
	if from == into {
		return errors.New("choose a different matter")
	}
	return s.transaction(func(d *Database) error {
		m, err := getMatter(d, from, revision)
		if err != nil {
			return err
		}
		target, err := getMatter(d, into, 0)
		if err != nil {
			return err
		}
		for _, a := range m.Participants {
			if a.RemovedAt != 0 {
				continue
			}
			for _, old := range target.Participants {
				if old.ID == a.ID && old.RemovedAt == 0 {
					return ErrConflict
				}
			}
			target.Participants = append(target.Participants, a)
		}
		m.MergedInto, m.State = into, "closed"
		// Messages retain original matter identity; readers follow the explicit merge edge.
		changed(d, at, "merge", from, into, "")
		saveMatter(d, m, at)
		saveMatter(d, target, at)
		return nil
	})
}

func (s *Store) Split(from, id, title string, associations []string, revision uint64, at int64) error {
	if !validID(id) {
		return errors.New("invalid matter ID")
	}
	if _, err := text(title, 512); err != nil {
		return err
	}
	if len(associations) == 0 {
		return errors.New("select conversations to move")
	}
	return s.transaction(func(d *Database) error {
		m, err := getMatter(d, from, revision)
		if err != nil {
			return err
		}
		if _, exists := d.Matters[id]; exists {
			return ErrConflict
		}
		want := map[string]bool{}
		for _, a := range associations {
			want[a] = true
		}
		target := Matter{ID: id, Title: title, State: "open", CreatedAt: at, Parent: from, Participants: []Association{}}
		for i, a := range m.Participants {
			if !want[a.ID] || a.RemovedAt != 0 {
				continue
			}
			target.Participants = append(target.Participants, a)
			m.Participants[i].RemovedAt = at
			delete(want, a.ID)
		}
		if len(want) > 0 {
			return ErrNotFound
		}
		changed(d, at, "split", from, id, "")
		saveMatter(d, m, at)
		saveMatter(d, target, at)
		return nil
	})
}

func Matters(d Database) []Matter {
	out := []Matter{}
	for _, m := range d.Matters {
		if m.MergedInto == "" {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}
