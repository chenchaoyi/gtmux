package hq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// Board and EventsJSON feed the remote HQ page (hq-command-page). Their contract is that
// "nothing to show" is an ordinary answer, and that a feed reads newest-first — the
// opposite of the CLI's log order.

func TestBoardReadsTextAndModTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "gtmux", "hq", "notes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const body = "# situation\n- %17 waiting on the inventory question"
	if err := os.WriteFile(filepath.Join(dir, "board.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	text, mod, ok := Board()
	if !ok || text != body {
		t.Fatalf("Board() = %q %v; want the file's text", text, ok)
	}
	if mod <= 0 || mod > time.Now().Unix()+1 {
		t.Errorf("mod time = %d; want a sane unix time", mod)
	}
}

// A machine with no HQ, or an HQ that hasn't written a board, is normal — not an error.
func TestBoardAbsentReportsNotOk(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if text, mod, ok := Board(); ok || text != "" || mod != 0 {
		t.Errorf("Board() with no HQ home = %q %d %v; want empty and not-ok", text, mod, ok)
	}
}

// A runaway board must not be pushed down a phone tunnel whole; the HEAD is kept because
// a board leads with its freshness line and current focus.
func TestBoardTruncatesToTheHead(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "gtmux", "hq", "notes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	big := append([]byte("HEAD-MARKER"), make([]byte, boardMaxBytes*2)...)
	if err := os.WriteFile(filepath.Join(dir, "board.md"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	text, _, ok := Board()
	if !ok || len(text) != boardMaxBytes {
		t.Fatalf("len = %d ok=%v; want capped at %d", len(text), ok, boardMaxBytes)
	}
	if text[:11] != "HEAD-MARKER" {
		t.Error("truncation dropped the head; the head is the part worth keeping")
	}
}

// writeEvents plants a ledger under a temp state dir, oldest line first.
func writeEvents(t *testing.T, recs []events.Record) {
	t.Helper()
	var buf []byte
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		buf = append(append(buf, b...), '\n')
	}
	if err := os.WriteFile(events.Path(), buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEventsJSONIsNewestFirstSeverityFlooredAndCapped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Dir(events.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	writeEvents(t, []events.Record{
		{Ts: now - 300, Seq: 1, Event: "UserPromptSubmit", Loc: "a:0.0", Severity: events.SevRoutine},
		{Ts: now - 200, Seq: 2, Event: "Stop", Loc: "b:0.0", Severity: events.SevNotable},
		{Ts: now - 100, Seq: 3, Event: "Waiting", Loc: "c:0.0", Kind: "question", Severity: events.SevImportant},
	})

	var got []events.Record
	b, err := EventsJSON(events.SevNotable, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records; want the 2 at/above notable (routine filtered out)", len(got))
	}
	if got[0].Seq != 3 {
		t.Errorf("first record seq = %d; want 3 — a feed reads newest first", got[0].Seq)
	}

	// The cap takes the NEWEST, not the oldest.
	b, _ = EventsJSON("", 1)
	got = nil
	_ = json.Unmarshal(b, &got)
	if len(got) != 1 || got[0].Seq != 3 {
		t.Errorf("limit 1 = %+v; want just the newest record", got)
	}
}

// A client must never have to special-case null.
func TestEventsJSONIsAlwaysAnArray(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, limit := range []int{0, 5} {
		b, err := EventsJSON("", limit)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "[]" {
			t.Errorf("EventsJSON(limit=%d) with no ledger = %s; want []", limit, b)
		}
	}
}
