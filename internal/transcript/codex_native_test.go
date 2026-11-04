package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func nativeTestRecord(kind string, values map[string]any) string {
	body, _ := json.Marshal(values)
	return fmt.Sprintf("{\"type\":\"event_msg\",\"payload\":{\"type\":\"item_completed\",\"item\":{\"type\":%q,%s}}\n", kind, body[1:])
}

func nativeTestSteps(st *parseState) []Step {
	var steps []Step
	for _, turn := range st.turns {
		for _, segment := range turn.Segments {
			steps = append(steps, segment.Steps...)
		}
	}
	return steps
}

func TestCodexNativeActionsAppearBesideOriginalCodeWithoutResults(t *testing.T) {
	read := map[string]any{"id": "command-1", "command": []string{"bash", "-lc", "cat sample.go"}, "cwd": "/example", "status": "completed",
		"parsed_cmd": []any{map[string]string{"type": "read", "path": "sample.go", "name": "sample.go"}}, "aggregated_output": "PRIVATE COMMAND OUTPUT"}
	change := map[string]any{"id": "file-1", "status": "completed", "changes": map[string]any{
		"sample.go": map[string]string{"type": "update", "unified_diff": "@@ -1 +1 @@\n-old\n+new\n", "move_path": "new.go"}}, "stdout": "PRIVATE STDOUT", "stderr": "PRIVATE STDERR"}
	input := chatRecord("user_message", "Read and update the synthetic file") + customRecord("exec", "await tools.exec_command({cmd:'cat sample.go'});") +
		nativeTestRecord("CommandExecution", read) + nativeTestRecord("FileChange", change) +
		nativeTestRecord("McpToolCall", map[string]any{"id": "mcp-1", "tool": "github_fetch_file", "arguments": map[string]string{"path": "README.md", "credential": "PRIVATE CONFIG"}, "result": "PRIVATE MCP RESULT"}) +
		nativeTestRecord("Extension", map[string]any{"id": "web-1", "kind": "web.search", "query": "official synthetic guide", "results": "PRIVATE SEARCH OUTPUT"}) +
		nativeTestRecord("Reasoning", map[string]any{"id": "reasoning-1", "raw_content": "PRIVATE REASONING"}) + chatRecord("agent_message", "Done")
	st := &parseState{}
	if _, err := readCodexRecords(strings.NewReader(input), 0, st); err != nil {
		t.Fatal(err)
	}
	st.flush()
	steps := nativeTestSteps(st)
	var operations []string
	for _, step := range steps {
		operations = append(operations, step.Operation)
	}
	if !reflect.DeepEqual(operations, []string{"code", "read", "edit", "read", "web"}) {
		t.Fatalf("missing native actions: %v", operations)
	}
	if steps[1].Workdir != "/example" || steps[1].Detail != "sample.go" || steps[2].InputKind != "changes" || !strings.Contains(steps[2].Input, "unified_diff") {
		t.Fatalf("missing selected action details: %#v", steps)
	}
	encoded, _ := json.Marshal(st.turns)
	if strings.Contains(string(encoded), "PRIVATE") || strings.Contains(string(encoded), "command-1") {
		t.Fatal("result, reasoning, configuration or private correlation ID leaked")
	}
}

func TestCodexNativeDeduplicatesOnlyExactIDsAndKeepsLegacyInput(t *testing.T) {
	function := `{"type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"same-call","arguments":"{\"cmd\":\"cat same.go\"}"}}` + "\n"
	values := map[string]any{"id": "same-call", "command": []string{"bash", "-lc", "cat same.go"}, "parsed_cmd": []any{map[string]string{"type": "read", "path": "same.go"}}}
	input := chatRecord("user_message", "q") + function + nativeTestRecord("CommandExecution", values) + nativeTestRecord("CommandExecution", values)
	values["id"] = "different-call"
	input += nativeTestRecord("CommandExecution", values)
	st := &parseState{}
	if _, err := readCodexRecords(strings.NewReader(input), 0, st); err != nil {
		t.Fatal(err)
	}
	st.flush()
	steps := nativeTestSteps(st)
	if len(steps) != 2 || steps[0].Input != "cat same.go" || steps[0].Operation != "read" || steps[1].Operation != "read" {
		t.Fatalf("duplicate or replaced original input: %#v", steps)
	}
}

func TestCodexNativeFileChangesKeepFullInputAndIncrementalIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	prompt := chatRecord("user_message", "q")
	if err := os.WriteFile(path, []byte(prompt), 0600); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	if _, err := l.loadCodexFile(path, 300); err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("中文🙂", 3000)
	record := nativeTestRecord("FileChange", map[string]any{"id": "large-file", "status": "failed", "changes": map[string]any{"example.md": map[string]string{"type": "add", "content": content}}})
	appendPart := func(text string) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString(text)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
	}
	appendPart(record[:len(record)-3])
	before, err := l.loadCodexFile(path, 300)
	if err != nil || len(before) != 1 || len(before[0].Segments) != 0 {
		t.Fatalf("partial native record exposed: %#v %v", before, err)
	}
	appendPart(record[len(record)-3:])
	turns, err := l.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	step := turns[0].Segments[0].Steps[0]
	if !step.DetailsTruncated || step.InputRef == "" {
		t.Fatal("missing bounded preview or full-input identity")
	}
	full, err := readToolInput(path, codexStep, int64(len(prompt)), step.InputRef)
	if err != nil || full.DetailsTruncated || !strings.Contains(full.Input, content) || !strings.Contains(full.Input, `"failed"`) {
		t.Fatalf("full changes unavailable or failure status lost: %v", err)
	}
	appendPart(record)
	again, err := l.loadCodexFile(path, 300)
	if err != nil || len(again[0].Segments[0].Steps) != 1 || again[0].Segments[0].Steps[0].InputRef != step.InputRef {
		t.Fatalf("incremental duplicate changed identity: %#v %v", again, err)
	}
}

func TestCodexNativeUnknownOutputCannotBecomeAnAction(t *testing.T) {
	input := chatRecord("user_message", "q") + nativeTestRecord("Reasoning", map[string]any{"summary_text": "FileChange"}) +
		nativeTestRecord("custom_tool_call", map[string]any{"name": "exec", "input": "duplicate lower-level mirror"}) +
		nativeTestRecord("Extension", map[string]any{"kind": "unknown.telemetry", "query": "not an action"}) +
		`{"type":"response_item","payload":{"type":"custom_tool_call_output","output":"FileChange CommandExecution"}}` + "\n" +
		chatRecord("agent_message", "done")
	st := &parseState{}
	if _, err := readCodexRecords(strings.NewReader(input), 0, st); err != nil {
		t.Fatal(err)
	}
	st.flush()
	if len(nativeTestSteps(st)) != 0 {
		t.Fatal("unknown/mirror/output data became a tool event")
	}
}

func TestCodexNativeLargeResultsDoNotSpendChatRetentionBudget(t *testing.T) {
	input := chatRecord("user_message", "earlier") + chatRecord("agent_message", "earlier reply") + chatRecord("user_message", "current") +
		nativeTestRecord("McpToolCall", map[string]any{"id": "large-result", "tool": "github_fetch_file", "arguments": map[string]string{"path": "example.go"}, "result": strings.Repeat("PRIVATE RESULT ", 1<<18)}) + chatRecord("agent_message", "current reply")
	st := &parseState{byteCap: 64 << 10}
	if _, err := readCodexRecords(strings.NewReader(input), 0, st); err != nil {
		t.Fatal(err)
	}
	st.flush()
	encoded, _ := json.Marshal(st.turns)
	if len(st.turns) != 2 || len(nativeTestSteps(st)) != 1 || len(encoded) > 4096 || strings.Contains(string(encoded), "PRIVATE RESULT") {
		t.Fatal("native result consumed the public history budget")
	}
}

func TestCodexNativeMixedCommandsStayGeneric(t *testing.T) {
	st := &parseState{}
	record := nativeTestRecord("CommandExecution", map[string]any{"id": "mixed", "command": []string{"bash", "-lc", "git status; cat example.go"}, "parsed_cmd": []any{map[string]string{"type": "unknown", "cmd": "git status"}, map[string]string{"type": "read", "path": "example.go"}}})
	if _, err := readCodexRecords(strings.NewReader(record), 0, st); err != nil {
		t.Fatal(err)
	}
	st.flush()
	steps := nativeTestSteps(st)
	if len(steps) != 1 || steps[0].Operation != "command" || steps[0].Detail != "example.go" {
		t.Fatalf("mixed command was guessed to be a pure file read: %#v", steps)
	}
}

func TestRecordedFileSummaryKeepsLeafAndDistinguishesDuplicates(t *testing.T) {
	got := recordedPathSummary([]string{"/very/long/repo/a/config.json", "/very/long/repo/b/config.json", "/repo/readme.md", "/repo/last.go"})
	if got != "a/config.json, b/config.json, readme.md +1" {
		t.Fatalf("unreadable file label: %s", got)
	}
}
