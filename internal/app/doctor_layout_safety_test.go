package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSS005CLIStorageAdmission(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	one, ok := enterCommandLayout("events", nil)
	if !ok {
		t.Fatal("normal command could not enter")
	}
	defer one.Close()
	two, ok := enterCommandLayout("codex", nil)
	if !ok {
		t.Fatal("a second command should share admission")
	}
	defer two.Close()
	noWriters := func() error { return nil }
	one.Close()
	if err := state.ChangeStorageLayout(true, noWriters); !errors.Is(err, state.ErrLeaseHeld) {
		t.Fatalf("move during admitted command: %v", err)
	}
	two.Close()
	if err := state.ChangeStorageLayout(true, noWriters); err != nil {
		t.Fatalf("move after command exit: %v", err)
	}
	exclusive, err := state.TryLease("storage-layout")
	if err != nil {
		t.Fatal(err)
	}
	defer exclusive.Close()
	for _, args := range [][]string{{"events"}, {"codex"}, {"serve"}} {
		if code := Run(args); code != 1 {
			t.Fatalf("command %v entered under exclusive lease: %d", args, code)
		}
	}
	if code := Run([]string{"doctor", "--storage-layout"}); code != 0 {
		t.Fatalf("inspection exit under exclusive lease: %d", code)
	}
}

func TestGSS007DoctorRejectsPendingLayoutConflict(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	write := func(path string) {
		t.Helper()
		if err := state.WritePrivateFile(path, []byte(path)); err != nil {
			t.Fatal(err)
		}
	}
	root := state.Dir()
	flat := filepath.Join(root, "events.jsonl")
	grouped := filepath.Join(root, "layout-v2", "events", "events.jsonl")
	write(flat)
	write(filepath.Join(root, "relay", "fixture"))
	calls := 0
	busy := errors.New("writer appeared")
	if err := state.ChangeStorageLayout(true, func() error {
		calls++
		if calls == 2 {
			return busy
		}
		return nil
	}); !errors.Is(err, busy) {
		t.Fatalf("prepare transition: %v", err)
	}
	if err := state.MkdirAll(filepath.Dir(grouped), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(flat, grouped); err != nil {
		t.Fatal(err)
	}
	preview := captureStdout(t, func() {
		if code := Run([]string{"--lang=en", "doctor", "--storage-layout"}); code != 0 {
			t.Fatalf("partial inspection exit: %d", code)
		}
	})
	if !strings.Contains(preview, "events.jsonl → layout-v2/events/events.jsonl · moved") ||
		!strings.Contains(preview, "relay → layout-v2/inbox/relay · pending") {
		t.Fatalf("wrong pending progress: %q", preview)
	}
	write(flat)
	preview = captureStdout(t, func() {
		if code := Run([]string{"--lang=en", "doctor", "--storage-layout"}); code != 1 {
			t.Fatalf("conflicting inspection exit: %d", code)
		}
	})
	if strings.Contains(preview, "Continue with") {
		t.Fatalf("conflicting plan suggests continuation: %q", preview)
	}
	if err := state.ChangeStorageLayout(true, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "event") {
		t.Fatalf("move accepted duplicate events log: %v", err)
	}
	for _, path := range []string{flat, grouped} {
		if data, err := os.ReadFile(path); err != nil || string(data) != flat {
			t.Fatalf("conflict changed %s: %q %v", path, data, err)
		}
	}
}
