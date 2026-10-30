package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func customRecord(name, input string) string {
	return fmt.Sprintf("{\"type\":\"response_item\",\"payload\":{\"type\":\"custom_tool_call\",\"name\":%q,\"input\":%q,\"status\":\"completed\",\"call_id\":\"synthetic-call\"}}\n", name, input)
}

// Exercise the streaming header filter and parser together: adding a parser branch
// alone is insufficient because the reader used to discard the record first.
func TestCodexCustomCallsAndLegacyCallsShareOrderedPublicHistory(t *testing.T) {
	code := "const value = await tools.clock__curr_time({});\ntext(value);"
	input := chatRecord("user_message", "Inspect the synthetic job") + customRecord("exec", code) +
		`{"type":"response_item","payload":{"type":"custom_tool_call_output","output":"PRIVATE TOOL OUTPUT"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"reasoning","text":"PRIVATE REASONING"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"custom_tool_call","name":"exec","input":"DUPLICATE EVENT"}}}` + "\n" +
		`{"type":"response_item","payload":{"type":"function_call","name":"wait","arguments":"{\"cell_id\":\"fixture\",\"yield_time_ms\":100}"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go test ./fixture\"}"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"function_call","name":"exec","arguments":"{\"cmd\":\"printf legacy\"}"}}` + "\n" + customRecord("apply_patch", "*** Begin Patch\n*** End Patch") + customRecord("custom_unknown", "literal input") + chatRecord("agent_message", "Done")
	st := &parseState{}
	if _, err := readCodexRecords(strings.NewReader(input), 0, st); err != nil {
		t.Fatal(err)
	}
	st.flush()
	if len(st.turns) != 1 {
		t.Fatalf("turns=%d", len(st.turns))
	}
	var steps []Step
	for _, segment := range st.turns[0].Segments {
		steps = append(steps, segment.Steps...)
	}
	if len(steps) != 6 {
		t.Fatalf("steps=%#v", steps)
	}
	if steps[0].Operation != "code" || steps[0].InputKind != "code" || steps[0].Input != code {
		t.Fatalf("custom code lost: %#v", steps[0])
	}
	if steps[1].Operation != "wait" || !strings.Contains(steps[1].Input, "fixture") {
		t.Fatalf("legacy wait lost: %#v", steps[1])
	}
	if steps[2].Operation != "command" || steps[2].Input != "go test ./fixture" {
		t.Fatalf("legacy command changed: %#v", steps[2])
	}
	if steps[3].Operation != "command" || steps[3].Input != "printf legacy" {
		t.Fatalf("legacy exec reclassified: %#v", steps[3])
	}
	if steps[4].Operation != "edit" || steps[4].Input != "*** Begin Patch\n*** End Patch" {
		t.Fatalf("custom patch lost: %#v", steps[4])
	}
	if steps[5].Operation != "tool" || steps[5].InputKind != "text" {
		t.Fatalf("unknown tool guessed: %#v", steps[5])
	}
	encoded, _ := json.Marshal(st.turns)
	if strings.Contains(string(encoded), "PRIVATE") || strings.Contains(string(encoded), "DUPLICATE") {
		t.Fatal("nonpublic/output/mirror records leaked")
	}
}

func TestCodexCustomCallsSurviveIncrementalReadAndBoundUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	initial := chatRecord("user_message", "q")
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	loader := &Loader{}
	if _, err := loader.loadCodexFile(path, 300); err != nil {
		t.Fatal(err)
	}
	record := customRecord("exec", strings.Repeat("中文🙂\n", 5000))
	split := len(record) - 12
	appendPart := func(s string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString(s)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("append: %v %v", err, closeErr)
		}
	}
	appendPart(record[:split])
	turns, err := loader.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	for _, segment := range turns[0].Segments {
		if len(segment.Steps) != 0 {
			t.Fatal("partial call exposed")
		}
	}
	appendPart(record[split:])
	turns, err = loader.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	var steps []Step
	for _, segment := range turns[0].Segments {
		steps = append(steps, segment.Steps...)
	}
	if len(steps) != 1 || !steps[0].DetailsTruncated || len(steps[0].Input) > maxToolInputBytes || !utf8.ValidString(steps[0].Input) {
		t.Fatalf("bounded input: %#v", steps)
	}
	again, err := loader.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(turns)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatal("poll duplicated or changed call")
	}
}
