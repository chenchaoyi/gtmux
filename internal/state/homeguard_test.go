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
	// Every exported root, because the board lives under one, the caches under another,
	// and everything the rest of the tree resolves for itself hangs off Home(). A guard
	// on only some of them is the "list somebody forgets to add to" this replaced — and
	// it was exactly that until check-design.sh started failing the build on a resolver
	// outside this package.
	for name, fn := range map[string]func() string{"HQHome": HQHome, "Dir": Dir, "Home": Home} {
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

func TestAShortTmpHomeIsDisposable(t *testing.T) {
	// A dozen tmux tests here cannot use t.TempDir(): a unix socket path caps near 104
	// bytes and macOS's os.TempDir() spends most of that on /var/folders/…, so they take
	// a short os.MkdirTemp("/tmp", "gtx") and point HOME there. That home is throwaway,
	// and the guard must say so — otherwise it fires on the tests that got isolation
	// RIGHT, and the fix it prints is the thing they already did.
	dir, err := os.MkdirTemp("/tmp", "gtxguard")
	if err != nil {
		t.Skipf("no writable /tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if !disposable(dir) {
		t.Fatalf("disposable(%q) = false — a short /tmp home is throwaway, not the operator's", dir)
	}
	// And the whole way through: a path resolver must hand it back rather than panic.
	t.Setenv("HOME", dir)
	if got := HQHome(); !strings.HasPrefix(got, dir) {
		t.Errorf("HQHome() = %q, want it under %q", got, dir)
	}
}

func TestTheOperatorsHomeStaysUndisposable(t *testing.T) {
	// The widening above must not have opened the door it was guarding. A real home is
	// still a real home, including the one this process actually has.
	if h := os.Getenv("HOME"); h != "" && disposable(h) {
		t.Errorf("HOME=%q read as disposable — the guard is open", h)
	}
	for _, p := range []string{"/", "/Users/somebody", "/var", "/private", "/tmpfoo", "/nottmp/x"} {
		if disposable(p) {
			t.Errorf("disposable(%q) = true", p)
		}
	}
}
