package app

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/mine"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Attributing a turn to whoever delivered it (who-sent-this-turn).
//
// A session log records what ARRIVED in a pane, never who caused it to arrive, so every
// prompt looked alike and the chat drew the reader's own avatar on all of them. On
// 2026-09-20 a message HQ relayed into the commander's pane appeared under his face, and
// his own history stopped telling him which instructions were his.
//
// gtmux knows, because gtmux performed the delivery: the audit journal records the target
// pane, a bounded head of the payload, and (since this change) the sender. Joining a turn
// to that record is what the parser cannot do and the server can.
//
// The join is deliberately narrow. It only ever ADDS a sender to a turn gtmux itself
// delivered on someone else's behalf; everything it cannot match stays exactly as it was,
// which is also what every turn the reader wrote themselves must look like.

// The journal window. The floor keeps a fresh conversation from reading a one-second
// slice; the margin covers a turn whose logged clock runs a little behind the delivery
// that caused it; the ceiling keeps a deeply stitched history from parsing the whole
// journal for turns older than any delivery record that survives in it.
const (
	sendLookbackFloor   = int64(2 * 60 * 60)
	sendLookbackMargin  = int64(10 * 60)
	sendLookbackCeiling = int64(30 * 24 * 60 * 60)
)

// minHeadForPrefix is how much recorded text a PREFIX match needs before it is allowed to
// claim a turn. The journal holds a bounded head of a payload, so a long instruction is
// recognised by its first runes even when the transcript kept more than the journal did.
// A short one has no such margin: "继续" is a prefix of any number of unrelated prompts,
// so below this width the two sides have to agree exactly.
const minHeadForPrefix = 12

// stampSenders fills in Turn.From for the turns gtmux delivered into this pane on
// someone else's behalf. Turns it cannot attribute are returned untouched.
func stampSenders(turns []transcript.Turn, pane string, now int64) []transcript.Turn {
	if pane == "" || len(turns) == 0 {
		return turns
	}
	return stampSendersWith(turns, sendersByHead(events.Read(sendLookback(turns, now), now), pane), paneLabel)
}

// stampSendersWith is the join itself, over what was already read from the journal: the
// half worth testing without a journal on disk.
func stampSendersWith(turns []transcript.Turn, by map[string]string, resolve func(string) (string, string)) []transcript.Turn {
	if len(by) == 0 {
		return turns
	}
	for i := range turns {
		if turns[i].From != nil || turns[i].Prompt == "" {
			continue
		}
		if s := senderFor(matchHead(by, turns[i].Prompt), resolve); s != nil {
			turns[i].From = s
		}
	}
	return turns
}

// matchHead finds who sent this prompt, or "" for nobody.
//
// The two sides are not the same text and cannot be compared as though they were: the
// journal holds a bounded HEAD of what was delivered, while the transcript holds the whole
// prompt, sometimes with the harness's own additions still attached. So a recorded head
// that the prompt BEGINS WITH is a match, longest first, because a longer head is the more
// specific claim. Anything shorter than minHeadForPrefix has to match exactly.
func matchHead(by map[string]string, prompt string) string {
	folded := mine.HeadKey(prompt)
	if actor, ok := by[folded]; ok {
		return actor
	}
	best, actor := 0, ""
	for head, who := range by {
		n := utf8.RuneCountInString(head)
		if n < minHeadForPrefix || n <= best || !strings.HasPrefix(folded, head) {
			continue
		}
		best, actor = n, who
	}
	return actor
}

// sendLookback is how far back the journal has to be read to cover these turns: to the
// oldest one that carries a clock, bounded at both ends.
func sendLookback(turns []transcript.Turn, now int64) int64 {
	oldest := now
	for _, t := range turns {
		if t.Time == "" {
			continue
		}
		if ts, err := time.Parse(time.RFC3339, t.Time); err == nil && ts.Unix() < oldest {
			oldest = ts.Unix()
		}
	}
	return min(max(now-oldest+sendLookbackMargin, sendLookbackFloor), sendLookbackCeiling)
}

// sendersByHead maps a delivered payload's head to who sent it, for one pane.
//
// Only a delivery that LANDED counts. A refused or failed send never reached the pane, so
// its payload cannot be the turn in front of the reader, and attributing one would put
// another party's name on the reader's own words.
func sendersByHead(recs []events.Record, pane string) map[string]string {
	out := map[string]string{}
	for _, r := range recs {
		if r.Event != events.AuditEventSend || r.Pane != pane || r.Actor == "" {
			continue
		}
		state, payload, ok := strings.Cut(r.Summary, ": ")
		if !ok || events.Outcome(state) != diag.OK {
			continue
		}
		if k := mine.HeadKey(payload); k != "" {
			out[k] = r.Actor
		}
	}
	return out
}

// senderFor turns an actor into the sender a chat can draw, or nil when the actor is the
// person reading.
//
// "user", "phone:…", "browser:…" and "terminal:…" are all the READER: they are the
// channels they reach their own fleet through, and marking those would say "this was not
// you" about the one case where it was. Only a supervisor or another session is someone
// else. An unknown actor is treated as the reader for the same reason: the default has to
// be the answer that is right when we cannot tell.
func senderFor(actor string, resolve func(pane string) (label, agent string)) *transcript.Sender {
	switch {
	case actor == "hq":
		return &transcript.Sender{Kind: "hq", Label: "HQ"}
	case strings.HasPrefix(actor, "agent:"):
		pane := strings.TrimPrefix(actor, "agent:")
		label, agent := resolve(pane)
		if label == "" {
			label = pane
		}
		return &transcript.Sender{Kind: "agent", Label: label, Agent: agent, Pane: pane}
	default:
		return nil
	}
}

// paneLabel names a sending pane the way the radar does: its session, and the agent
// running in it so a surface can draw that agent's icon. Both are best-effort — a pane
// that has since closed still deserves its id on the message it sent.
func paneLabel(pane string) (string, string) {
	if tmux.Bin == "" {
		return "", ""
	}
	loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
	if loc == "" {
		return "", ""
	}
	label := tmux.Display(pane, "#{session_name}")
	if rec, ok := resume.Load(loc); ok {
		return label, rec.Agent
	}
	return label, ""
}
