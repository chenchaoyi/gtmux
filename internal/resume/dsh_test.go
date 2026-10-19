package resume

import (
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDSHResumeRequiresVerifiedTerminalHomeAndExistingRoot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "work", "synthetic")
	_ = os.MkdirAll(dir, 0700)
	_ = os.WriteFile(filepath.Join(dir, "session.v4.jsonl"), []byte(`{"type":"session","version":4,"id":"synthetic","cwd":"/synthetic project"}`+"\n"), 0600)
	r := Record{Agent: "dsh", HomeID: dsh.HomeID(), SessionID: "synthetic", TerminalClient: "dst"}
	cmd, ok := Command(r)
	if !ok || !strings.Contains(cmd, "'dst' '--backend' 'dsh' '--resume' 'synthetic'") || !strings.Contains(cmd, "'/synthetic project'") {
		t.Fatalf("terminal resume %q %v", cmd, ok)
	}
	for _, bad := range []Record{{Agent: "dsh", HomeID: r.HomeID, SessionID: r.SessionID}, {Agent: "dsh", HomeID: "foreign", SessionID: r.SessionID, TerminalClient: "dst"}, {Agent: "dsh", HomeID: r.HomeID, SessionID: "missing", TerminalClient: "dst"}, {Agent: "dsh", HomeID: r.HomeID, SessionID: r.SessionID, TerminalClient: "other"}} {
		if _, ok := Command(bad); ok {
			t.Fatal("unsafe resume enabled", bad)
		}
	}
	if Resumable("dsh") {
		t.Fatal("external Web adoption was globally enabled")
	}
}
