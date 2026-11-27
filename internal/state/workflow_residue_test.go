package state

import (
	"github.com/chenchaoyi/gtmux/assets"
	"os"
	"path/filepath"
	"testing"
)

func TestPurposeCleanVerifiedScaffold(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := ChangeWorkflowLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	dir := StorageServiceRecoveryDir()
	mk := func() {
		t.Helper()
		if err := MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	inspect := func() *RecoveryScaffold {
		t.Helper()
		p, err := InspectRecoveryScaffold()
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	mk()
	p := inspect()
	if p == nil {
		t.Fatal("missing empty recovery plan")
	}
	file := filepath.Join(dir, "job.plist")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if inspect() != nil || p.Prune() == nil {
		t.Fatal("parked profile must protect directory")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+".saved"); err != nil {
		t.Fatal(err)
	}
	mk()
	if err := os.Remove(dir + ".saved"); err != nil {
		t.Fatal(err)
	}
	if p.Prune() == nil {
		t.Fatal("replacement must survive stale plan")
	}
	p = inspect()
	if p == nil {
		t.Fatal("fresh inspection should allow cleanup")
	}
	if err := p.Prune(); err != nil {
		t.Fatal(err)
	}
	if inspect() != nil {
		t.Fatal("clean store should offer no cleanup")
	}
	if err := CheckStorageLayout(); err != nil {
		t.Fatal(err)
	}
}

func TestPurposeRetiresOnlyProvenIcons(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	icon := filepath.Join(CacheDir(), "notify-icons", "codex.png")
	if err := WritePrivateFile(icon, assets.AgentIcon("codex")); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFile(IconPath(), []byte("custom icon")); err != nil {
		t.Fatal(err)
	}
	plan, err := InspectRetiredIcons()
	if err != nil || plan.Count() != 1 {
		t.Fatal(plan.Count(), err)
	}
	request := filepath.Join(NotifyDir(), "queued.json")
	if err := WritePrivateFile(request, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if n, err := plan.Prune(layoutWritersStopped); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if err := os.Remove(request); err != nil {
		t.Fatal(err)
	}
	if n, err := plan.Prune(layoutWritersStopped); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if got, err := os.ReadFile(IconPath()); err != nil || string(got) != "custom icon" {
		t.Fatal("custom thumbnail lost", err)
	}
}
