package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestToolInputPreservesSourceAndSelectsOnlyPublicFields(t *testing.T) {
	command := "  printf '%s\\n' 'a b'\n  go test ./internal/transcript  "
	args, _ := json.Marshal(map[string]any{"cmd": command, "description": "Run checks", "workdir": "/example/repo", "token": "unrelated-secret"})
	s := toolStep("exec_command", "exec", args, "Run checks")
	if s.Input != command || s.InputKind != "command" || s.Workdir != "/example/repo" || s.Operation != "command" || s.Detail != "Run checks" {
		t.Fatalf("input lost its structure: %#v", s)
	}
	for _, raw := range []string{`{"token":"unrelated-secret"}`, `not JSON`, `{"cmd":["go",42]}`} {
		if got := toolStep("unknown", "unknown", json.RawMessage(raw), "summary"); got.Input != "" {
			t.Fatalf("unexpected raw argument dump: %#v", got)
		}
	}
	argv := toolStep("shell", "exec", json.RawMessage(`{"command":["printf","a b","c\nd"]}`), "printf")
	if argv.InputKind != "arguments" || argv.Input != `["printf","a b","c\nd"]` {
		t.Fatalf("argument boundaries lost: %#v", argv)
	}
}

func TestToolInputBoundsUTF8AndEachTurn(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"command": strings.Repeat("鲸", 10000), "cwd": strings.Repeat("a", 5000)})
	s := toolStep("Bash", "Bash", args, "summary")
	if !utf8.ValidString(s.Input) || len(s.Input) > maxToolInputBytes || len(s.Workdir) > 4096 || !s.DetailsTruncated {
		t.Fatalf("unbounded or invalid input: %d / %d", len(s.Input), len(s.Workdir))
	}
	st := &parseState{}
	for range 40 {
		st.addSteps([]Step{s})
	}
	total := 0
	for _, step := range st.cur.Segments[0].Steps {
		total += len(step.Input) + len(step.Workdir)
	}
	if total != st.toolDetailBytes || total > maxTurnToolDetailBytes || st.cur.Segments[0].Steps[39].Input != "" || !st.cur.Segments[0].Steps[39].DetailsTruncated {
		t.Fatalf("turn detail budget lost: %d / %d", total, st.toolDetailBytes)
	}
	st.open("next turn", "")
	st.addSteps([]Step{s})
	if st.cur.Segments[0].Steps[0].Input != s.Input {
		t.Fatal("budget did not reset at a new turn")
	}
}

func TestToolInputParticipatesInCacheBudget(t *testing.T) {
	small := []Turn{{Segments: []Segment{{Steps: []Step{{Kind: "tool", Title: "exec"}}}}}}
	large := []Turn{{Segments: []Segment{{Steps: []Step{{Kind: "tool", Title: "exec", Operation: "command", Input: strings.Repeat("a", 8192), InputKind: "command", Workdir: "/repo"}}}}}}
	if cachedTurnBytes(large)-cachedTurnBytes(small) < int64(8192+len("command")*2+len("/repo")) {
		t.Fatal("tool detail omitted from cache accounting")
	}
}

func TestCodexDesktopPageControlIsNotUserInput(t *testing.T) {
	control := `<external_codex_apps_open_page>{"page_id":null}</external_codex_apps_open_page>`
	for _, tc := range []struct{ raw, want string }{
		{control, ""},
		{control + "\nPlease inspect this repo", "Please inspect this repo"},
		{`<external_codex_apps_open_page>{"page_id":"example"}</external_codex_apps_open_page>`, ""},
		{"Explain " + control, "Explain " + control},
		{`<external_codex_apps_open_page>not JSON</external_codex_apps_open_page>`, `<external_codex_apps_open_page>not JSON</external_codex_apps_open_page>`},
		{`<external_codex_apps_open_page>{"different":null}</external_codex_apps_open_page>`, `<external_codex_apps_open_page>{"different":null}</external_codex_apps_open_page>`},
	} {
		got, ok := codexUserPrompt(tc.raw)
		if got != tc.want || ok != (tc.want != "") {
			t.Fatalf("prompt %q => %q / %t", tc.raw, got, ok)
		}
	}
	st := &parseState{}
	for _, prompt := range []string{control, "Actual user prompt"} {
		b, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]string{"type": "input_text", "text": prompt}}}})
		codexStep(string(b), st)
	}
	st.flush()
	if len(st.turns) != 1 || st.turns[0].Prompt != "Actual user prompt" {
		t.Fatalf("control became a turn: %#v", st.turns)
	}
}

func TestAllToolAdaptersExposeOriginalCommand(t *testing.T) {
	command := "printf 'a b'\n  pwd"
	args, _ := json.Marshal(map[string]string{"command": command, "cwd": "/example/repo"})
	for _, adapter := range []string{"claude", "codex", "kimi", "dsh"} {
		t.Run(adapter, func(t *testing.T) {
			st := &parseState{}
			var line any
			switch adapter {
			case "claude":
				line = map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "name": "Bash", "input": json.RawMessage(args)}}}}
			case "codex":
				line = map[string]any{"type": "response_item", "payload": map[string]any{"type": "function_call", "name": "shell", "arguments": string(args)}}
			case "kimi":
				line = map[string]any{"type": "context.append_message", "agentId": "main", "message": map[string]any{"role": "assistant", "toolCalls": []any{map[string]string{"name": "Shell", "arguments": string(args)}}}}
			case "dsh":
				line = map[string]any{"type": "tool/call", "surfaceOp": "append", "data": map[string]string{"name": "shell", "arguments": string(args)}}
			}
			b, _ := json.Marshal(line)
			switch adapter {
			case "claude":
				claudeStep(string(b), st)
			case "codex":
				codexStep(string(b), st)
			case "kimi":
				kimiStep(string(b), st)
			case "dsh":
				if err := dshStep(b, st); err != nil {
					t.Fatal(err)
				}
			}
			if st.cur == nil || len(st.cur.Segments) != 1 || len(st.cur.Segments[0].Steps) != 1 {
				t.Fatalf("missing tool input: %#v", st.cur)
			}
			step := st.cur.Segments[0].Steps[0]
			if step.Input != command || step.Workdir != "/example/repo" || step.Operation != "command" {
				t.Fatalf("adapter lost command: %#v", step)
			}
		})
	}
}

func TestCodexAppendedDetailsKeepTurnBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	args, _ := json.Marshal(map[string]string{"cmd": strings.Repeat("a", maxToolInputBytes)})
	row, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]string{"type": "function_call", "name": "exec_command", "arguments": string(args)}})
	line := string(row) + "\n"
	body := chatRecord("user_message", "inspect") + strings.Repeat(line, maxTurnToolDetailBytes/maxToolInputBytes)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	loader := &Loader{}
	before, err := loader.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(line)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("append: %v / %v", err, closeErr)
	}
	after, err := loader.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	steps := after[0].Segments[0].Steps
	if last := steps[len(steps)-1]; last.Input != "" || !last.DetailsTruncated {
		t.Fatal("incremental reload reset the turn budget")
	}
	if len(before[0].Segments[0].Steps) != maxTurnToolDetailBytes/maxToolInputBytes {
		t.Fatal("previous snapshot mutated")
	}
	if dshTurnBytes(after) < maxTurnToolDetailBytes {
		t.Fatal("DSH retention omits input bytes")
	}
}
