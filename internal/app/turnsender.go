package app

import (
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
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

// stampSenders fills in Turn.From for the turns gtmux delivered into this pane on
// someone else's behalf. Turns it cannot attribute are returned untouched.
//
// The deliveries come from events.Sends, which reads only what the journal gained since
// the last request. This used to parse the whole journal for every chat request, about
// 110ms on a real 16 MB journal, every 8 seconds for as long as a chat was open.
func stampSenders(turns []transcript.Turn, pane string, now int64) []transcript.Turn {
	if pane == "" || len(turns) == 0 {
		return turns
	}
	return stampSendersWith(turns, events.Sends.Senders(pane, now-sendLookback(turns, now)), paneLabel)
}

// stampSendersWith is the join itself, over deliveries already looked up: the half worth
// testing without a journal on disk. resolve is asked once per sending pane, not once per
// turn: it runs tmux, and a long conversation with one colleague used to ask it the same
// question for every message that colleague sent.
func stampSendersWith(turns []transcript.Turn, by map[string]string, resolve func(string) (string, string)) []transcript.Turn {
	if len(by) == 0 {
		return turns
	}
	type named struct{ label, agent string }
	seen := map[string]named{}
	once := func(pane string) (string, string) {
		n, ok := seen[pane]
		if !ok {
			n.label, n.agent = resolve(pane)
			seen[pane] = n
		}
		return n.label, n.agent
	}
	for i := range turns {
		if turns[i].From != nil || turns[i].Prompt == "" {
			continue
		}
		if s := senderFor(events.MatchHead(by, turns[i].Prompt), once); s != nil {
			turns[i].From = s
		}
	}
	return turns
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
