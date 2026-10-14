package hqworkspace

import (
	"errors"
	"sort"
)

func validKind(k string) bool {
	return k == "report" || k == "progress" || k == "decision" || k == "result" || k == "error"
}

// Start commits a new goal and its first instruction together; a failed write
// cannot leave a goal that appears submitted without the user's instruction.
func (s *Store) Start(id, title, messageID, body string, at int64) (Matter, error) {
	var out Matter
	if !validID(id) || !validID(messageID) {
		return out, errors.New("invalid matter or message ID")
	}
	if _, err := text(title, 512); err != nil {
		return out, err
	}
	if _, err := text(body, 32768); err != nil {
		return out, err
	}
	err := s.transaction(func(d *Database) error {
		if m, ok := d.Matters[id]; ok {
			msg, exists := d.Messages[messageID]
			if !exists || (msg.Subject != "" && msg.Subject != title || msg.Subject == "" && m.Title != title) || msg.Body != body || msg.MatterID != id || msg.Speaker != "user" {
				return ErrConflict
			}
			out = m
			return nil
		}
		if _, ok := d.Messages[messageID]; ok {
			return ErrConflict
		}
		changed(d, at, "create", id, "", "")
		out = Matter{ID: id, Title: title, State: "open", CreatedAt: at, Participants: []Association{}}
		saveMatter(d, out, at)
		out = d.Matters[id]
		changed(d, at, "instruction", id, messageID, "")
		d.Messages[messageID] = Message{ID: messageID, MatterID: id, Speaker: "user", Kind: "instruction", Subject: title, Body: body, At: at, Seq: d.Revision, Delivery: "pending"}
		return nil
	})
	return out, err
}

// Submit persists before any input is attempted. Exact ID/content reuse converges,
// including after a caller lost the first response; it never duplicates a command.
func (s *Store) Submit(id, matter, body string, at int64) (Message, error) {
	var out Message
	if !validID(id) {
		return out, errors.New("invalid message ID")
	}
	body, err := text(body, 32768)
	if err != nil {
		return out, err
	}
	err = s.transaction(func(d *Database) error {
		if old, exists := d.Messages[id]; exists {
			if old.Speaker != "user" || old.MatterID != matter || old.Body != body {
				return ErrConflict
			}
			out = old
			return nil
		}
		if matter != "" {
			if _, err := getMatter(d, matter, 0); err != nil {
				return err
			}
		}
		changed(d, at, "instruction", matter, id, "")
		out = Message{ID: id, MatterID: matter, Speaker: "user", Kind: "instruction", Body: body, At: at, Seq: d.Revision, Delivery: "pending"}
		d.Messages[id] = out
		return nil
	})
	return out, err
}

func (s *Store) Publish(id, matter, replyTo, kind, body, state string, at int64) (Message, error) {
	var out Message
	if !validID(id) || !validKind(kind) {
		return out, errors.New("invalid report ID or kind")
	}
	if state != "" && !validState(state) {
		return out, errors.New("invalid matter state")
	}
	body, err := text(body, 32768)
	if err != nil {
		return out, err
	}
	err = s.transaction(func(d *Database) error {
		if matter != "" {
			seen := map[string]bool{}
			for d.Matters[matter].MergedInto != "" && !seen[matter] {
				seen[matter] = true
				matter = d.Matters[matter].MergedInto
			}
		}
		if old, exists := d.Messages[id]; exists {
			if old.Speaker != "hq" || (old.MatterID != matter && !resolvesTo(*d, old.MatterID, matter)) || old.MatterState != state || old.Kind != kind || old.Body != body || old.ReplyTo != replyTo {
				return ErrConflict
			}
			out = old
			return nil
		}
		var m Matter
		if matter != "" {
			m, err = getMatter(d, matter, 0)
			if err != nil {
				return err
			}
		}
		if replyTo != "" {
			parent, ok := d.Messages[replyTo]
			if !ok || (parent.MatterID != matter && !resolvesTo(*d, parent.MatterID, matter)) || parent.Speaker != "user" {
				return errors.New("reply must refer to a user message in this scope")
			}
		}
		changed(d, at, "hq-"+kind, matter, id, "")
		out = Message{ID: id, MatterID: matter, Speaker: "hq", Kind: kind, MatterState: state, Body: body, At: at, Seq: d.Revision, ReplyTo: replyTo}
		d.Messages[id] = out
		if matter != "" {
			m.Assessment = body
			if state != "" {
				m.State = state
			} // Never infer completed from kind=result or a turn ending.
			saveMatter(d, m, at)
		}
		return nil
	})
	return out, err
}

func (s *Store) Cancel(id string, at int64) error {
	return s.transaction(func(d *Database) error {
		m, ok := d.Messages[id]
		if !ok {
			return ErrNotFound
		}
		if m.Delivery == "cancelled" {
			return nil
		}
		if m.Delivery != "pending" {
			return errors.New("only an unattempted pending instruction can be cancelled")
		}
		m.Delivery, m.Reason = "cancelled", ""
		changed(d, at, "cancel", m.MatterID, id, "")
		d.Messages[id] = m
		return nil
	})
}

// Retry is explicit. An uncertain attempt may have executed and requires the
// caller's acknowledgement, never a background timeout or an ordinary Retry click.
func (s *Store) Retry(id string, acknowledgeUncertain bool, at int64) error {
	return s.transaction(func(d *Database) error {
		m, ok := d.Messages[id]
		if !ok {
			return ErrNotFound
		}
		if m.Delivery == "pending" {
			return nil
		}
		if m.Delivery != "uncertain" || !acknowledgeUncertain {
			return errors.New("instruction may already have been submitted; explicitly acknowledge before retrying")
		}
		m.Delivery, m.Reason, m.AttemptAt, m.Target = "pending", "", 0, Conversation{}
		changed(d, at, "retry", m.MatterID, id, "acknowledged-uncertain")
		d.Messages[id] = m
		return nil
	})
}

func resolvesTo(d Database, id, target string) bool {
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		if id == target {
			return true
		}
		seen[id] = true
		id = d.Matters[id].MergedInto
	}
	return false
}

// MessagePage reads earlier pages by sequence, not timestamps shared by messages.
func MessagePage(d Database, matter string, before uint64, limit int) (rows []Message, earlier bool) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows = []Message{}
	for _, m := range d.Messages {
		if before != 0 && m.Seq >= before {
			continue
		}
		if matter == "" && m.MatterID != "" || matter != "" && !resolvesTo(d, m.MatterID, matter) {
			continue
		}
		rows = append(rows, m)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seq < rows[j].Seq })
	if len(rows) > limit {
		earlier = true
		rows = rows[len(rows)-limit:]
	}
	return rows, earlier
}

// CanonicalMatter follows only recorded merge edges and rejects broken/cyclic graphs.
func CanonicalMatter(d Database, id string) string {
	if id == "" {
		return ""
	}
	seen := map[string]bool{}
	for !seen[id] {
		seen[id] = true
		m, ok := d.Matters[id]
		if !ok {
			return ""
		}
		if m.MergedInto == "" {
			return id
		}
		id = m.MergedInto
	}
	return ""
}

// Restored records are historical evidence, not fresh authorization to replay
// instructions on a new environment. Review explicitly before resubmitting them.
func (s *Store) MarkRestored(at int64) error {
	d, err := s.Read()
	if err != nil {
		return err
	}
	if len(d.Messages) == 0 {
		return nil
	}
	return s.transaction(func(d *Database) error {
		for id, m := range d.Messages {
			if m.Delivery != "pending" && m.Delivery != "submitting" && m.Delivery != "accepted-queue" {
				continue
			}
			m.Delivery, m.Reason = "uncertain", "restored-backup"
			changed(d, at, "delivery-uncertain", m.MatterID, id, m.Reason)
			d.Messages[id] = m
		}
		return nil
	})
}
