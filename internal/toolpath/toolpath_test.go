package toolpath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A GUI-launched process inherits launchd's PATH (/usr/bin:/bin:/usr/sbin:/sbin), which
// has neither Homebrew prefix on it. exec.LookPath therefore reported cloudflared and
// brew as "not installed" on a Mac holding both in /usr/local/bin — so the menu bar could
// never switch to Anywhere, while the identical command from a terminal worked
// (tool-path-resolution).

func TestLookToolPrefersPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "faketool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got := Look("faketool"); got != bin {
		t.Errorf("Look = %q; want the PATH hit %q", got, bin)
	}
}

// The regression: PATH doesn't have it, but it IS installed where tools live.
func TestLookToolFindsAnInstallDirMissingFromPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin") // launchd's GUI PATH
	local := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(local, "faketool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Look("faketool"); got != bin {
		t.Errorf("Look = %q; want %q — a tool outside launchd's PATH must still be found", got, bin)
	}
}

// "Not installed" must stay a true statement, or the fallback just moves the lie.
func TestLookToolReportsAGenuinelyAbsentTool(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	if got := Look("definitely-not-a-real-tool-xyzzy"); got != "" {
		t.Errorf("Look = %q; want \"\" for a tool that really isn't installed", got)
	}
}

// A non-executable file of the right name is not the tool.
func TestLookToolIgnoresANonExecutable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	local := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "faketool"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Look("faketool"); got != "" {
		t.Errorf("Look = %q; want \"\" — a non-executable file is not the tool", got)
	}
}

func TestNVMDiscoveryUsesDefaultThenNumericVersionOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NVM_DIR", filepath.Join(home, ".nvm"))
	t.Setenv("PATH", t.TempDir())
	root := filepath.Join(home, ".nvm")
	for _, v := range []string{"v9.1.0", "v22.3.0", "v24.1.0"} {
		dir := filepath.Join(root, "versions", "node", v, "bin")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "dst"), []byte("#!/bin/sh\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "alias"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias", "default")
	if err := os.WriteFile(alias, []byte("22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := Look("dst"); !strings.Contains(got, "/v22.3.0/") {
		t.Fatal("default ignored", got)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if got := Look("dst"); !strings.Contains(got, "/v24.1.0/") {
		t.Fatal("numeric order ignored", got)
	}
	explicit := t.TempDir()
	if err := os.WriteFile(filepath.Join(explicit, "dst"), []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", explicit)
	if Look("dst") != filepath.Join(explicit, "dst") {
		t.Fatal("PATH override lost")
	}
}
