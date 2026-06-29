package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if turns[0].Prompt != "fix the build" || turns[0].Response != "Done — fixed it." {
		t.Fatalf("turn0 mismatch: %+v", turns[0])
	}
	if len(turns[0].Steps) != 1 || turns[0].Steps[0].Title != "Edit" || turns[0].Steps[0].Detail != "/x/main.go" {
		t.Fatalf("turn0 steps mismatch: %+v", turns[0].Steps)
	}
	if turns[1].Prompt != "now run tests" || turns[1].Response != "All green." {
		t.Fatalf("turn1 mismatch: %+v", turns[1])
	}
	if len(turns[1].Steps) != 1 || turns[1].Steps[0].Title != "Bash" {
		t.Fatalf("turn1 steps mismatch: %+v", turns[1].Steps)
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
	if turns[1].Prompt != "q2" || turns[1].Response != "a2 final" {
		t.Fatalf("turn1 not updated incrementally: %+v", turns[1])
	}
	if len(turns[1].Steps) != 1 || turns[1].Steps[0].Title != "Bash" {
		t.Fatalf("turn1 step not picked up: %+v", turns[1].Steps)
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
	if turns[0].Response != "Build passed." { // task_complete overrides agent_message
		t.Fatalf("response mismatch: %q", turns[0].Response)
	}
	if len(turns[0].Steps) != 1 || turns[0].Steps[0].Title != "exec" || turns[0].Steps[0].Detail != "go build ./..." {
		t.Fatalf("steps mismatch: %+v", turns[0].Steps)
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
