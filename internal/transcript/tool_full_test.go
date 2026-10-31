package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFullToolInputOutlivesBothPreviewBudgets(t *testing.T) {
	source := "const value = '" + strings.Repeat("鲸🙂", 5000) + "';\n// complete end"
	raw, _ := json.Marshal(map[string]string{"command": source, "workdir": "/example"})
	custom, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]string{"type": "custom_tool_call", "name": "exec", "input": source}})
	legacy, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]string{"type": "function_call", "name": "exec_command", "arguments": string(raw)}})
	claude, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "name": "Bash", "input": json.RawMessage(raw)}, map[string]any{"type": "tool_use", "name": "Bash", "input": json.RawMessage(raw)}}}})
	for _, tc := range []struct {
		name   string
		line   []byte
		parser stepFn
	}{{"custom", custom, codexStep}, {"legacy", legacy, codexStep}, {"claude-multiple", claude, claudeStep}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "calls.jsonl")
			content := "{}\n" + string(tc.line) + "\n"
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			st := &parseState{sourceID: recordSourceID(path), off: 3, toolDetailBytes: maxTurnToolDetailBytes}
			st.cur = &Turn{Prompt: "run"} // exhaust this turn before the call
			tc.parser(string(tc.line), st)
			for _, s := range st.cur.Segments[len(st.cur.Segments)-1].Steps {
				if !s.DetailsTruncated || s.Input != "" || s.InputRef == "" || s.fullInput != "" {
					t.Fatal("preview must be bounded with a reader", s)
				}
				full, err := readToolInput(path, tc.parser, 3, s.InputRef)
				if err != nil || full.Input != source || full.DetailsTruncated {
					t.Fatal("full content unavailable", len(full.Input), err)
				}
				if _, err := readToolInput(path, tc.parser, 0, s.InputRef); err == nil {
					t.Fatal("offset crossed source record")
				}
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(content, "complete end", "rewritten end")), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := readToolInput(path, tc.parser, 3, s.InputRef); err == nil {
					t.Fatal("rewritten record accepted")
				}
				os.WriteFile(path, []byte(content), 0600)
			}
		})
	}
}

func TestFullParametersRetainLongMessagesAndArraysWithoutUnselectedFields(t *testing.T) {
	message := strings.Repeat("hello", 5000)
	args, _ := json.Marshal(map[string]any{"message": message, "token": "never expose"})
	st := &parseState{sourceID: "source", fullTools: true}
	st.addSteps([]Step{toolStep("spawn_agent", "spawn_agent", args, "")})
	s := st.cur.Segments[0].Steps[0]
	if !strings.Contains(s.Input, message) || strings.Contains(s.Input, "never expose") || s.DetailsTruncated {
		t.Fatal("parameter selection changed")
	}
}

func TestKimiOriginalInputCanBeReadIndependently(t *testing.T) {
	source := strings.Repeat("hello\n", 5000)
	args, _ := json.Marshal(map[string]string{"command": source})
	row, _ := json.Marshal(map[string]any{"type": "context.append_message", "agentId": "main", "message": map[string]any{"role": "assistant", "toolCalls": []any{map[string]string{"name": "Shell", "arguments": string(args)}}}})
	path := filepath.Join(t.TempDir(), "wire.jsonl")
	os.WriteFile(path, append(row, '\n'), 0600)
	st := &parseState{sourceID: recordSourceID(path)}
	kimiStep(string(row), st)
	step := st.cur.Segments[0].Steps[0]
	full, err := readToolInput(path, kimiStep, 0, step.InputRef)
	if err != nil || full.Input != source || full.DetailsTruncated {
		t.Fatal(len(full.Input), err)
	}
}
