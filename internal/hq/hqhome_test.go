package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `gtmux hq --home` exists so a SURFACE that must run a knowledge mutation can find the
// one directory the cwd gate accepts, without re-implementing the path (menubar-kb-actions,
// the same reason `--board` exists). Two things have to hold: the path it prints is the
// path the gate compares against, and a missing home says so instead of handing back a
// directory that is about to fail a chdir.

func TestHQHomePrintsThePathTheGateAcceptsExactly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".config", "gtmux", "hq")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}

	var rc int
	out, errOut := captureBoth(t, func() { rc = CmdHQ([]string{"--home"}) })
	if rc != 0 {
		t.Fatalf("rc = %d, stderr %q; want 0 for an existing home", rc, errOut)
	}
	got := strings.TrimSpace(out)
	if got != want {
		t.Fatalf("printed %q; want %q", got, want)
	}
	// The point of printing it: chdir there and the gate lets the verb through. Anything
	// else — a symlink alias, a trailing slash — would print a path the gate refuses.
	t.Chdir(got)
	if !fromHQHome() {
		t.Errorf("cwd %q does not satisfy fromHQHome(); --home printed a path the gate refuses", got)
	}
}

func TestHQHomeSaysSoWhenThereIsNoHomeYet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var rc int
	out, errOut := captureBoth(t, func() { rc = CmdHQ([]string{"--home"}) })
	if rc == 0 {
		t.Errorf("rc = 0 for a machine with no HQ; want non-zero so a caller can stop")
	}
	if strings.TrimSpace(out) == "" {
		t.Error("printed no path; a caller wants it for the message it is about to show")
	}
	if !strings.Contains(errOut, "gtmux hq") {
		t.Errorf("stderr = %q; want a reason naming the command", errOut)
	}
}
