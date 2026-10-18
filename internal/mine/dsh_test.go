package mine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func TestDSHMiningRequiresFutureIndependentConsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "project", "learn")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "session.v4.jsonl")
	if err := os.WriteFile(file, []byte(`{"type":"session","version":4,"id":"learn","cwd":"/project"}`+"\n"+`{"type":"user/message","seq":0,"time":2000,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"Original request"}]}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	opts := Options{Roots: []Root{{Agent: "dsh", Dir: dir, Glob: "session.v4.jsonl"}}}
	ledger := t.TempDir()
	run := func() Report {
		t.Helper()
		r, e := Run(ledger, opts)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	if r := run(); r.Files != 0 {
		t.Fatal("unconsented content mined")
	}
	p, err := sessionpolicy.SaveFor("dsh", "learn", sessionpolicy.Settings{HQ: true, Knowledge: true})
	if err != nil {
		t.Fatal(err)
	}
	if r := run(); len(r.Candidates) != 0 {
		t.Fatal("old activity mined")
	}
	after := time.Unix(p.KnowledgeSince+2, 0).UnixMilli()
	rows := []rec{{"type": "assistant/message", "seq": 1, "time": after, "data": rec{"message": rec{"role": "assistant", "content": []rec{{"type": "text", "text": "Made a change."}}}}}, {"type": "user/message", "seq": 2, "time": after, "data": rec{"role": "user", "source": rec{"kind": "user"}, "content": []rec{{"type": "text", "text": "不对，你没部署，那是 dry run"}}}}}
	appendDesktopLog(t, file, rows...)
	if r := run(); len(r.Candidates) != 1 || r.Candidates[0].Agent != "dsh" {
		t.Fatalf("new authorized lead missing: %+v", r)
	}
	p.HQ = false
	if _, err = sessionpolicy.SaveFor("dsh", "learn", p); err != nil {
		t.Fatal(err)
	}
	if r := run(); r.Files != 0 {
		t.Fatal("revoked conversation still mined")
	}
}
