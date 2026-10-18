package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func TestDSHReaderUsesHomeAndLeavesSourceAndConsentUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "project", "web")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "session.v4.jsonl")
	raw := `{"type":"session","version":4,"id":"web","cwd":"/project"}` + "\n" + `{"type":"user/message","time":2000,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"Inspect tests"}]}}` + "\n" + `{"type":"assistant/message","time":3000,"data":{"message":{"role":"assistant","content":[{"type":"text","text":"Public reply"},{"type":"reasoning","text":"PRIVATE"}]}}}` + "\n"
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(file)
	b, meta, err := desktopTranscriptRead("web", "", dsh.HomeID())
	if err != nil || !strings.Contains(string(b), "Public reply") || strings.Contains(string(b), "PRIVATE") {
		t.Fatalf("public history: %s %v", b, err)
	}
	if b, _, err = desktopTranscriptRead("web", meta.Etag, dsh.HomeID()); err != nil || b != nil {
		t.Fatal("unchanged revision ignored", err)
	}
	if _, _, err = desktopTranscriptRead("web", "", "foreign"); err == nil {
		t.Fatal("foreign home read")
	}
	after, _ := os.Stat(file)
	bytes, _ := os.ReadFile(file)
	if string(bytes) != raw || !before.ModTime().Equal(after.ModTime()) || before.Mode() != after.Mode() {
		t.Fatal("reader changed DSH artifact")
	}
	if got := sessionpolicy.GetFor("dsh", "web"); got.HQ || got.Notify || got.Knowledge || got.Revision != 0 {
		t.Fatal("manual reading granted follow", got)
	}
}
