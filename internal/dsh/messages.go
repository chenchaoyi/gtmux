package dsh

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Message is one public append, not a model-context replacement or a streaming
// delta. Sequence gaps are normal because private and tool events share the log.
type Message struct {
	ID   string `json:"id"`
	Seq  int64  `json:"seq"`
	Time int64  `json:"time"`
	Role string `json:"role"`
	Text string `json:"text"`
}

func PublicMessage(event Event) (Message, bool, error) {
	if !event.AppendsHistory() || event.Type != "user/message" && event.Type != "assistant/message" {
		return Message{}, false, nil
	}
	type message struct {
		ID, Role string
		Source   struct{ Kind string }
		Content  []struct{ Type, Text string }
	}
	var value message
	if event.Type == "user/message" {
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return Message{}, false, err
		}
		if value.Role != "user" || value.Source.Kind != "user" {
			return Message{}, false, nil
		}
	} else {
		var wrapped struct{ Message message }
		if err := json.Unmarshal(event.Data, &wrapped); err != nil {
			return Message{}, false, err
		}
		value = wrapped.Message
		if value.Role != "assistant" {
			return Message{}, false, nil
		}
	}
	var pieces []string
	for _, block := range value.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			pieces = append(pieces, block.Text)
		}
	}
	text := strings.Join(pieces, "\n")
	if text == "" {
		return Message{}, false, nil
	}
	if event.Seq < 0 || len(text) > 1<<20 || len(value.ID) > 256 {
		return Message{}, false, errors.New("DSH public message exceeds read limits")
	}
	id := value.ID
	if id == "" {
		id = fmt.Sprintf("seq:%d", event.Seq)
	}
	return Message{id, event.Seq, event.Time, value.Role, text}, true, nil
}

type MessagePage struct {
	Messages        []Message `json:"messages"`
	NextSeq         int64     `json:"next_seq"`
	HasMore         bool      `json:"has_more"`
	ReviewedThrough int64     `json:"reviewed_through"`
	SourceSeq       int64     `json:"source_seq"`
	Revision        string    `json:"revision"`
}

var errPageComplete = errors.New("public message page complete")

// ReadMessages is an explicit bounded history read. Long-lived reconciliations
// use Scan with their own cursor, so periodic work reads only committed appends.
func ReadMessages(id string, after int64, limit int) (MessagePage, error) {
	page := MessagePage{Messages: []Message{}, NextSeq: after, SourceSeq: -1, ReviewedThrough: -1}
	if after < -1 || limit < 1 || limit > 200 {
		return page, errors.New("invalid public message window")
	}
	session, err := Sessions.Resolve(id)
	if err != nil || session.Origin == "subagent" {
		return page, ErrUnsupported
	}
	page.Revision = session.Revision
	var cursor Cursor
	bytes := 0
	err = Scan(session.Path, &cursor, func() {}, func(line []byte) error {
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		if event.Type == "session" {
			return nil
		}
		if event.Seq > page.SourceSeq {
			page.SourceSeq = event.Seq
		}
		if event.Seq <= after {
			return nil
		}
		message, public, err := PublicMessage(event)
		if err != nil {
			return err
		}
		if !public {
			return nil
		}
		if len(page.Messages) >= limit || bytes+len(message.Text) > 1<<20 {
			page.HasMore = true
			return errPageComplete
		}
		page.Messages = append(page.Messages, message)
		page.NextSeq = message.Seq
		bytes += len(message.Text)
		return nil
	})
	if errors.Is(err, errPageComplete) {
		err = nil
	}
	return page, err
}
