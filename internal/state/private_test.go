package state

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// restoreUmask puts back the umask a test's PrivateUmask call replaced.
func restoreUmask(old int) { syscall.Umask(old) }

func mode(t *testing.T, p string) os.FileMode {
	t.Helper()
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}

// The tree as it was found: files 0644 under 0755 directories, plus a script HQ keeps
// executable and a symlink pointing out of the tree.
func TestNarrowTakesGroupAndOtherAwayAndLeavesTheOwner(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "hq", "knowledge", "tools")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(root, "events.jsonl")
	script := filepath.Join(sub, "check.sh")
	outside := filepath.Join(t.TempDir(), "elsewhere")
	for p, m := range map[string]os.FileMode{journal: 0o644, script: 0o755, outside: 0o644} {
		if err := os.WriteFile(p, []byte("x"), m); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, m); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{root, filepath.Join(root, "hq"), sub} {
		_ = os.Chmod(d, 0o755)
	}

	changed := Narrow(root)

	for p, want := range map[string]os.FileMode{
		journal: 0o600, script: 0o700, root: 0o700, sub: 0o700,
	} {
		if got := mode(t, p); got != want {
			t.Errorf("%s is %#o, want %#o", filepath.Base(p), got, want)
		}
	}
	if got := mode(t, outside); got != 0o644 {
		t.Errorf("a file outside the tree, reached through a symlink, was changed to %#o", got)
	}
	if len(changed) == 0 {
		t.Error("Narrow reported no changes")
	}
	if again := Narrow(root); len(again) != 0 {
		t.Errorf("a second pass changed %d things; narrowing must settle", len(again))
	}
}

// Files gtmux edits but does not own keep their mode when rewritten, and a new one gets
// the conventional mode even under the private umask.
func TestWriteForeignKeepsConventionalModes(t *testing.T) {
	old := PrivateUmask()
	defer restoreUmask(old)

	dir := t.TempDir()
	existing := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(existing, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(existing, 0o640)
	if err := WriteForeign(existing, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mode(t, existing); got != 0o640 {
		t.Errorf("rewriting someone else's file changed its mode to %#o, want it kept at 0640", got)
	}

	fresh := filepath.Join(dir, "AGENTS.md")
	if err := WriteForeign(fresh, []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mode(t, fresh); got != 0o644 {
		t.Errorf("a new foreign file is %#o under the private umask, want 0644", got)
	}
}

// What the entry point relies on: after PrivateUmask, a call site that asks for 0644
// gets 0600.
func TestThePrivateUmaskOverridesALiteralMode(t *testing.T) {
	old := PrivateUmask()
	defer restoreUmask(old)
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mode(t, p); got != 0o600 {
		t.Errorf("a file written with 0644 under the private umask is %#o, want 0600", got)
	}
	d := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := mode(t, d); got != 0o700 {
		t.Errorf("a directory made with 0755 under the private umask is %#o, want 0700", got)
	}
}
