package transcript

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/klauspost/compress/zstd"
)

func TestHistoryCompressedDSHPagesKeepSourcePositions(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "project", "history")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "session.v4.jsonl.zstd")
	enc, _ := zstd.NewWriter(nil, zstd.WithEncoderCRC(true))
	defer enc.Close()
	data := enc.EncodeAll([]byte(`{"type":"session","version":4,"id":"history","createdAt":1000,"cwd":"/project"}`+"\n"), nil)
	for i := 1; i <= 167; i++ {
		line := fmt.Sprintf(`{"type":"user/message","time":%d,"seq":%d,"surfaceOp":"append","data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"Question %d"}]}}`+"\n", i*1000, i, i)
		data = append(data, enc.EncodeAll([]byte(line), nil)...)
	}
	if e := os.WriteFile(file, data, 0600); e != nil {
		t.Fatal(e)
	}
	l := &Loader{}
	ts, _, e := l.loadDSH("history", 10)
	if e != nil || ts[0].Ordinal != 158 {
		t.Fatal("DSH source ordinal", e)
	}
	SetHistoryCursor("dsh", "history", ts)
	page, e := LoadBefore(context.Background(), "dsh", "history", ts[0].HistoryBefore)
	if e != nil || len(page) != 50 || page[0].Ordinal != 108 || page[49].Ordinal != 157 {
		t.Fatalf("DSH page len%d %v", len(page), e)
	}
}
