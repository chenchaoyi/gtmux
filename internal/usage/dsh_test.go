package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dsh"
)

func TestDSHDailyAndSessionAccountingSurviveRestartsAndCorrections(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	old := dsh.Sessions
	dsh.Sessions = &dsh.Catalog{}
	t.Cleanup(func() { dsh.Sessions = old })
	now := time.Now()
	at := now.Add(-time.Minute).UnixMilli()
	dir := filepath.Join(dsh.Home(), "sessions", "workspace", "one")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "session.v4.jsonl")
	data := fmt.Sprintf(`{"type":"session","version":4,"id":"one","cwd":"/project"}`+"\n"+`{"type":"user/message","time":%d,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"fixture"}]}}`+"\n"+`{"type":"assistant/message","time":%d,"data":{"turn":0,"step":0,"usage":{"inputTokens":100,"outputTokens":20,"cacheReadTokens":200}}}`+"\n", at, at)
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	l := dailyLedger{V: 1, Files: map[string]fileMark{}, Days: map[string]map[string]*Count{}}
	updateDSHDaily(&l, now)
	day := time.UnixMilli(at).Local().Format("2006-01-02")
	if got := l.Days[day]["dsh"]; got.Out != 20 || got.In != 100 {
		t.Fatal(got)
	}
	dsh.Sessions = &dsh.Catalog{}
	updateDSHDaily(&l, now)
	if l.Days[day]["dsh"].Out != 20 {
		t.Fatal("restart double-counted")
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(f, `{"type":"assistant/message","time":%d,"data":{"turn":0,"step":0,"usage":{"inputTokens":100,"outputTokens":25,"cacheReadTokens":200}}}`+"\n", at+1)
	f.Close()
	updateDSHDaily(&l, now)
	if l.Days[day]["dsh"].Out != 25 {
		t.Fatal("restatement counted as another request")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	updateDaily(&l, now, map[string][]string{})
	if _, ok := l.Files[file]; !ok || l.Days[day]["dsh"].Out != 25 {
		t.Fatal("source removal erased accounting identity/history")
	}
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	// Continue from the corrected durable projection, rather than restoring an old snapshot.
	f, _ = os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	fmt.Fprintf(f, `{"type":"assistant/message","time":%d,"data":{"turn":0,"step":0,"usage":{"inputTokens":100,"outputTokens":25,"cacheReadTokens":200}}}`+"\n", at+1)
	f.Close()
	dsh.Sessions = &dsh.Catalog{}
	updateDSHDaily(&l, now)
	if l.Days[day]["dsh"].Out != 25 {
		t.Fatal("restored source counted twice")
	}
	s, ok := ForSession("dsh", "one", now)
	if !ok || s.OutTok != 25 || s.InTok != 100 || s.CtxTok != 300 || s.Window != 0 {
		t.Fatal(s, ok)
	}
}
