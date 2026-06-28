package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitInfo(t *testing.T) {
	// non-repo cwd → empty
	if p, b := gitInfo(t.TempDir()); p != "" || b != "" {
		t.Fatalf("non-repo: got project=%q branch=%q, want empty", p, b)
	}
	if p, b := gitInfo(""); p != "" || b != "" {
		t.Fatalf("empty cwd: got project=%q branch=%q, want empty", p, b)
	}

	// a normal repo with a checked-out branch, queried from a nested subdir.
	root := filepath.Join(t.TempDir(), "myproj")
	mustMkdir(t, filepath.Join(root, ".git"))
	mustWrite(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/feat/cool-thing\n")
	sub := filepath.Join(root, "a", "b")
	mustMkdir(t, sub)
	if p, b := gitInfo(sub); p != "myproj" || b != "feat/cool-thing" {
		t.Fatalf("nested repo: got project=%q branch=%q, want myproj/feat/cool-thing", p, b)
	}

	// detached HEAD → short SHA
	mustWrite(t, filepath.Join(root, ".git", "HEAD"), "0123456789abcdef0123456789abcdef01234567\n")
	if _, b := gitInfo(root); b != "0123456" {
		t.Fatalf("detached: got branch=%q, want 0123456", b)
	}
}

func TestGitInfoWorktree(t *testing.T) {
	// a linked worktree: .git is a FILE pointing at the real gitdir, where HEAD lives.
	base := t.TempDir()
	gitdir := filepath.Join(base, "main", ".git", "worktrees", "wt")
	mustMkdir(t, gitdir)
	mustWrite(t, filepath.Join(gitdir, "HEAD"), "ref: refs/heads/wt-branch\n")

	wt := filepath.Join(base, "wt")
	mustMkdir(t, wt)
	mustWrite(t, filepath.Join(wt, ".git"), "gitdir: "+gitdir+"\n")

	if p, b := gitInfo(wt); p != "wt" || b != "wt-branch" {
		t.Fatalf("worktree: got project=%q branch=%q, want wt/wt-branch", p, b)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}
