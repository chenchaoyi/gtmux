package events

import (
	"strings"
	"testing"
)

// Every audit record must be a control record: the sensor-delta exclusion
// (IsControl) covers the trail with no further case analysis, and losing that
// nesting would let audit records satisfy zero-change gates.
func TestAuditNestsInsideControl(t *testing.T) {
	for _, ev := range []string{
		AuditEventWakeDelivered, AuditEventWakeDropped, AuditEventSend,
		AuditEventReap, AuditEventRotate, AuditEventHQSession, AuditEventKnowledge,
	} {
		r := Record{Event: ev}
		if !IsAudit(r) {
			t.Errorf("IsAudit(%q) = false", ev)
		}
		if !IsControl(r) {
			t.Errorf("IsControl(%q) = false — audit must nest inside control", ev)
		}
	}
	// The containment is strict: a maintenance trigger is control but NOT audit,
	// which is what keeps it counting toward the consumption debt.
	trigger := Record{Event: "gtmux:distill"}
	if IsAudit(trigger) {
		t.Fatal("IsAudit(gtmux:distill) = true — a maintenance trigger is not trail")
	}
	if !IsControl(trigger) {
		t.Fatal("IsControl(gtmux:distill) = false")
	}
}

func TestAuditLineBoundsAndCollapses(t *testing.T) {
	// Newlines collapse: one record is one journal line.
	if got := auditLine("a\nb\n\nc", 100); got != "a b c" {
		t.Fatalf("newline collapse: got %q", got)
	}
	// Under budget passes through untouched.
	if got := auditLine("short", 100); got != "short" {
		t.Fatalf("under budget: got %q", got)
	}
	// Over budget truncates at a rune boundary — multibyte text must never be
	// split mid-rune into invalid UTF-8.
	long := strings.Repeat("中", 100) // 300 bytes
	got := auditLine(long, 250)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated line missing ellipsis: %q", got)
	}
	body := strings.TrimSuffix(got, "…")
	if len(body) > 250 {
		t.Fatalf("truncated body %d bytes > budget 250", len(body))
	}
	for _, r := range body {
		if r != '中' {
			t.Fatalf("rune boundary broken: found %q", r)
		}
	}
}

// The constructors' field mapping, proven through a real Append/ReadSince
// round-trip in a redirected state dir — the shape every later consumer
// (unread tally, pull view, doctor) will read back.
func TestAuditConstructorsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	AuditWakeDelivered("%4", "» gtmux·done  %21 │ finished · #a1b2c3", 100)
	AuditWakeDropped(DropSuperseded, "» gtmux·resource·warn  disk low", 101)
	AuditSend("%28", "landed", "check the failing test", 102)
	AuditReap("t-9", "%21", "worktree removed, branch deleted", 103)
	AuditRotate("sess-old", "/clear", 104)
	AuditHQSession("sess-new", "sess-old", 105)
	AuditKnowledge("add pitfalls/x (capture pitfalls/x)", 106)

	recs, gap := ReadSince(0)
	if gap {
		t.Fatal("unexpected cursor gap on a fresh journal")
	}
	if len(recs) != 7 {
		t.Fatalf("got %d records, want 7", len(recs))
	}
	type want struct {
		event, pane, contains string
	}
	wants := []want{
		{AuditEventWakeDelivered, "%4", "#a1b2c3"},
		{AuditEventWakeDropped, "", DropSuperseded + ": "},
		{AuditEventSend, "%28", "landed: check the failing test"},
		{AuditEventReap, "%21", "t-9: worktree removed"},
		{AuditEventRotate, "", "session sess-old → reset (/clear)"},
		{AuditEventHQSession, "", "sess-new replaces sess-old"},
		{AuditEventKnowledge, "", "add pitfalls/x"},
	}
	for i, w := range wants {
		r := recs[i]
		if r.Event != w.event || r.Pane != w.pane || !strings.Contains(r.Summary, w.contains) {
			t.Errorf("record %d = {%s %q %q}, want {%s %q ~%q}",
				i, r.Event, r.Pane, r.Summary, w.event, w.pane, w.contains)
		}
		if r.Severity != SevRoutine {
			t.Errorf("record %d severity = %q, want routine — a trail never asks for attention", i, r.Severity)
		}
		if r.Seq == 0 {
			t.Errorf("record %d has no sequence", i)
		}
	}
}
