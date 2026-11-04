package transcript

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestKimi25ActualLoopToolCalls(t *testing.T) {
	b, err := os.ReadFile("testdata/kimi-2.5-selected.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	turns := runKimi(t, strings.Split(string(b), "\n"))
	if len(turns) != 1 || turns[0].Response != "Synthetic provider complete." {
		t.Fatalf("lost real 2.5 turn: %#v", turns)
	}
	var steps []Step
	for _, segment := range turns[0].Segments {
		steps = append(steps, segment.Steps...)
	}
	if len(steps) != 3 || steps[0].Operation != "read" || steps[1].Operation != "edit" || steps[2].Operation != "command" {
		t.Fatalf("lost real 2.5 tools: %#v", steps)
	}
	if !strings.Contains(steps[1].Input, `"old_string": "alpha"`) || !strings.Contains(steps[1].Input, `"new_string": "beta"`) {
		t.Fatalf("edit input missing: %#v", steps[1])
	}
}

func TestKimiToolMirrorsAndChildIsolation(t *testing.T) {
	lines := []string{
		`{"type":"context.append_message","message":{"role":"user","content":[{"type":"text","text":"work"}]}}`,
		`{"type":"context.append_loop_event","agentId":"main","event":{"type":"tool.call","toolCallId":"one","name":"Read","args":{"path":"a.go"}}}`,
		`{"type":"context.append_message","message":{"role":"assistant","toolCalls":[{"id":"one","name":"Read","arguments":"{\"path\":\"a.go\"}"}]}}`,
		`{"type":"context.append_loop_event","agentId":"agent-0","event":{"type":"tool.call","toolCallId":"child","name":"Bash","args":{"command":"child-secret"}}}`,
		`{"type":"context.append_loop_event","event":{"type":"tool.result","result":"private-result"}}`,
	}
	turns := runKimi(t, lines)
	b, _ := json.Marshal(turns)
	if len(turns) != 1 || len(turns[0].Segments[0].Steps) != 1 || strings.Contains(string(b), "private-result") || strings.Contains(string(b), "child-secret") {
		t.Fatalf("bad mirror/isolation: %s", b)
	}
}

func TestOpencodeRecordedToolsKeepFullScopedInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sid := "ses_fixture"
	AppendOpencode(sid, "user", "work")
	content := strings.Repeat("line\n", 3000)
	args, _ := json.Marshal(map[string]any{"kind": "tool", "id": "call1", "name": "write", "input": map[string]any{"path": "sample.go", "content": content, "apiKey": "not-for-transcript"}, "output": "not-for-transcript"})
	AppendOpencodeEvent(sid, args)
	AppendOpencodeEvent(sid, args)
	AppendOpencode(sid, "assistant", "done")
	path := opencodeLogPath(sid)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "not-for-transcript") {
		t.Fatal("retained unselected data")
	}
	turns, _, _, err := parseTurns(path, 0, opencodeStep)
	if err != nil {
		t.Fatal(err)
	}
	steps := turns[0].Segments[0].Steps
	if len(steps) != 1 || !steps[0].DetailsTruncated || steps[0].InputRef == "" {
		t.Fatalf("bad retained input: %#v", steps)
	}
	full, err := ToolInput("opencode", sid, steps[0].InputRef)
	if err != nil || full.DetailsTruncated || !strings.Contains(full.Input, `line\nline`) {
		t.Fatalf("full input unavailable: %#v %v", full, err)
	}
}
