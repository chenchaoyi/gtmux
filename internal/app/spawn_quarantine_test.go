package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// hq-home-quarantine: spawn must refuse to put a worker in the HQ home — the home's
// AGENTS.md is the supervisor charter, and a worker launched there impersonates HQ
// (the recursion incident: HQ spawned into its own cwd, the worker read the charter
// and spawned again).

func quarantineHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := state.HQHome()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSpawnDir_ExplicitCwdInHQHomeRefused(t *testing.T) {
	dir := quarantineHome(t)
	if got, bad := spawnDirInHQHome("", dir); !bad || got == "" {
		t.Fatalf("an explicit --cwd naming the HQ home must be refused; got=%q bad=%v", got, bad)
	}
}

func TestSpawnDir_InheritedCwdInHQHomeRefused(t *testing.T) {
	dir := quarantineHome(t)
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	// No --cwd: the session would inherit the caller's cwd — the exact HQ trap.
	if _, bad := spawnDirInHQHome("", ""); !bad {
		t.Fatal("an empty --cwd while sitting in the HQ home must be refused")
	}
}

func TestSpawnDir_NormalProjectAllowed(t *testing.T) {
	quarantineHome(t)
	proj := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, bad := spawnDirInHQHome("", proj); bad {
		t.Fatal("a normal project dir must pass")
	}
}
