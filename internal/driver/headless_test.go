package driver

import (
	"strings"
	"testing"
)

func TestHeadlessRegistration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{"claude", "codex"} {
		if For(k).Headless == nil {
			t.Errorf("For(%q).Headless = nil; it has a structured one-shot mode", k)
		}
	}
	for _, k := range []string{"gemini", "cursor", "unknown"} {
		if For(k).Headless != nil {
			t.Errorf("For(%q).Headless must be nil", k)
		}
	}
	writeConfig(t, `{"driver": {"claude": {"headless": false}}}`)
	d := For("claude")
	if d.Headless != nil {
		t.Error("driver.claude.headless=false must strip Headless")
	}
	if d.Receipt == nil || d.Content == nil {
		t.Error("the headless switch must not touch other capabilities")
	}
}

func TestHeadlessArgs(t *testing.T) {
	got := strings.Join(claudeHeadless.Args("fix the bug", "haiku"), " ")
	want := "claude -p fix the bug --output-format stream-json --verbose --model haiku"
	if got != want {
		t.Errorf("claude args = %q, want %q", got, want)
	}
	got = strings.Join(codexHeadless.Args("fix the bug", ""), " ")
	if got != "codex exec --json fix the bug" {
		t.Errorf("codex args = %q", got)
	}
}

// The claude stream: init names the session, result closes the run; anything
// unknown (or unparsable) changes nothing — the tolerance contract.
func TestClaudeHeadlessParse(t *testing.T) {
	var o HeadlessOutcome
	for _, line := range []string{
		`{"type":"system","subtype":"init","session_id":"abc-123"}`,
		`{"type":"assistant","message":{"content":"…"}}`,
		`not json at all`,
		`{"type":"future-event-kind","x":1}`,
	} {
		claudeHeadless.ParseLine(line, &o)
	}
	if o.Done || o.Failed || o.Session != "abc-123" {
		t.Fatalf("mid-run state wrong: %+v", o)
	}
	claudeHeadless.ParseLine(`{"type":"result","subtype":"success","is_error":false,"result":"done: 3 files changed","session_id":"abc-123"}`, &o)
	if !o.Done || o.Failed || o.Summary != "done: 3 files changed" {
		t.Fatalf("success result: %+v", o)
	}

	var e HeadlessOutcome
	claudeHeadless.ParseLine(`{"type":"result","subtype":"error_max_turns","is_error":true}`, &e)
	if !e.Done || !e.Failed || e.Summary != "error_max_turns" {
		t.Fatalf("error result: %+v", e)
	}
}

// The codex stream: events may nest their type under msg; task_complete and
// error are terminal, everything else is ignored.
func TestCodexHeadlessParse(t *testing.T) {
	var o HeadlessOutcome
	for _, line := range []string{
		`{"id":"0","msg":{"type":"agent_message","message":"thinking…"}}`,
		`garbage`,
	} {
		codexHeadless.ParseLine(line, &o)
	}
	if o.Done || o.Failed {
		t.Fatalf("mid-run state wrong: %+v", o)
	}
	codexHeadless.ParseLine(`{"id":"1","msg":{"type":"task_complete","last_agent_message":"all tests green"}}`, &o)
	if !o.Done || o.Failed || o.Summary != "all tests green" {
		t.Fatalf("task_complete: %+v", o)
	}

	var e HeadlessOutcome
	codexHeadless.ParseLine(`{"id":"2","msg":{"type":"error","message":"model overloaded"}}`, &e)
	if !e.Done || !e.Failed || e.Summary != "model overloaded" {
		t.Fatalf("error: %+v", e)
	}
	// A flat (un-nested) terminal event parses too.
	var f HeadlessOutcome
	codexHeadless.ParseLine(`{"type":"task_complete"}`, &f)
	if !f.Done || f.Failed {
		t.Fatalf("flat task_complete: %+v", f)
	}
}
