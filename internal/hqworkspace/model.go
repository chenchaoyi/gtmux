// Package hqworkspace owns explicit goals and their conversation relationships.
// It neither discovers panes nor types into them; app supplies those adapters.
package hqworkspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"
)

const Schema = 1

var (
	ErrNotFound = errors.New("workspace record not found")
	ErrConflict = errors.New("workspace record changed; reload before applying")
)

type Database struct {
	Schema   int                `json:"schema"`
	Revision uint64             `json:"revision"`
	Matters  map[string]Matter  `json:"matters"`
	Messages map[string]Message `json:"messages"`
	Changes  []Change           `json:"changes"`
}

type Matter struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	State        string        `json:"state"` // open | needs-input | completed | closed
	Assessment   string        `json:"assessment,omitempty"`
	CreatedAt    int64         `json:"created_at"`
	UpdatedAt    int64         `json:"updated_at"`
	Revision     uint64        `json:"revision"`
	Participants []Association `json:"participants"`
	MergedInto   string        `json:"merged_into,omitempty"`
	Parent       string        `json:"parent,omitempty"`
}

// Conversation identifies a source by agent/home/session, never its locator.
type Conversation struct {
	Agent     string `json:"agent"`
	HomeID    string `json:"home_id,omitempty"`
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
	Pane      string `json:"pane,omitempty"` // last-known display/jump hint only
	Loc       string `json:"loc,omitempty"`
}

func (c Conversation) Same(other Conversation) bool {
	return c.Agent == other.Agent && c.HomeID == other.HomeID && c.SessionID == other.SessionID
}

type Association struct {
	ID           string       `json:"id"`
	Conversation Conversation `json:"conversation"`
	Role         string       `json:"role"` // execution | reference
	From         int64        `json:"from"`
	Until        int64        `json:"until,omitempty"` // 0 = ongoing
	RemovedAt    int64        `json:"removed_at,omitempty"`
}

type Message struct {
	Subject     string       `json:"subject,omitempty"`
	MatterState string       `json:"matter_state,omitempty"`
	ID          string       `json:"id"`
	MatterID    string       `json:"matter_id,omitempty"` // empty = global HQ conversation
	Speaker     string       `json:"speaker"`             // user | hq
	Kind        string       `json:"kind"`                // instruction | report | progress | decision | result | error
	Body        string       `json:"body"`
	At          int64        `json:"at"`
	Seq         uint64       `json:"seq"`
	ReplyTo     string       `json:"reply_to,omitempty"`
	Delivery    string       `json:"delivery,omitempty"` // pending | submitting | delivered | accepted-queue | uncertain | cancelled
	Reason      string       `json:"reason,omitempty"`
	Target      Conversation `json:"target,omitempty"`
	AttemptAt   int64        `json:"attempt_at,omitempty"`
	DeliveredAt int64        `json:"delivered_at,omitempty"`
	Evidence    string       `json:"evidence,omitempty"`
}

type Change struct {
	Seq    uint64 `json:"seq"`
	At     int64  `json:"at"`
	Kind   string `json:"kind"`
	Matter string `json:"matter,omitempty"`
	Target string `json:"target,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

func validID(s string) bool {
	if len(s) < 3 || len(s) > 80 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func text(s string, max int) (string, error) {
	if strings.TrimSpace(s) == "" || !utf8.ValidString(s) || len(s) > max {
		return "", errors.New("text is empty, invalid UTF-8 or too long")
	}
	return s, nil
}

func validState(s string) bool {
	return s == "open" || s == "needs-input" || s == "completed" || s == "closed"
}

func Empty() Database {
	return Database{Schema: Schema, Matters: map[string]Matter{}, Messages: map[string]Message{}, Changes: []Change{}}
}
