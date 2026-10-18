package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHQSessionIndexReadsOnlyTheTail(t *testing.T) {
	withJournal(t, []Record{
		{Event: "Stop", Summary: "unrelated"},
		{Event: AuditEventHQSession, Summary: "b replaces a"},
		{Event: AuditEventHQSession, AgentSession: "c", PreviousAgentSession: "b", PreviousAgentKey: "claude"},
	})
	x := &HQSessionIndex{}
	if id, agent := x.Predecessor("c"); id != "b" || agent != "claude" {
		t.Fatalf("link=%q/%q", id, agent)
	}
	if id, _ := x.Predecessor("b"); id != "a" {
		t.Fatalf("legacy link=%q", id)
	}
	before := x.parsed
	for i := 0; i < 20; i++ {
		x.Predecessor("c")
		x.Predecessor("missing")
	}
	if x.parsed != before {
		t.Fatal("unchanged polls decoded the journal again")
	}
	appendRecords(t, Path(), Record{Event: AuditEventHQSession, AgentSession: "c", PreviousAgentSession: "new", PreviousAgentKey: "codex"}, Record{Event: "Stop"})
	if id, agent := x.Predecessor("c"); id != "new" || agent != "codex" {
		t.Fatalf("newest link=%q/%q", id, agent)
	}
	if x.parsed-before != 1 {
		t.Fatalf("tail read decoded %d records", x.parsed-before)
	}
}

func TestHQSessionIndexRotationTruncationAndPartialAppend(t *testing.T) {
	withJournal(t, []Record{{Event: AuditEventHQSession, Summary: "b replaces a"}})
	x := &HQSessionIndex{}
	x.Predecessor("b")
	if err := os.Rename(Path(), rotatedPath()); err != nil {
		t.Fatal(err)
	}
	if id, _ := x.Predecessor("b"); id != "a" {
		t.Fatal("link lost between rotation and next append")
	}
	appendRecords(t, Path(), Record{Event: AuditEventHQSession, Summary: "c replaces b"})
	if id, _ := x.Predecessor("b"); id != "a" {
		t.Fatal("rotated link was lost")
	}
	if id, _ := x.Predecessor("c"); id != "b" {
		t.Fatal("new generation was missed")
	}
	// A second rotation must evict the now-unretained first generation.
	if err := os.Rename(Path(), rotatedPath()); err != nil {
		t.Fatal(err)
	}
	appendRecords(t, Path(), Record{Event: AuditEventHQSession, Summary: "d replaces c"})
	if id, _ := x.Predecessor("b"); id != "" {
		t.Fatal("expired generation remained indexed")
	}
	if err := os.WriteFile(Path(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if id, _ := x.Predecessor("d"); id != "" {
		t.Fatal("truncated link remained indexed")
	}
	b, _ := json.Marshal(Record{Event: AuditEventHQSession, Summary: "e replaces d"})
	f, err := os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.Write(b[:len(b)/2])
	if id, _ := x.Predecessor("e"); id != "" {
		t.Fatal("partial record was accepted")
	}
	_, _ = f.Write(append(b[len(b)/2:], '\n'))
	if id, _ := x.Predecessor("e"); id != "d" {
		t.Fatal("completed record was lost")
	}
}

func BenchmarkHQSessionIndexUnchanged(b *testing.B) {
	b.Setenv("HOME", b.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0o755); err != nil {
		b.Fatal(err)
	}
	f, err := os.Create(Path())
	if err != nil {
		b.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for i := 0; i < 80000; i++ {
		_ = enc.Encode(Record{Event: "Stop", Summary: "unrelated lifecycle event"})
	}
	_ = enc.Encode(Record{Event: AuditEventHQSession, Summary: "b replaces a"})
	f.Close()
	x := &HQSessionIndex{}
	x.Predecessor("b")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Predecessor("b")
	}
}
