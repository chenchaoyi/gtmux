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
