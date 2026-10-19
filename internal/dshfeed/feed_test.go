package dshfeed

import (
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (agentclient.Attachment, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	dsh.Sessions = &dsh.Catalog{}
	events.PublicMessages = &events.MessageIndex{}
	a := agentclient.Attachment{Agent: "dsh", HomeID: dsh.HomeID(), SessionID: "synthetic", Pane: "%77", StartedAt: 1000, BindingEpoch: 1000}
	path := filepath.Join(dsh.Home(), "sessions", "work", a.SessionID, "session.v4.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(dsh.Header{Type: "session", Version: 4, ID: a.SessionID, Cwd: "/synthetic", CreatedAt: 500})
	if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	return a, path
}
func appendText(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
func publicLine(seq int, text string) string {
	b, _ := json.Marshal(map[string]any{"type": "user/message", "seq": seq, "time": 2000 + seq, "data": map[string]any{"role": "user", "source": map[string]string{"kind": "user"}, "content": []any{map[string]string{"type": "text", "text": text}}}})
	return string(b) + "\n"
}
func messageCount() int {
	n := 0
	for _, r := range events.Read(0, time.Now().Unix()+1) {
		if r.Event == "ConversationMessage" {
			n++
		}
	}
	return n
}
func TestReconcileRestartPartialAppendAndExplicitReview(t *testing.T) {
	a, path := fixture(t)
	appendText(t, path, publicLine(1, "synthetic one"))
	x := &Reconciler{}
	x.Tick([]agentclient.Attachment{a})
	if messageCount() != 1 {
		t.Fatal("missing first message")
	}
	x.Tick([]agentclient.Attachment{a})
	(&Reconciler{}).Tick([]agentclient.Attachment{a})
	if messageCount() != 1 {
		t.Fatal("unchanged or restarted reader duplicated message")
	}
	line := publicLine(2, "synthetic two")
	appendText(t, path, line[:len(line)-3])
	x.Tick([]agentclient.Attachment{a})
	if messageCount() != 1 {
		t.Fatal("partial append emitted")
	}
	appendText(t, path, line[len(line)-3:])
	x.Tick([]agentclient.Attachment{a})
	if messageCount() != 2 {
		t.Fatal("completed append missing")
	}
	prior, err := Reviewed(a.HomeID, a.SessionID)
	if err != nil || prior != -1 {
		t.Fatal("reading events declared review")
	}
	page, err := dsh.ReadMessages(a.SessionID, -1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err = Review(a.HomeID, a.SessionID, 2, page); err != nil {
		t.Fatal(err)
	}
	if n, _ := Reviewed(a.HomeID, a.SessionID); n != 2 {
		t.Fatal("review not saved")
	}
	if Review(a.HomeID, a.SessionID, 3, page) == nil {
		t.Fatal("review jumped past returned messages")
	}
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path + ".old")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	x.Tick([]agentclient.Attachment{a})
	if n, _ := Reviewed(a.HomeID, a.SessionID); n != -1 {
		t.Fatal("replacement retained review claim")
	}
	found := false
	for _, r := range events.Read(0, time.Now().Unix()+1) {
		if r.Event == "ConversationGap" {
			found = true
		}
	}
	if !found {
		t.Fatal("replacement gap was silent")
	}
}
func TestReconcileDoesNotAdvanceOnDisabledJournalOrImportOlderWebMessages(t *testing.T) {
	a, path := fixture(t)
	appendText(t, path, publicLine(1, "synthetic"))
	cfg := filepath.Join(state.ConfigDir(), "config.json")
	_ = os.MkdirAll(filepath.Dir(cfg), 0700)
	_ = os.WriteFile(cfg, []byte(`{"eventsCapMB":0}`), 0600)
	x := &Reconciler{}
	x.Tick([]agentclient.Attachment{a})
	p, err := readProgress(a.HomeID, a.SessionID)
	if err != nil || p.SourceSeq != -1 {
		t.Fatalf("disabled journal advanced %+v %v", p, err)
	}
	_ = os.WriteFile(cfg, []byte(`{"eventsCapMB":20}`), 0600)
	a.StartedAt = 3000
	x.Tick([]agentclient.Attachment{a})
	if messageCount() != 0 {
		t.Fatal("older external message imported")
	}
	data, _ := os.ReadFile(progressPath(a.HomeID, a.SessionID))
	if strings.Contains(string(data), "synthetic\"") {
		t.Fatal("progress retained message body")
	}
}
