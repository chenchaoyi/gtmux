package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestDSHReaderUsesHomeAndLeavesSourceAndConsentUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "project", "web")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "session.v4.jsonl")
	raw := `{"type":"session","version":4,"id":"web","cwd":"/project"}` + "\n" + `{"type":"user/message","time":2000,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"Inspect tests"}]}}` + "\n" + `{"type":"assistant/message","time":3000,"data":{"message":{"role":"assistant","content":[{"type":"text","text":"Public reply"},{"type":"reasoning","text":"PRIVATE"}]}}}` + "\n"
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(file)
	b, meta, err := desktopTranscriptRead("web", "", dsh.HomeID())
	if err != nil || !strings.Contains(string(b), "Public reply") || strings.Contains(string(b), "PRIVATE") {
		t.Fatalf("public history: %s %v", b, err)
	}
	if b, _, err = desktopTranscriptRead("web", meta.Etag, dsh.HomeID()); err != nil || b != nil {
		t.Fatal("unchanged revision ignored", err)
	}
	if _, _, err = desktopTranscriptRead("web", "", "foreign"); err == nil {
		t.Fatal("foreign home read")
	}
	after, _ := os.Stat(file)
	bytes, _ := os.ReadFile(file)
	if string(bytes) != raw || !before.ModTime().Equal(after.ModTime()) || before.Mode() != after.Mode() {
		t.Fatal("reader changed DSH artifact")
	}
	if got := sessionpolicy.GetFor("dsh", "web"); got.HQ || got.Notify || got.Knowledge || got.Revision != 0 {
		t.Fatal("manual reading granted follow", got)
	}
}

func TestDSHHistoryRetainsHQSenderWithoutLivePaneAndRefreshesETag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	old := events.Sends
	events.Sends = &events.SendIndex{}
	t.Cleanup(func() { events.Sends = old })
	dir := filepath.Join(dsh.Home(), "sessions", "project", "archived")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	raw := `{"type":"session","version":4,"id":"archived"}` + "\n"
	for i, at := range []int64{now - 1000, now - 500, now - 100} {
		raw += fmt.Sprintf(`{"type":"user/message","seq":%d,"time":%d,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"Continue"}]}}`+"\n", i+1, at*1000)
	}
	file := filepath.Join(dir, "session.v4.jsonl")
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	appendJournal := func(records ...events.Record) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(events.Path()), 0700); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(events.Path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		for _, rec := range records {
			if err := json.NewEncoder(f).Encode(rec); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, at := range []int64{now - 1000, now - 500, now - 100} {
		appendJournal(events.Record{Seq: int64(i + 1), Ts: at, Event: "ConversationMessage", Pane: "%999", AgentKey: "dsh", HomeID: dsh.HomeID(), AgentSession: "archived", MessageID: fmt.Sprint(i), MessageSeq: int64(i + 1), MessageRole: "user", Summary: "Continue"})
	}
	_, before, err := desktopTranscriptRead("archived", "", dsh.HomeID())
	if err != nil {
		t.Fatal(err)
	}
	appendJournal(
		events.Record{Seq: 4, Ts: now - 999, Event: events.AuditEventSend, Pane: "%999", Actor: "hq", Summary: "landed: Continue"},
		events.Record{Seq: 5, Ts: now - 499, Event: events.AuditEventSend, Pane: "%999", Actor: "phone:synthetic", Summary: "landed: Continue"},
	)
	b, after, err := desktopTranscriptRead("archived", before.Etag, dsh.HomeID())
	if err != nil || b == nil || before.Etag == after.Etag {
		t.Fatalf("late delivery did not invalidate ETag: %v", err)
	}
	var turns []transcript.Turn
	if err := json.Unmarshal(b, &turns); err != nil || len(turns) != 3 {
		t.Fatalf("invalid bounded transcript: %v", err)
	}
	if turns[0].From == nil || turns[0].From.Kind != "hq" || turns[1].From != nil || turns[2].From != nil {
		t.Fatalf("HQ, phone and unmatched repeated prompt conflated: %+v", turns)
	}
	if b, _, err := desktopTranscriptRead("archived", after.Etag, dsh.HomeID()); err != nil || b != nil {
		t.Fatal("unchanged sender validator did not short-circuit", err)
	}
	bytes, _ := os.ReadFile(file)
	if string(bytes) != raw || sessionpolicy.GetFor("dsh", "archived").Revision != 0 {
		t.Fatal("attribution modified history or follow permission")
	}
}

func TestDSHTerminalRevocationDoesNotServeStalePaneHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	stub := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf 'synthetic:0.0\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	tmux.Bin = stub
	defer func() { tmux.Bin = old }()
	dir := filepath.Join(dsh.Home(), "sessions", "work", "synthetic")
	_ = os.MkdirAll(dir, 0700)
	_ = os.WriteFile(filepath.Join(dir, "session.v4.jsonl"), []byte(`{"type":"session","version":4,"id":"synthetic"}`+"\n"+`{"type":"user/message","seq":1,"time":2000,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"stale conversation text"}]}}`+"\n"), 0600)
	if err := resume.Save("synthetic:0.0", resume.Record{Agent: "dsh", HomeID: dsh.HomeID(), SessionID: "synthetic", TerminalClient: "dst"}); err != nil {
		t.Fatal(err)
	}
	b, _, err := transcriptForPane("%44", 0)
	if err != nil || strings.Contains(string(b), "stale conversation text") {
		t.Fatalf("revoked binding returned stale history: %s %v", b, err)
	}
}
