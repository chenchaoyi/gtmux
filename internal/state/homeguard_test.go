package state

// The real home must be UNREACHABLE from a test, not merely discouraged.
//
// Five times between 2026-09-08 and 09-09 a test overwrote the operator's live situation
// board. Every one of those tests had redirected `XDG_CONFIG_HOME` first — a variable
// nothing in gtmux reads. They were not careless: the resolver ignored the override that
// looks like the right one, so each round of "make the test write to a temp dir" fixed a
// symptom while the next failure was already on its way.
//
// These pin the guard itself. The one that matters is the first: remove the panic from
// `home()` and it fails, which is the whole point of a guard nobody can see working.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvingTheRealHomeUnderTestIsFatal(t *testing.T) {
	// This test IS running under `go test`, so the guard is live. Point HOME at a real
	// place and any gtmux path must refuse rather than hand it over.
	t.Setenv("HOME", "/Users/somebody")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("resolving the REAL home under test did not panic — the guard is gone, " +
				"and the next test that writes a board will write the operator's")
		}
		msg, _ := r.(string)
		// The message has to say what to do. Five rounds of this went wrong because the
		// failure was silent; a panic that does not name the fix is only louder.
		if !strings.Contains(msg, `t.Setenv("HOME"`) {
			t.Errorf("the panic does not say how to fix it: %q", msg)
		}
		if !strings.Contains(msg, "XDG_CONFIG_HOME") {
			t.Errorf("the panic does not warn that XDG is not read — the exact trap: %q", msg)
		}
	}()
	_ = HQHome()
}

func TestEveryPathRootGoesThroughTheGuard(t *testing.T) {
	// Both roots, because the board lives under one and the caches under the other. A
	// guard on only one of them is the "list somebody forgets to add to" this replaced.
	for name, fn := range map[string]func() string{"HQHome": HQHome, "Dir": Dir} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s() returned a real path under test", name)
				}
			}()
			t.Setenv("HOME", "/Users/somebody")
			_ = fn()
		}()
	}
}

func TestARedirectedHomeIsFine(t *testing.T) {
	// The guard must not make tests impossible — only wrong ones.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if got := HQHome(); !strings.HasPrefix(got, tmp) {
		t.Errorf("HQHome() = %q, want it under %q", got, tmp)
	}
	if got := Dir(); !strings.HasPrefix(got, tmp) {
		t.Errorf("Dir() = %q, want it under %q", got, tmp)
	}
}

func TestOnlyTheTempTreeCounts(t *testing.T) {
	// "Disposable" is the temp tree this process would hand a test — not any path that
	// happens to contain the word tmp, and not the real home however it is spelled.
	if disposable("") {
		t.Error("an empty HOME is not disposable")
	}
	if disposable(filepath.Join(string(filepath.Separator), "Users", "somebody")) {
		t.Error("a real home is not disposable")
	}
	if !disposable(filepath.Join(os.TempDir(), "x")) {
		t.Error("a path under the temp dir is disposable")
	}
	// A sibling that merely shares a prefix must not pass.
	if disposable(filepath.Clean(os.TempDir()) + "-not-really") {
		t.Error("a path that only shares the prefix is not disposable")
	}
}
