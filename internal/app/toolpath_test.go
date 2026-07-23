package app

import (
	"os"
	"path/filepath"
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
	if got := lookTool("faketool"); got != bin {
		t.Errorf("lookTool = %q; want the PATH hit %q", got, bin)
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
	if got := lookTool("faketool"); got != bin {
		t.Errorf("lookTool = %q; want %q — a tool outside launchd's PATH must still be found", got, bin)
	}
}

// "Not installed" must stay a true statement, or the fallback just moves the lie.
func TestLookToolReportsAGenuinelyAbsentTool(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	if got := lookTool("definitely-not-a-real-tool-xyzzy"); got != "" {
		t.Errorf("lookTool = %q; want \"\" for a tool that really isn't installed", got)
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
	if got := lookTool("faketool"); got != "" {
		t.Errorf("lookTool = %q; want \"\" — a non-executable file is not the tool", got)
	}
}
