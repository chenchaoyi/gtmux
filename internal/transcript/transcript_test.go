package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// allSteps flattens every tool step across a turn's segments (steps now live per
// segment rather than in one flat list).
func allSteps(t Turn) []Step {
	var out []Step
	for _, s := range t.Segments {
		out = append(out, s.Steps...)
	}
	return out
}

// segTexts collects a turn's non-empty segment texts in order.
func segTexts(t Turn) []string {
	var out []string
	for _, s := range t.Segments {
		if s.Text != "" {
			out = append(out, s.Text)
		}
	}
	return out
}

// writeLog lays out a session log under a fake HOME so the glob-based loaders
// find it, and returns nothing (the test reads via Load).
func writeClaudeLog(t *testing.T, home, sessionID string, lines []string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", "-some-project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, sessionID+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeCodexLog(t *testing.T, home, sessionID string, lines []string) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "06", "28")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	name := "rollout-2026-06-28T10-00-00-" + sessionID + ".jsonl"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// CodexSessionForCwd derives a Codex pane's session id from disk (its notify
// carries none): the most-recent rollout whose session_meta.cwd matches wins.
func TestCodexSessionForCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")

	meta := func(sid, cwd string) []string {
		return []string{`{"timestamp":"t","type":"session_meta","payload":{"type":"session_meta","session_id":"` + sid + `","cwd":"` + cwd + `"}}`}
	}
	writeCodexLog(t, home, "old-sid", meta("old-sid", "/proj/a"))
	writeCodexLog(t, home, "other", meta("other", "/proj/b"))

	if sid, ok := CodexSessionForCwd("/proj/b"); !ok || sid != "other" {
		t.Errorf("cwd /proj/b → %q,%v; want other,true", sid, ok)
	}
	if _, ok := CodexSessionForCwd("/proj/none"); ok {
		t.Error("unknown cwd should not match")
	}
	if _, ok := CodexSessionForCwd(""); ok {
		t.Error("empty cwd should not match")
	}

	// A newer rollout for the same cwd must win (mtime-ordered).
	newer := filepath.Join(home, ".codex", "sessions", "2026", "06", "28", "rollout-2026-06-28T10-00-00-new-sid.jsonl")
	os.WriteFile(newer, []byte(meta("new-sid", "/proj/a")[0]+"\n"), 0o644)
	future := time.Now().Add(time.Hour)
	os.Chtimes(newer, future, future)
	if sid, ok := CodexSessionForCwd("/proj/a"); !ok || sid != "new-sid" {
		t.Errorf("cwd /proj/a → %q,%v; want new-sid,true (most recent)", sid, ok)
	}
}

func TestLoadClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "11111111-2222-3333-4444-555555555555"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"ai-title","title":"ignored"}`,
		`{"type":"user","message":{"role":"user","content":"fix the build"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"text","text":"Let me look"},{"type":"tool_use","name":"Edit","input":{"file_path":"/x/main.go"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"Done — fixed it."}]}}`,
		`{"type":"user","isMeta":true,"message":{"role":"user","content":"<command-name>/clear</command-name>"}}`,
		`{"type":"user","message":{"role":"user","content":"now run tests"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"All green."}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 turns, got %d: %+v", len(turns), turns)
	}
	// both assistant text segments are kept (preamble + closing), not just the last.
	if turns[0].Prompt != "fix the build" || turns[0].Response != "Let me look\n\nDone — fixed it." {
		t.Fatalf("turn0 mismatch: %+v", turns[0])
	}
	if len(allSteps(turns[0])) != 1 || allSteps(turns[0])[0].Title != "Edit" || allSteps(turns[0])[0].Detail != "/x/main.go" {
		t.Fatalf("turn0 steps mismatch: %+v", allSteps(turns[0]))
	}
	if turns[1].Prompt != "now run tests" || turns[1].Response != "All green." {
		t.Fatalf("turn1 mismatch: %+v", turns[1])
	}
	if len(allSteps(turns[1])) != 1 || allSteps(turns[1])[0].Title != "Bash" {
		t.Fatalf("turn1 steps mismatch: %+v", allSteps(turns[1]))
	}
}

// The prompt's log timestamp is captured onto Turn.Time (chat-view time label).
func TestLoadClaudeCapturesPromptTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","timestamp":"2026-06-15T04:15:35.802Z","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","timestamp":"2026-06-15T04:15:40.000Z","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"hi"}]}}`,
	})
	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("want 1 turn, got %d", len(turns))
	}
	if turns[0].Time != "2026-06-15T04:15:35.802Z" {
		t.Fatalf("want prompt timestamp, got %q", turns[0].Time)
	}
}

// Codex turns carry the user_message line's timestamp too.
func TestLoadCodexCapturesPromptTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	sid := "ffffffff-1111-2222-3333-444444444444"
	writeCodexLog(t, home, sid, []string{
		`{"timestamp":"2026-06-28T10:00:01.000Z","type":"event_msg","payload":{"type":"user_message","message":"build it"}}`,
		`{"timestamp":"2026-06-28T10:00:05.000Z","type":"event_msg","payload":{"type":"agent_message","message":"done"}}`,
	})
	turns, err := Load("Codex", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || turns[0].Time != "2026-06-28T10:00:01.000Z" {
		t.Fatalf("codex time mismatch: %+v", turns)
	}
}

// A turn's reply spread across several assistant messages (text → tool → text →
// tool → text) keeps EVERY text segment, not just the last one.
func TestLoadClaudeKeepsAllReplySegments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"do the thing"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"First, some context."},{"type":"tool_use","name":"Read","input":{"file_path":"/a"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Now the middle bit."},{"type":"tool_use","name":"Edit","input":{"file_path":"/a"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"ok"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"And the conclusion."}]}}`,
	})
	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := "First, some context.\n\nNow the middle bit.\n\nAnd the conclusion."
	if len(turns) != 1 || turns[0].Response != want {
		t.Fatalf("want all 3 segments %q, got %+v", want, turns)
	}
	wantSegs := []string{"First, some context.", "Now the middle bit.", "And the conclusion."}
	got := segTexts(turns[0])
	if len(got) != len(wantSegs) {
		t.Fatalf("want %d segment texts, got %v", len(wantSegs), got)
	}
	for i, s := range wantSegs {
		if got[i] != s {
			t.Fatalf("segment %d = %q, want %q", i, got[i], s)
		}
	}
	// the Read step attaches to its own segment (text "First…"), the Edit to the
	// second — i.e. steps interleave with the bubbles in order.
	if len(turns[0].Segments) != 3 || len(turns[0].Segments[0].Steps) != 1 || turns[0].Segments[0].Steps[0].Title != "Read" {
		t.Fatalf("segment/step interleave wrong: %+v", turns[0].Segments)
	}
	if len(turns[0].Segments[1].Steps) != 1 || turns[0].Segments[1].Steps[0].Title != "Edit" {
		t.Fatalf("segment 1 step wrong: %+v", turns[0].Segments[1])
	}
}

// Harness-injected user turns (<task-notification>, <system-reminder>) must not
// show up as chat prompts; a reminder appended to a real prompt is trimmed off.
func TestLoadClaudeSkipsHarnessBlocks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "22222222-3333-4444-5555-666666666666"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"real question"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"an answer"}]}}`,
		// a pure background-task notice injected as a user turn — NOT a prompt
		`{"type":"user","message":{"role":"user","content":"<task-notification>\n<task-id>abc</task-id>\n<status>completed</status>\n</task-notification>"}}`,
		// a pure context reminder injected as a user turn — NOT a prompt
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"<system-reminder>be nice</system-reminder>"}]}}`,
		// a real prompt with a reminder appended by the harness — keep only the prompt
		`{"type":"user","message":{"role":"user","content":"second question\n<system-reminder>context here</system-reminder>"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"second answer"}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 turns (harness blocks dropped), got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "real question" {
		t.Fatalf("turn0 prompt mismatch: %q", turns[0].Prompt)
	}
	if turns[1].Prompt != "second question" || turns[1].Response != "second answer" {
		t.Fatalf("turn1 mismatch: %+v", turns[1])
	}
}

// Rejecting a tool with "No, and tell Claude what to do" embeds the typed message
// in the rejection tool_result — it must surface as a user turn (it used to vanish).
func TestLoadClaudeShowsRejectFeedback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "33333333-4444-5555-6666-777777777777"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"edit main.go"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/x/main.go"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"The user doesn't want to proceed with this tool use. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). To tell you how to proceed, the user said:\nuse a map instead of a slice"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"Got it, switching to a map."}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 turns (reject feedback opens one), got %d: %+v", len(turns), turns)
	}
	if turns[1].Prompt != "use a map instead of a slice" {
		t.Fatalf("reject-feedback prompt mismatch: %q", turns[1].Prompt)
	}
	if turns[1].Response != "Got it, switching to a map." {
		t.Fatalf("turn1 response mismatch: %q", turns[1].Response)
	}
}

// A normal tool OUTPUT result and a plain rejection (no typed feedback) must NOT
// open a turn — only a rejection that carries the user's own message does.
func TestLoadClaudePlainRejectionNotATurn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "44444444-5555-6666-7777-888888888888"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"do it"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"file1\nfile2"}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"The user doesn't want to proceed with this tool use. STOP what you are doing and wait for the user to tell you how to proceed."}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("want 1 turn (output/plain-reject open none), got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "do it" {
		t.Fatalf("turn0 prompt mismatch: %q", turns[0].Prompt)
	}
}

func TestLoadClaudeMaxTurns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines,
			`{"type":"user","message":{"role":"user","content":"q"}}`,
			`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"a"}]}}`,
		)
	}
	writeClaudeLog(t, home, sid, lines)
	turns, _ := Load("claude", sid, 2)
	if len(turns) != 2 {
		t.Fatalf("want 2 (last-N), got %d", len(turns))
	}
}

// appendClaudeLog appends lines to an existing session log (simulating a growing
// live log between polls).
func appendClaudeLog(t *testing.T, home, sessionID string, lines []string) {
	t.Helper()
	p := filepath.Join(home, ".claude", "projects", "-some-project", sessionID+".jsonl")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

// A growing log must re-parse incrementally: the last cached turn picks up its
// appended content, and new turns are spliced on — without dropping or
// duplicating turns. This exercises Loader's growth path.
func TestLoadClaudeIncrementalGrowth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "abababab-cdcd-efef-0101-202020202020"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"q1"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a1"}]}}`,
		`{"type":"user","message":{"role":"user","content":"q2"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a2 partial"}]}}`,
	})

	turns, _ := Load("claude", sid, 50)
	if len(turns) != 2 || turns[1].Prompt != "q2" || turns[1].Response != "a2 partial" {
		t.Fatalf("pre-growth mismatch: %+v", turns)
	}

	// The open turn (q2) gains more output, then a brand-new turn (q3) arrives.
	appendClaudeLog(t, home, sid, []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a2 final"}]}}`,
		`{"type":"user","message":{"role":"user","content":"q3"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a3"}]}}`,
	})

	turns, _ = Load("claude", sid, 50)
	if len(turns) != 3 {
		t.Fatalf("want 3 turns after growth, got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "q1" || turns[0].Response != "a1" {
		t.Fatalf("turn0 clobbered: %+v", turns[0])
	}
	// q2 accumulates both reply segments across the Bash step (text → tool → text).
	if turns[1].Prompt != "q2" || turns[1].Response != "a2 partial\n\na2 final" {
		t.Fatalf("turn1 not updated incrementally: %+v", turns[1])
	}
	if len(allSteps(turns[1])) != 1 || allSteps(turns[1])[0].Title != "Bash" {
		t.Fatalf("turn1 step not picked up: %+v", allSteps(turns[1]))
	}
	if turns[2].Prompt != "q3" || turns[2].Response != "a3" {
		t.Fatalf("turn2 (new) mismatch: %+v", turns[2])
	}
}

func TestLoadCodex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	sid := "019efd9f-2183-79b3-8c8d-7cef4444cc31"
	writeCodexLog(t, home, sid, []string{
		`{"type":"session_meta","payload":{"id":"` + sid + `","cwd":"/x"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"<permissions>"}]}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"build the app"}}`,
		`{"type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go build ./...\"}"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{}}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"working on it"}}`,
		`{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":"Build passed."}}`,
	})

	turns, err := Load("Codex", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("want 1 turn, got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "build the app" {
		t.Fatalf("prompt mismatch: %q", turns[0].Prompt)
	}
	// both the interim agent_message and the task_complete final are kept.
	if turns[0].Response != "working on it\n\nBuild passed." {
		t.Fatalf("response mismatch: %q", turns[0].Response)
	}
	if len(allSteps(turns[0])) != 1 || allSteps(turns[0])[0].Title != "exec" || allSteps(turns[0])[0].Detail != "go build ./..." {
		t.Fatalf("steps mismatch: %+v", allSteps(turns[0]))
	}
}

func TestLoadUnknownAgentOrMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if turns, err := Load("aider", "x", 10); err != nil || turns != nil {
		t.Fatalf("unknown agent should be (nil,nil), got %v %v", turns, err)
	}
	if turns, err := Load("claude", "does-not-exist", 10); err != nil || turns != nil {
		t.Fatalf("missing log should be (nil,nil), got %v %v", turns, err)
	}
	if turns, err := Load("claude", "", 10); err != nil || turns != nil {
		t.Fatalf("empty sessionID should be (nil,nil), got %v %v", turns, err)
	}
}

// TestLastActivityForCwd: the newest session-log mtime for a cwd, in unix
// SECONDS (not nanoseconds), with the '/'→'-' project-dir encoding.
func TestLastActivityForCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/Users/x/proj"
	dir := filepath.Join(home, ".claude", "projects", "-Users-x-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "old.jsonl")
	newer := filepath.Join(dir, "new.jsonl")
	for _, p := range []string{old, newer} {
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := time.Unix(1700000000, 0)
	os.Chtimes(old, time.Unix(1600000000, 0), time.Unix(1600000000, 0))
	os.Chtimes(newer, want, want)

	got := LastActivityForCwd(cwd)
	if got != want.Unix() {
		t.Errorf("LastActivityForCwd = %d, want the newest log's mtime in SECONDS %d", got, want.Unix())
	}
	if LastActivityForCwd("/no/such/cwd") != 0 {
		t.Error("unknown cwd should return 0")
	}
}
