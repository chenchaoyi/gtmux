package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/klauspost/compress/zstd"
)

func TestDSHCurrentPublicTranscriptIsIncrementalAndPrivateFieldsStayPrivate(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "project", "visible")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "session.v4.jsonl.zstd")
	rows := []string{
		`{"type":"session","version":4,"id":"visible","createdAt":1000,"cwd":"/project"}`,
		`{"type":"user/message","time":1500,"seq":1,"data":{"role":"user","source":{"kind":"injection"},"content":[{"type":"text","text":"PRIVATE INSTRUCTIONS"}]},"surfaceOp":"append"}`,
		`{"type":"user/message","time":2000,"seq":2,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"Please inspect the tests"}]},"surfaceOp":"append"}`,
		`{"type":"assistant/message","seq":3,"time":2100,"data":{"message":{"role":"assistant","content":[{"type":"reasoning","text":"PRIVATE REASONING"},{"type":"text","text":"Inspecting now"}],"source":{"kind":"model","replayState":{"token":"PRIVATE REPLAY"}}},"stream":[{"private":"PRIVATE STREAM"}]},"surfaceOp":"append"}`,
		`{"type":"tool/call","seq":4,"time":2200,"data":{"name":"bash","arguments":"{\"command\":\"go test ./...\"}"}}`,
		`{"type":"assistant/message","seq":5,"time":2300,"data":{"message":{"role":"assistant","content":[{"type":"text","text":"Tests pass"}]}} ,"surfaceOp":"append"}`,
	}
	rows = append(rows, `{"type":"user/message","seq":6,"time":2350,"surfaceOp":{"op":"replace","startSeq":2,"endSeq":2},"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"OLD USER COPY"}]}}`)
	enc, _ := zstd.NewWriter(nil, zstd.WithEncoderCRC(true))
	defer enc.Close()
	var data []byte
	for _, row := range rows {
		data = append(data, enc.EncodeAll([]byte(row+"\n"), nil)...)
	}
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	l := &Loader{m: map[string]*cacheEntry{}}
	first, err := l.load("dsh", "visible", 300)
	if err != nil || len(first) != 1 {
		t.Fatalf("turns=%+v %v", first, err)
	}
	if first[0].Prompt != "Please inspect the tests" || !strings.Contains(first[0].Response, "Inspecting now") || !strings.Contains(first[0].Response, "Tests pass") || len(first[0].Segments) != 3 || first[0].Segments[1].Steps[0].Title != "bash" {
		t.Fatalf("public transcript=%+v", first)
	}
	if ctx := contexts(first[0]); len(ctx) != 1 || ctx[0].Text != "PRIVATE INSTRUCTIONS" || !ctx[0].BeforePrompt {
		t.Fatalf("scoped injection context missing: %#v", ctx)
	}
	if first[0].PromptMessageSeq == nil || *first[0].PromptMessageSeq != 2 {
		t.Fatal("snapshot lost the native prompt sequence")
	}
	*first[0].PromptMessageSeq = 999
	for _, turn := range first {
		if strings.Contains(turn.Prompt+turn.Response, "PRIVATE") {
			t.Fatal("private data became public")
		}
	}
	priorOffset := l.dsh[file].cursor.Offset
	same, err := l.load("dsh", "visible", 300)
	if err != nil || l.dsh[file].cursor.Offset != priorOffset || len(same) != 1 {
		t.Fatal("unchanged lookup failed")
	}
	if same[0].PromptMessageSeq == nil || *same[0].PromptMessageSeq != 2 {
		t.Fatal("a caller mutated cached prompt evidence")
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	next := `{"type":"assistant/message","seq":7,"time":2400,"data":{"message":{"role":"assistant","content":[{"type":"text","text":"Final update"}]}},"surfaceOp":"append"}`
	_, _ = f.Write(enc.EncodeAll([]byte(next+"\n"), nil))
	f.Close()
	updated, err := l.load("dsh", "visible", 300)
	if err != nil || !strings.Contains(updated[0].Response, "Final update") {
		t.Fatalf("append=%+v %v", updated, err)
	}
	if strings.Contains(first[0].Response, "Final update") {
		t.Fatal("returned snapshot mutated")
	}
	if FirstMessageTime("dsh", "visible") != 2 || LastMessageTime("dsh", "visible") != 2 {
		t.Fatal("message timestamps unavailable")
	}
}
