package app

import (
	"context"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"strings"
	"testing"
)

func TestRBL003CodexReplyBodyNeverEntersQueuedOrLegacyInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	previous := relayNotifyActive
	t.Cleanup(func() { relayNotifyActive = previous })
	mutations := 0
	relayNotifyActive = func(ctx context.Context, target codexrpc.Target, id, text string, check func(string) bool) codexrpc.Outcome {
		if target.HomeID != transcript.CodexHomeID() || target.ThreadID != "thread-a" || strings.Contains(text, "PRIVATE REPLY") || !strings.Contains(text, "gtmux relay inbox") {
			t.Fatal("reply payload/identity leaked into hint", target, text)
		}
		if !check("active-turn") {
			return codexrpc.Outcome{Reason: "coalesced"}
		}
		mutations++
		return codexrpc.Outcome{Accepted: true, Attempted: true, ActiveTurnID: "active-turn"}
	}
	for _, id := range []string{"reply-aaa", "reply-bbb"} {
		r, _, e := relay.Put(relay.Request{ID: id, Kind: "ask", Agent: "codex", HomeID: transcript.CodexHomeID(), ThreadID: "thread-a", SourcePID: 12345, SourceStart: "synthetic", Session: "synthetic", Body: "question", For: "hq"})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = relay.Claim(id); e != nil {
			t.Fatal(e)
		}
		if r, e = relay.Resolve(id, "PRIVATE REPLY content"); e != nil {
			t.Fatal(e)
		}
		if deliverRelayReply(r.ID) != 0 {
			t.Fatal("durable inbox rejected")
		}
		saved, e := relay.Get(id)
		if e != nil || saved.Delivered || saved.ReplyAttempt != "" || saved.Reply != "PRIVATE REPLY content" {
			t.Fatal("notice mislabeled full delivery", saved, e)
		}
	}
	if mutations != 1 {
		t.Fatal("burst injected individual replies", mutations)
	}
	// Losing notification transport does not invoke another channel or consume reply.
	relayNotifyActive = func(context.Context, codexrpc.Target, string, string, func(string) bool) codexrpc.Outcome {
		return codexrpc.Outcome{Reason: "local-proxy-unavailable"}
	}
	if deliverRelayReply("reply-bbb") != 0 {
		t.Fatal("pull-only is not a send failure")
	}
	source := relaySource(callerIdentity{Agent: "codex", HomeID: transcript.CodexHomeID(), ThreadID: "thread-a"})
	if len(relay.Inbox(source, false, 20)) != 2 {
		t.Fatal("unread replies lost")
	}
}

func TestRBL003CompactNoticeHasVerifiedHQProvenanceAndHumanCopiesDoNot(t *testing.T) {
	r := relay.Request{ID: "reply-aaa", Agent: "codex", HomeID: "home-a", ThreadID: "thread-a", State: "replied", For: "hq", ReplyChannel: "inbox", Reply: "PRIVATE BODY", Notice: &relay.Notice{TurnID: "turn-a", At: 100, State: "accepted"}}
	text := relayInboxNoticeText(r)
	if strings.Contains(text, r.Reply) {
		t.Fatal("full reply was injected")
	}
	turns := []transcript.Turn{{Agent: "codex", SessionID: "thread-a", Prompt: text, RecordID: "first"}, {Agent: "codex", SessionID: "thread-a", Prompt: text, RecordID: "human-copy"}}
	get := func(string) (relay.Request, error) { return r, nil }
	witness := func(ctx context.Context, a, s string, p map[string]int64) map[string]string {
		if a != "codex" || s != "thread-a" || p[text] != 100 {
			t.Fatal("wrong source witness", a, s, p)
		}
		return map[string]string{text: "first"}
	}
	got, _ := stampRelaySendersWith(turns, "home-a", get, witness)
	if got[0].From == nil || got[0].From.Kind != "hq" || got[1].From != nil {
		t.Fatal("notice/copy attribution", got)
	}
	r.Notice.State = "unconfirmed"
	got, _ = stampRelaySendersWith([]transcript.Turn{{Agent: "codex", SessionID: "thread-a", Prompt: text, RecordID: "first"}}, "home-a", get, func(context.Context, string, string, map[string]int64) map[string]string {
		t.Fatal("unconfirmed notice requested witness")
		return nil
	})
	if got[0].From != nil {
		t.Fatal("unconfirmed notice gained authority")
	}
}
