package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func relaySenderFixture(t *testing.T, id, sid string) relay.Request {
	t.Helper()
	r, _, err := relay.Put(relay.Request{ID: id, Kind: "ask", Agent: "codex", HomeID: transcript.CodexHomeID(), ThreadID: sid, Session: "fixture", Pane: "%70", PanePID: "42", Body: "synthetic question", For: "hq"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = relay.Claim(r.ID); err != nil {
		t.Fatal(err)
	}
	r, err = relay.Resolve(r.ID, "Synthetic HQ coordination, not user authorization.\n")
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func acceptRelayFixture(t *testing.T, id string) {
	t.Helper()
	if err := relay.BeginReplyAttempt(id); err != nil {
		t.Fatal(err)
	}
	if err := relay.MarkDelivered(id); err != nil {
		t.Fatal(err)
	}
}
func relayPromptRecord(prompt string, at int64) string {
	b, _ := json.Marshal(map[string]any{"type": "event_msg", "timestamp": time.Unix(at, 0).UTC().Format(time.RFC3339), "payload": map[string]string{"type": "user_message", "message": prompt}})
	return string(b)
}
func decodeRelayTurns(t *testing.T, b []byte) []transcript.Turn {
	t.Helper()
	var ts []transcript.Turn
	if err := json.Unmarshal(b, &ts); err != nil {
		t.Fatal(err)
	}
	return ts
}
func TestHRS003ReceiptAwareSharedTranscript(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	r := relaySenderFixture(t, "native-reply", "native-thread")
	desktopRollout(t, root, r.ThreadID, "01", "codex-tui", relayPromptRecord(strings.TrimSpace(relayReplyText(r)), r.CreatedAt+2), `{"type":"event_msg","payload":{"type":"agent_message","message":"ack"}}`, relayPromptRecord(strings.TrimSpace(relayReplyText(r)), r.CreatedAt+60))
	b, m, err := desktopTranscriptRead(r.ThreadID, "")
	if err != nil {
		t.Fatal(err)
	}
	ts := decodeRelayTurns(t, b)
	if len(ts) != 2 || ts[0].From != nil || ts[1].From != nil {
		t.Fatal("unaccepted receipt attributed", string(b))
	}
	acceptRelayFixture(t, r.ID)
	b, n, err := desktopTranscriptRead(r.ThreadID, m.Etag)
	if err != nil {
		t.Fatal(err)
	}
	if b == nil || m.Etag == n.Etag {
		t.Fatal("receipt change retained stale human validator")
	}
	ts = decodeRelayTurns(t, b)
	if len(ts) != 2 || ts[0].From == nil || ts[0].From.Kind != "hq" || ts[0].From.Label != "HQ" || ts[1].From != nil {
		t.Fatal("shared JSON misattributed native HQ or later human copy", string(b))
	}
	if b, _, err = desktopTranscriptRead(r.ThreadID, n.Etag); err != nil || b != nil {
		t.Fatal("unchanged snapshot not cacheable", err)
	}
	if p := sessionpolicy.Get(r.ThreadID); p.HQ || p.Notify || p.Knowledge || p.Revision != 0 {
		t.Fatal("reading enabled observation", p)
	}
	got, _ := relay.Get(r.ID)
	if got.ReplyAttempt != "accepted" || got.Reply != r.Reply {
		t.Fatal("reading changed delivery")
	}
}
func TestHRS003EarlierPagesKeepFirstAttribution(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	r := relaySenderFixture(t, "history-reply", "history-thread")
	acceptRelayFixture(t, r.ID)
	rows := []string{relayPromptRecord(strings.TrimSpace(relayReplyText(r)), r.CreatedAt+1)}
	for i := 0; i < 309; i++ {
		rows = append(rows, relayPromptRecord(fmt.Sprintf("synthetic %d", i), r.CreatedAt+int64(i)+2))
	}
	rows = append(rows, relayPromptRecord(strings.TrimSpace(relayReplyText(r)), r.CreatedAt+1000))
	desktopRollout(t, root, r.ThreadID, "01", "codex_work_desktop", rows...)
	b, m, err := desktopTranscriptRead(r.ThreadID, "")
	if err != nil {
		t.Fatal(err)
	}
	ts := decodeRelayTurns(t, b)
	if len(ts) == 0 || ts[len(ts)-1].From != nil || m.History == nil {
		t.Fatal("later copy or history contract", string(b))
	}
	b, _, err = m.History(context.Background(), ts[0].HistoryBefore)
	if err != nil {
		t.Fatal(err)
	}
	earlier := decodeRelayTurns(t, b)
	if len(earlier) == 0 || earlier[0].Prompt != strings.TrimSpace(relayReplyText(r)) || earlier[0].From == nil || earlier[0].From.Kind != "hq" {
		t.Fatal("earlier page lost real HQ identity", string(b))
	}
}
