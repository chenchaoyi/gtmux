package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// MessageSender is historical delivery evidence, not a live pane binding.
type MessageSender struct {
	Actor string
	Head  string
	Time  int64
}

func (s MessageSender) Matches(text string, at int64) bool {
	return at > 0 && s.Time-at <= deliveryLead && at-s.Time <= deliveryLag && agreement(PayloadHead(text), s.Head) > 0
}

type messageScope struct{ agent, home, session string }
type messageAuthors struct {
	by       map[int64]MessageSender
	revision string
}

// Bound the extra evidence independently of journal size. Eviction loses an
// attribution; it never turns saved history into a terminal attachment.
const sendWitnessLimit = 8192

type sendWitness struct {
	seq, ts, messageSeq int64
	pane, head, actor   string
	scope               messageScope
	message             bool
}

func (x *SendIndex) resetWitnesses() {
	x.witnesses, x.witnessNext, x.messageCache = nil, 0, nil
}

func (x *SendIndex) keepWitness(r Record) {
	if r.Seq <= 0 || r.Ts <= 0 {
		return
	}
	w := sendWitness{seq: r.Seq, ts: r.Ts}
	if pane, head, actor, ok := landedSend(r); ok {
		w.pane, w.head, w.actor = pane, head, actor
	} else if r.Event == "ConversationMessage" && r.AgentKey == "dsh" && r.MessageRole == "user" && r.HomeID != "" && r.AgentSession != "" && r.MessageID != "" && r.MessageSeq >= 0 {
		w.pane, w.head = r.Pane, PayloadHead(r.Summary)
		w.scope, w.messageSeq, w.message = messageScope{r.AgentKey, r.HomeID, r.AgentSession}, r.MessageSeq, true
	} else if r.Event == "UserPromptSubmit" {
		// Other submitted prompts still compete for a delivery. A same-text
		// prompt in another session must not be ignored by the one-to-one join.
		w.pane, w.head = r.Pane, PayloadHead(r.Summary)
	} else {
		return
	}
	if w.pane == "" || w.head == "" {
		return
	}
	if len(x.witnesses) < sendWitnessLimit {
		x.witnesses = append(x.witnesses, w)
	} else {
		x.witnesses[x.witnessNext] = w
		x.witnessNext = (x.witnessNext + 1) % sendWitnessLimit
	}
	x.messageCache = nil
}

// MessageSenders joins verified public-message observations to actual deliveries
// using AuthorOf's bounded, one-to-one rule. The full home/session tuple and native
// message sequence survive pane closure. No lookup uses a title, cwd or old pane ID
// as current input authority. Both reads share SendIndex's incremental journal scan.
func (x *SendIndex) MessageSenders(agent, home, session string) (map[int64]MessageSender, string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.refreshLocked()
	key := messageScope{agent, home, session}
	cached, ok := x.messageCache[key]
	if !ok {
		cached = x.messageAuthors(key)
		if len(x.messageCache) >= 32 || x.messageCache == nil {
			x.messageCache = map[messageScope]messageAuthors{}
		}
		x.messageCache[key] = cached
	}
	copy := make(map[int64]MessageSender, len(cached.by))
	for seq, sender := range cached.by {
		copy[seq] = sender
	}
	return copy, cached.revision
}

func (x *SendIndex) messageAuthors(key messageScope) messageAuthors {
	panes := map[string]bool{}
	if key.agent == "dsh" && key.home != "" && key.session != "" {
		for _, w := range x.witnesses {
			if w.message && w.scope == key {
				panes[w.pane] = true
			}
		}
	}
	var records []Record
	counts := map[int64]int{}
	for _, w := range x.witnesses {
		if !panes[w.pane] {
			continue
		}
		r := Record{Seq: w.seq, Ts: w.ts, Pane: w.pane, Summary: w.head, Event: "UserPromptSubmit"}
		if w.actor != "" {
			r.Event, r.Actor, r.Summary = AuditEventSend, w.actor, "landed: "+w.head
		}
		records = append(records, r)
		if w.message && w.scope == key {
			counts[w.messageSeq]++
		}
	}
	authors := AuthorOf(records)
	by := map[int64]MessageSender{}
	for _, w := range x.witnesses {
		if w.message && w.scope == key && counts[w.messageSeq] == 1 && authors[w.seq] != "" {
			by[w.messageSeq] = MessageSender{authors[w.seq], w.head, w.ts}
		}
	}
	encoded, _ := json.Marshal(by)
	sum := sha256.Sum256(encoded)
	return messageAuthors{by: by, revision: hex.EncodeToString(sum[:16])}
}
