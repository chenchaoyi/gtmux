package transcript

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func chatRecord(kind, message string) string {
	return fmt.Sprintf("{\"type\":\"event_msg\",\"payload\":{\"type\":%q,\"message\":%q}}\n", kind, message)
}

func TestCodexChatRecordHeaders(t *testing.T) {
	for _, tc := range []struct {
		line string
		keep bool
	}{
		{`{"type":"compacted","payload":{"type":"message"`, false},
		{`{"type":"response_item","payload":{"type":"custom_tool_call_output","output":"`, false},
		{`{"type":"event_msg","payload":{"type":"item_completed","item":`, false},
		{`{"type":"response_item","payload":{"type":"message","role":"user"`, true},
		{`{"type":"event_msg","payload":{"message":"body","type":"agent_message"}}`, true},
		{`{"payload":{"type":"message"},"type":"response_item"}`, true},
		{`{"timestamp":"long incomplete`, true},
		{`{"type":"response_item","payload":{"content":"embedded \\\"type\\\":\\\"reasoning\\\"","type":"message"}}`, true},
	} {
		if got := codexChatRecord([]byte(tc.line)); got != tc.keep {
			t.Errorf("%s: got %v", tc.line, got)
		}
	}
}

func TestCodexHistorySurvivesLargeToolOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(f, chatRecord("user_message", "first question")+chatRecord("agent_message", "first answer"))
	_, _ = io.WriteString(f, chatRecord("user_message", "second question"))
	_, _ = io.WriteString(f, `{"type":"response_item","payload":{"type":"custom_tool_call_output","output":"`)
	chunk := strings.Repeat("x", 1<<20)
	for i := 0; i < 65; i++ {
		if _, err := io.WriteString(f, chunk); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = io.WriteString(f, "\"}}\n"+chatRecord("agent_message", "second answer"))
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	turns, err := l.loadCodexFile(path, 300)
	if err != nil || len(turns) != 2 {
		t.Fatalf("turns=%d err=%v", len(turns), err)
	}
	if turns[0].Prompt != "first question" || turns[1].Prompt != "second question" || turns[1].Response != "second answer" {
		t.Fatalf("lost conversation: %#v", turns)
	}
}

func TestCodexAppendOnlyAndPartialRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	initial := chatRecord("user_message", "q1") + chatRecord("agent_message", "a1")
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	before, err := l.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	appendLog := func(s string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, s); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	partial := chatRecord("agent_message", "a2")
	appendLog(partial[:len(partial)-2])
	turns, err := l.loadCodexFile(path, 300)
	if err != nil || !reflect.DeepEqual(turns, before) {
		t.Fatalf("partial modified turn: %#v %v", turns, err)
	}
	if l.m[path].nextLine != int64(len(initial)) {
		t.Fatal("partial line consumed")
	}
	appendLog(partial[len(partial)-2:] + `{"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{}"}}` + "\n")
	turns, err = l.loadCodexFile(path, 300)
	if err != nil || turns[0].Response != "a1\n\na2" || len(turns[0].Segments[1].Steps) != 1 {
		t.Fatalf("append: %#v %v", turns, err)
	}
	snapshot := turns
	appendLog(`{"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{}"}}` + "\n" + chatRecord("user_message", "q2") + chatRecord("agent_message", "final"))
	turns, err = l.loadCodexFile(path, 300)
	if err != nil || len(turns) != 2 || len(turns[0].Segments[1].Steps) != 2 {
		t.Fatalf("new turn: %#v %v", turns, err)
	}
	if before[0].Response != "a1" || len(before[0].Segments) != 1 || len(snapshot[0].Segments[1].Steps) != 1 {
		t.Fatal("previous response snapshot mutated")
	}
	if l.m[path].nextLine != l.m[path].size {
		t.Fatal("reader did not advance to end")
	}
}

func TestCodexReaderRetentionAndReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	var b strings.Builder
	for i := 0; i < cacheTurnCap+10; i++ {
		b.WriteString(chatRecord("user_message", fmt.Sprintf("q%d", i)))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	turns, err := l.loadCodexFile(path, 0)
	if err != nil || len(turns) != cacheTurnCap || turns[0].Prompt != "q10" {
		t.Fatalf("retention: %d %v", len(turns), err)
	}
	for _, q := range []string{"a", "b"} {
		if err := os.WriteFile(path, []byte(chatRecord("user_message", q)), 0600); err != nil {
			t.Fatal(err)
		}
		// Ensure same-size rewrite invalidation does not depend on clock granularity.
		fi, _ := os.Stat(path)
		at := fi.ModTime().Add(1)
		_ = os.Chtimes(path, at, at)
		turns, err = l.loadCodexFile(path, 300)
		if err != nil || len(turns) != 1 || turns[0].Prompt != q {
			t.Fatalf("rewrite: %#v %v", turns, err)
		}
	}
	replacement := path + ".new"
	if err := os.WriteFile(replacement, []byte(chatRecord("user_message", "c")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	turns, err = l.loadCodexFile(path, 300)
	if err != nil || len(turns) != 1 || turns[0].Prompt != "c" {
		t.Fatalf("replace: %#v %v", turns, err)
	}
}

func TestCodexLargeRelevantAndReorderedRecords(t *testing.T) {
	text := strings.Repeat("a", 100000)
	input := fmt.Sprintf("{\"payload\":{\"type\":\"user_message\",\"message\":%q},\"type\":\"event_msg\"}\n", text)
	st := &parseState{}
	next, err := readCodexRecords(strings.NewReader(input), 0, st)
	st.flush()
	if err != nil || next != int64(len(input)) || len(st.turns) != 1 || st.turns[0].Prompt != text {
		t.Fatalf("large relevant record lost: %d %v", next, err)
	}
}

type codexBrokenReader struct{}

func (codexBrokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestCodexReaderReportsReadFailure(t *testing.T) {
	input := io.MultiReader(strings.NewReader(chatRecord("user_message", "q")), codexBrokenReader{})
	_, err := readCodexRecords(input, 0, &parseState{})
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("read failure hidden: %v", err)
	}
}
