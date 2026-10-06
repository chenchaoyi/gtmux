package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `gtmux uninstall` used to print "✓ removed Gtmux.app" whether or not there was an app at
// ~/Applications: os.RemoveAll returns nil for a path that does not exist. A Homebrew
// install lives in /Applications, so it was left in place under a success line. Only the
// removal and its report are exercised here, in a temp dir: uninstallApp itself also
// stops the real menu-bar app and unloads its login item, which a test must not do.
func TestRemovingTheAppSaysWhetherThereWasOne(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "Gtmux.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}

	kind, err := removeAppBundle(app)
	if kind != appRemoved || err != nil {
		t.Fatalf("an app that was there: kind %d, err %v", kind, err)
	}
	if _, err := os.Lstat(app); !os.IsNotExist(err) {
		t.Fatal("the app is still there")
	}
	if out := captureStdout(t, func() { reportAppRemoval(kind, err) }); !strings.Contains(out, "removed Gtmux.app") {
		t.Errorf("removed, but it says: %q", out)
	}

	kind, err = removeAppBundle(app) // nothing there now
	if kind != appNotThere || err != nil {
		t.Fatalf("no app: kind %d, err %v", kind, err)
	}
	var rc int
	out := captureStdout(t, func() { rc = reportAppRemoval(kind, err) })
	if rc != 0 {
		t.Errorf("finding no app is not a failure: rc %d", rc)
	}
	if strings.Contains(out, "✓ removed") {
		t.Errorf("nothing was removed, but it says so: %q", out)
	}
	if !strings.Contains(out, "brew uninstall --cask") {
		t.Errorf("no way to remove a Homebrew install is given: %q", out)
	}
}

func TestAFailedRemovalIsAFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the read-only directory this relies on")
	}
	dir := t.TempDir()
	app := filepath.Join(dir, "Gtmux.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	kind, err := removeAppBundle(app)
	if kind != appRemoveFailed || err == nil {
		t.Fatalf("a removal that could not happen: kind %d, err %v", kind, err)
	}
	if rc := reportAppRemoval(kind, err); rc != 1 {
		t.Errorf("rc %d, want 1", rc)
	}
}
