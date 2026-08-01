package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// WorktreeContext must distinguish a LINKED worktree (safe to `git worktree remove`)
// from the main checkout — the safety hinge of reap-by-bare-pane.
func TestWorktreeContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("git init -b unsupported: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", ".")
	run(repo, "commit", "-m", "init")

	// The main checkout is NOT a linked worktree.
	if top, _, isLinked, ok := WorktreeContext(repo); !ok || isLinked || top == "" {
		t.Fatalf("main checkout: top=%q isLinked=%v ok=%v (want ok, not-linked)", top, isLinked, ok)
	}

	// A linked worktree IS.
	wt := filepath.Join(t.TempDir(), "wt")
	run(repo, "worktree", "add", "-b", "feat/x", wt)
	top, branch, isLinked, ok := WorktreeContext(wt)
	if !ok || !isLinked {
		t.Fatalf("linked worktree: isLinked=%v ok=%v (want both true)", isLinked, ok)
	}
	if branch != "feat/x" {
		t.Errorf("branch = %q, want feat/x", branch)
	}
	if top == "" {
		t.Errorf("worktree top should resolve")
	}

	// A non-repo dir is not ok.
	if _, _, _, ok := WorktreeContext(t.TempDir()); ok {
		t.Errorf("a non-git dir must report ok=false")
	}
}

// AddWorktree must be IDEMPOTENT. The 2026-08-01 incident: a dispatch failed after
// creating its worktree, and every retry of the identical command then died on
// `exit status 128` (git: path already exists) — the second failure caused entirely by
// the first one's leftovers, so re-running could never converge.
func TestAddWorktree_Idempotent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("git init -b unsupported: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", ".")
	run(repo, "commit", "-m", "init")
	t.Setenv("GTMUX_WORKTREE_DIR", filepath.Join(t.TempDir(), "wt"))

	first, err := AddWorktree(repo, "feat/x")
	if err != nil {
		t.Fatalf("first AddWorktree: %v", err)
	}
	if first.Reused || !first.NewBranch {
		t.Fatalf("first acquisition = %+v, want a fresh worktree on a new branch", first)
	}

	// The retry: same command, same branch. It must ADOPT, not fail.
	second, err := AddWorktree(repo, "feat/x")
	if err != nil {
		t.Fatalf("re-running the identical dispatch must reuse the worktree, got: %v", err)
	}
	if !second.Reused {
		t.Errorf("second acquisition = %+v, want Reused", second)
	}
	if second.Path != first.Path || second.Branch != first.Branch {
		t.Errorf("reuse pointed elsewhere: %+v vs %+v", second, first)
	}
	if second.NewBranch {
		t.Error("a reused worktree must not claim it created the branch (rollback would delete it)")
	}

	// The main checkout is never offered as a reusable worktree — reap must not be
	// handed the main repo.
	if p, ok := worktreeForBranch(repo, "main"); ok {
		t.Errorf("worktreeForBranch(main) = %q, want no match (that is the main checkout)", p)
	}

	// A path occupied by something that is NOT this branch's worktree stays a hard
	// error: adopting an unrelated directory is worse than the failure it replaces.
	occupied := filepath.Join(t.TempDir(), "wt2")
	t.Setenv("GTMUX_WORKTREE_DIR", occupied)
	if err := os.MkdirAll(filepath.Join(occupied, "feat-y"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AddWorktree(repo, "feat/y"); err == nil {
		t.Error("an occupied non-worktree path must error, not be silently adopted")
	}
}

// BranchMerged must catch a SQUASH merge, not just a fast-forward/regular merge:
// GitHub's squash rewrites the branch's commits into one new commit on main, so
// the branch tip is never an ancestor of main even though the work landed
// (incident: PR #420 squash-merged as 58c2bef, reap still refused it as "not
// merged"). The squashMerged fallback (tree-identity with a commit on main)
// must recognize this case.
func TestBranchMerged_SquashMerge(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("git init -b unsupported: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", ".")
	run(repo, "commit", "-m", "init")

	// A feature branch with its own commit, never regular-merged.
	run(repo, "checkout", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(repo, "g"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", ".")
	run(repo, "commit", "-m", "feat work")
	run(repo, "checkout", "main")

	// Not merged yet — the fast path and the fallback must both say so.
	if merged, err := BranchMerged(repo, "feat/x"); err != nil || merged {
		t.Fatalf("BranchMerged before squash-merge = %v, %v; want false, nil", merged, err)
	}

	// Simulate GitHub's squash merge: apply the branch's diff as ONE new commit
	// on main (branch tip is NOT an ancestor of this commit).
	run(repo, "merge", "--squash", "feat/x")
	run(repo, "commit", "-m", "squash merge feat/x (#1)")

	if merged, err := BranchMerged(repo, "feat/x"); err != nil || !merged {
		t.Fatalf("BranchMerged after squash-merge = %v, %v; want true, nil (squash-merge must be detected)", merged, err)
	}
}
