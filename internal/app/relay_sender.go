package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

const relayReplyPrefix = "[gtmux HQ reply to agent request "

// A header is only a bounded lookup hint into gtmux's own durable receipt. It
// never establishes authority, and its `source` is the requester, not the sender.
func relayReplyID(prompt string) string {
	if !strings.HasPrefix(prompt, relayReplyPrefix) {
		return ""
	}
	rest := strings.TrimPrefix(prompt, relayReplyPrefix)
	n := strings.IndexByte(rest, ';')
	if n < 3 || n > 80 {
		return ""
	}
	return rest[:n]
}

func stampRelaySenders(turns []transcript.Turn, home string) ([]transcript.Turn, string) {
	return stampRelaySendersWith(turns, home, relay.Get, transcript.FirstPromptRecords)
}

type relayPromptWitness func(context.Context, string, string, map[string]int64) map[string]string

func stampRelaySendersWith(turns []transcript.Turn, home string, get func(string) (relay.Request, error), witness relayPromptWitness) ([]transcript.Turn, string) {
	if home == "" {
		home = transcript.CodexHomeID()
	}
	h := sha256.New()
	receipts := map[string]relay.Request{}
	checked := map[string]bool{}
	prompts := map[string]map[string]int64{}
	for _, t := range turns {
		if t.Agent != "codex" || t.SessionID == "" {
			continue
		}
		id := relayReplyID(t.Prompt)
		if id == "" {
			continue
		}
		if !checked[id] {
			r, err := get(id)
			checked[id] = true
			// Receipt state contributes even before acceptance: ETags must not keep a
			// transient Human snapshot after a late acceptance receipt is persisted.
			if err == nil {
				fmt.Fprintf(h, "%s:%t:%s:%s:%s:%s:%s:%d:%x;", id, r.Delivered, r.ReplyAttempt, r.State, r.For, r.HomeID, r.ThreadID, r.CreatedAt, sha256.Sum256([]byte(relayReplyText(r))))
			}
			if err == nil && r.ID == id && r.Agent == "codex" && r.HomeID == home && r.State == "replied" && r.For == "hq" && r.Delivered && r.ReplyAttempt == "accepted" && r.CreatedAt > 0 && strings.TrimSpace(r.Reply) != "" {
				receipts[id] = r
			}
		}
		r, ok := receipts[id]
		if !ok || r.ThreadID != t.SessionID || t.Prompt != strings.TrimSpace(relayReplyText(r)) {
			continue
		}
		if prompts[t.SessionID] == nil {
			prompts[t.SessionID] = map[string]int64{}
		}
		prompts[t.SessionID][t.Prompt] = r.CreatedAt
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	records := map[string]map[string]string{}
	for sid, p := range prompts {
		records[sid] = witness(ctx, "codex", sid, p)
	}
	for i, t := range turns {
		if t.Agent != "codex" || t.From != nil || t.RecordID == "" {
			continue
		}
		if records[t.SessionID][t.Prompt] == t.RecordID {
			turns[i].From = senderFor("hq", nil)
		}
	}
	// Include the actual join result so cancelled/failed source reads are retried
	// instead of letting a Human ETag conceal a later successful witness.
	for _, t := range turns {
		if t.Agent == "codex" && relayReplyID(t.Prompt) != "" {
			fmt.Fprintf(h, "%s:%v;", t.RecordID, t.From)
		}
	}
	return turns, fmt.Sprintf("relay-v1-%x", h.Sum(nil)[:12])
}
