package hqworkspace

import "errors"

// Register records HQ's verified existing goal, never a new user instruction.
// The initial report is also the immutable retry witness after the matter changes.
func (s *Store) Register(id, title, body, state string, origin Origin, at int64) (Matter, error) {
	var out Matter
	messageID := id + "-registration"
	if !validID(id) || !validID(messageID) {
		return out, errors.New("invalid registration ID")
	}
	for _, field := range []struct {
		value string
		limit int
	}{{title, 512}, {body, 32768}, {origin.Ref, 2048}} {
		if _, err := text(field.value, field.limit); err != nil {
			return out, err
		}
	}
	if !validState(state) || (origin.Kind != "board" && origin.Kind != "task" && origin.Kind != "conversation") {
		return out, errors.New("registration requires an explicit state and board, task or conversation source")
	}
	err := s.transaction(func(d *Database) error {
		if m, ok := d.Matters[id]; ok {
			msg, exists := d.Messages[messageID]
			if !exists || m.Origin == nil || *m.Origin != origin || msg.Subject != title || msg.Body != body || msg.MatterState != state || msg.MatterID != id || msg.Speaker != "hq" || msg.Kind != "report" {
				return ErrConflict
			}
			out = m
			return nil
		}
		if _, exists := d.Messages[messageID]; exists {
			return ErrConflict
		}
		changed(d, at, "register", id, messageID, origin.Kind)
		m := Matter{ID: id, Title: title, State: state, Assessment: body, Origin: &origin, CreatedAt: at, Participants: []Association{}}
		saveMatter(d, m, at)
		out = d.Matters[id]
		d.Messages[messageID] = Message{ID: messageID, MatterID: id, Speaker: "hq", Kind: "report", Subject: title, Body: body, MatterState: state, At: at, Seq: d.Revision}
		return nil
	})
	return out, err
}
