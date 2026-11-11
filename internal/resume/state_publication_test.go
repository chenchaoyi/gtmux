package resume

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestCSR003SavePublishesAnAtomicResumeSnapshot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	const loc = "synthetic:0.0"
	first := Record{Agent: "codex", SessionID: "original", Cwd: "/" + strings.Repeat("original/", 64)}
	if err := Save(loc, first); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(fileFor(loc))
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := os.Open(fileFor(loc))
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if err := Save(loc, Record{Agent: "codex", SessionID: "short", Cwd: "/work"}); err != nil {
		t.Fatal(err)
	}
	old, err := io.ReadAll(pinned)
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != string(before) {
		t.Fatal("resume overwrite changed a pinned incumbent snapshot")
	}
	next, err := os.ReadFile(fileFor(loc))
	if err != nil {
		t.Fatal(err)
	}
	var got Record
	if err := json.Unmarshal(next, &got); err != nil || got.SessionID != "short" {
		t.Fatalf("new pathname is not complete replacement JSON: %v %+v", err, got)
	}
}

func TestCSR003CodexSaveAndHeartbeatRespectBindingLease(t *testing.T) {
	f, _ := frontendFixture(t)
	var err error
	f, err = BindCodexFrontend(f, "root-a")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(fileFor(f.Loc))
	if err != nil {
		t.Fatal(err)
	}
	lease, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := Save(f.Loc, Record{Agent: "codex", HomeID: f.HomeID, SessionID: "root-a"}); !errors.Is(err, state.ErrLeaseHeld) {
		t.Fatalf("Save ignored live binding lease: %v", err)
	}
	if _, err := RefreshCodexFrontend(f); !errors.Is(err, state.ErrLeaseHeld) {
		t.Fatalf("heartbeat ignored live binding lease: %v", err)
	}
	after, err := os.ReadFile(fileFor(f.Loc))
	if err != nil || string(after) != string(before) {
		t.Fatal("contended writer altered incumbent resume")
	}
	lease.Close()
	if err := Save(f.Loc, Record{Agent: "codex", HomeID: f.HomeID, SessionID: "root-a"}); err != nil {
		t.Fatal(err)
	}
	r, _ := Load(f.Loc)
	r.Frontend = ""
	if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshCodexFrontend(f); err != nil {
		t.Fatalf("legacy-field recovery could not reacquire binding lease: %v", err)
	}
	if r, ok := LoadLive(f.Loc); !ok || r.Frontend != f.Instance {
		t.Fatal("same-owner recovery lost binding")
	}
}
