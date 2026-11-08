package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCallDetailsAreLazyScopedAndRedacted(t *testing.T) {
	for _, result := range []any{map[string]any{"structuredContent": map[string]any{"workflow_runs": []any{map[string]string{"status": "success"}}, "authorization": "PRIVATE"}, "content": []any{map[string]string{"type": "text", "text": "recorded response"}}, "isError": false}, "", nil, strings.Repeat("large result ", 30000)} {
		row := nativeTestRecord("McpToolCall", map[string]any{"id": "query-1", "tool": "github.fetch_commit_workflow_runs", "appName": "GitHub", "status": "completed", "arguments": map[string]any{"repo_full_name": "example/project", "commit_sha": "abc123", "token": "PRIVATE", "options": map[string]string{"api_key": "PRIVATE"}}, "duration": map[string]int{"secs": 1, "nanos": 250000000}, "result": result})
		path := filepath.Join(t.TempDir(), "record.jsonl")
		if err := os.WriteFile(path, []byte(row), 0600); err != nil {
			t.Fatal(err)
		}
		st := &parseState{sourceID: recordSourceID(path)}
		codexStep(row, st)
		step := st.cur.Segments[0].Steps[0]
		if step.Call == nil || !step.Call.HasArguments || !step.Call.HasResult || step.Call.Loaded || step.Call.Arguments != "" || step.Call.Result != "" || step.Call.Ref != step.InputRef || step.Call.Provider != "GitHub" || step.Call.DurationMS != 1250 {
			t.Fatal("missing bounded metadata", step.Call)
		}
		data, _ := json.Marshal(st.cur)
		if strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "recorded response") || len(data) > 2048 {
			t.Fatal("poll retained tool bodies")
		}
		full, err := readToolInput(path, codexStep, 0, step.Call.Ref)
		if err != nil {
			t.Fatal(err)
		}
		if !full.Call.Loaded || !full.Call.Redacted || !strings.Contains(full.Call.Arguments, "example/project") || strings.Contains(full.Call.Arguments, "PRIVATE") || strings.Contains(full.Call.Result, "PRIVATE") {
			t.Fatal("details unavailable or credentials exposed", full.Call)
		}
		if _, err = readToolInput(path, codexStep, 1, step.Call.Ref); err == nil {
			t.Fatal("accepted wrong offset")
		}
		other := filepath.Join(t.TempDir(), "other.jsonl")
		os.WriteFile(other, []byte(row), 0600)
		if _, err = readToolInput(other, codexStep, 0, step.Call.Ref); err == nil {
			t.Fatal("accepted foreign source")
		}
		changed := strings.Replace(row, "abc123", "def456", 1)
		os.WriteFile(path, []byte(changed), 0600)
		if _, err = readToolInput(path, codexStep, 0, step.Call.Ref); err == nil {
			t.Fatal("accepted changed arguments")
		}
		changed = strings.Replace(row, `"status":"completed"`, `"status":"failed"`, 1) // status is metadata, result identity must still bind to original output.
		changed = strings.Replace(changed, `"result":`, `"result":{"changed":true},"previous":`, 1)
		os.WriteFile(path, []byte(changed), 0600)
		if _, err = readToolInput(path, codexStep, 0, step.Call.Ref); err == nil {
			t.Fatal("accepted changed result")
		}
	}
}

func TestNativeCallMirrorPreservesOriginalInputAndResultReference(t *testing.T) {
	command := "  cat sample.go\n  # exact whitespace"
	args, _ := json.Marshal(map[string]string{"cmd": command})
	legacy, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]string{"type": "function_call", "name": "exec_command", "call_id": "call-1", "arguments": string(args)}})
	native := nativeTestRecord("CommandExecution", map[string]any{"id": "call-1", "command": []string{"bash", "-lc", command}, "status": "completed", "aggregated_output": "package example\n"})
	path := filepath.Join(t.TempDir(), "record.jsonl")
	first := string(legacy) + "\n"
	os.WriteFile(path, []byte(first+native), 0600)
	st := &parseState{sourceID: recordSourceID(path)}
	codexStep(first, st)
	original := st.cur.Segments[0].Steps[0]
	st.off = int64(len(first))
	codexStep(native, st)
	steps := st.cur.Segments[0].Steps
	if len(steps) != 1 || steps[0].Input != command || steps[0].InputRef != original.InputRef || steps[0].Call == nil || steps[0].Call.Ref == original.InputRef {
		t.Fatal("mirror lost input or result", steps)
	}
	full, err := readToolInput(path, codexStep, st.off, steps[0].Call.Ref)
	if err != nil || !strings.Contains(full.Call.Result, "package example") {
		t.Fatal("result ref not readable", err)
	}
	originalFull, err := readToolInput(path, codexStep, 0, original.InputRef)
	if err != nil || originalFull.Input != command {
		t.Fatal("original input overwritten", err)
	}
	// An incremental parser starts with the cached turn and must keep the same one-call identity.
	next := &parseState{sourceID: recordSourceID(path), off: st.off, cur: st.cur}
	codexStep(native, next)
	if len(next.cur.Segments[0].Steps) != 1 {
		t.Fatal("incremental duplicate")
	}
}

func TestNativeCallMissingResultIsNotAnEmptyResult(t *testing.T) {
	st := &parseState{sourceID: "source"}
	codexStep(nativeTestRecord("McpToolCall", map[string]any{"tool": "service.query", "arguments": map[string]string{"query": "sample"}}), st)
	if c := st.cur.Segments[0].Steps[0].Call; c == nil || c.HasResult {
		t.Fatal("missing result presented as an empty response")
	}
}
