package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDTV004DiffNamesActualLinkedWorktreeFromNestedCwd(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	main := filepath.Join(root, "main")
	linked := filepath.Join(root, "linked tree")
	if err := os.MkdirAll(filepath.Join(main, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", main, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(main, "nested/a.go"), []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-qm", "base")
	git("worktree", "add", "-qb", "feature/diff-tree", linked)
	if err := os.WriteFile(filepath.Join(linked, "nested/a.go"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linked, "untracked.txt"), []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := diffForCwd(filepath.Join(linked, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	var actual string
	for _, line := range strings.Split(out, "\n") {
		if value, ok := strings.CutPrefix(line, "# worktree "); ok {
			if err := json.Unmarshal([]byte(value), &actual); err != nil {
				t.Fatal(err)
			}
		}
	}
	want, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	if actual != want {
		t.Fatalf("wrong worktree: got %q, want %q", actual, want)
	}
	if !strings.Contains(out, "# branch feature/diff-tree\n") || !strings.Contains(out, "+changed") || !strings.Contains(out, "+ untracked.txt") {
		t.Fatalf("context lost original changes: %q", out)
	}
}
