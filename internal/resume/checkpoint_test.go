package resume

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func recoveryFile(t *testing.T, home, sid string) {
	t.Helper()
	dir := filepath.Join(home, "sessions/2026/01/01")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-time-"+sid+".jsonl"), []byte(`{"type":"session_meta","payload":{"id":"`+sid+`","cwd":"/project"}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCheckpointDoesNotReplaceLiveIdentityWithOlderHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	recoveryFile(t, transcript.CodexHome(), "previous")
	old := Record{Agent: "codex", SessionID: "previous", Cwd: "/project", HomeID: transcript.CodexHomeID()}
	if err := Save("hq:0.0", old); err != nil {
		t.Fatal(err)
	}
	next := old
	next.SessionID = "empty-new"
	if err := Save("hq:0.0", next); err != nil {
		t.Fatal(err)
	}
	r, _ := Load("hq:0.0")
	if r.SessionID != "empty-new" || r.RecoverySessionID != "previous" {
		t.Fatal(r)
	}
	if _, ready := Resolve(r); ready {
		t.Fatal("empty live thread was treated as recoverable")
	}
	recoveryFile(t, transcript.CodexHome(), "empty-new")
	if err := Save("hq:0.0", next); err != nil {
		t.Fatal(err)
	}
	r, _ = Load("hq:0.0")
	if r.RecoverySessionID != "empty-new" {
		t.Fatal(r)
	}
	if _, ready := Resolve(r); !ready {
		t.Fatal("new history did not become recoverable")
	}
	r.HomeID = "foreign"
	if _, ready := Resolve(r); ready {
		t.Fatal("foreign home borrowed local history")
	}
}

func TestCheckpointPromotionDoesNotChangeBindingOwner(t *testing.T) {
	before := Record{Agent: "codex", SessionID: "thread", HomeID: "home", Frontend: "frontend", UpdatedAt: 10}
	after := before
	after.RecoverySessionID, after.RecoveryCwd, after.RecoveryAt = "thread", "/project", 20
	if !sameResumeOwner(before, after) {
		t.Fatal("durability promotion invalidated an identity lease")
	}
	after.SessionID = "different"
	if sameResumeOwner(before, after) {
		t.Fatal("different thread shared a binding owner")
	}
}
