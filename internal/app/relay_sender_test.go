package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestHRS002TrustedReceiptAndFirstSourceOnly(t *testing.T) {
	good := relay.Request{ID: "synthetic-id", Agent: "codex", HomeID: "home-a", ThreadID: "thread-a", For: "hq", State: "replied", Delivered: true, ReplyAttempt: "accepted", CreatedAt: 100, Session: "original pane", Pane: "%7", Reply: "Coordination only\n"}
	prompt := strings.TrimSpace(relayReplyText(good))
	base := transcript.Turn{Agent: "codex", SessionID: "thread-a", Prompt: prompt, RecordID: "first", Time: time.Unix(102, 0).UTC().Format(time.RFC3339)}
	for _, c := range []struct {
		name string
		edit func(*relay.Request, *transcript.Turn)
		home string
		want bool
	}{
		{name: "native accepted including historical pane", home: "home-a", want: true},
		{name: "missing receipt", edit: func(r *relay.Request, _ *transcript.Turn) { r.ID = "" }, home: "home-a"},
		{name: "pending", edit: func(r *relay.Request, _ *transcript.Turn) { r.State = "pending" }, home: "home-a"},
		{name: "unconfirmed", edit: func(r *relay.Request, _ *transcript.Turn) { r.Delivered = false; r.ReplyAttempt = "submitting" }, home: "home-a"},
		{name: "legacy without thread acceptance", edit: func(r *relay.Request, _ *transcript.Turn) { r.ReplyAttempt = "" }, home: "home-a"},
		{name: "different home", home: "home-b"},
		{name: "different thread", edit: func(_ *relay.Request, t *transcript.Turn) { t.SessionID = "thread-b" }, home: "home-a"},
		{name: "not codex", edit: func(_ *relay.Request, t *transcript.Turn) { t.Agent = "claude" }, home: "home-a"},
		{name: "user decision", edit: func(r *relay.Request, _ *transcript.Turn) { r.For = "user" }, home: "home-a"},
		{name: "tampered body", edit: func(_ *relay.Request, t *transcript.Turn) { t.Prompt += " extra" }, home: "home-a"},
		{name: "header quoted", edit: func(_ *relay.Request, t *transcript.Turn) { t.Prompt = "> " + t.Prompt }, home: "home-a"},
		{name: "later human copy", edit: func(_ *relay.Request, t *transcript.Turn) { t.RecordID = "later" }, home: "home-a"},
		{name: "missing source", edit: func(_ *relay.Request, t *transcript.Turn) { t.RecordID = "" }, home: "home-a"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, turn := good, base
			if c.edit != nil {
				c.edit(&r, &turn)
			}
			unchanged := r
			got, _ := stampRelaySendersWith([]transcript.Turn{turn}, c.home, func(id string) (relay.Request, error) {
				if r.ID == "" {
					return r, errors.New("missing")
				}
				if id != good.ID {
					t.Fatal(id)
				}
				return r, nil
			}, func(_ context.Context, a, s string, p map[string]int64) map[string]string {
				if a != "codex" || s != good.ThreadID || p[prompt] != 100 {
					t.Fatal("unverified witness scope", a, s, p)
				}
				return map[string]string{prompt: "first"}
			})
			yes := got[0].From != nil && got[0].From.Kind == "hq" && got[0].From.Label == "HQ"
			if yes != c.want {
				t.Fatalf("sender=%+v wantHQ=%v", got[0].From, c.want)
			}
			if !reflect.DeepEqual(r, unchanged) {
				t.Fatal("receipt mutated")
			}
		})
	}
	existing := &transcript.Sender{Kind: "agent", Label: "recorded colleague"}
	base.From = existing
	got, _ := stampRelaySendersWith([]transcript.Turn{base}, "home-a", func(string) (relay.Request, error) { return good, nil }, func(context.Context, string, string, map[string]int64) map[string]string {
		return map[string]string{prompt: "first"}
	})
	if got[0].From != existing {
		t.Fatal("overrode recorded sender")
	}
}
