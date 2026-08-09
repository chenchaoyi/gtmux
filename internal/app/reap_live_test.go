package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// This file exists because of what the OTHER reap tests cannot see.
//
// Every test in reap_test.go drives planAndReap through INJECTED ops, and the injected
// deleteBranch is `func(...) error { return nil }` — unfailable by construction. So
// `TestReap_CleanMerged_Reaps` asserts the branch step was CALLED, and there is nothing
// downstream of the call for a test to be wrong about. Meanwhile git_test.go tests
// BranchMerged — the DECISION — against a real repo.
//
// The bug lived in the gap between them: the decision was right (the gate passed and the
// worktree was reclaimed) and the execution failed anyway, twice over — reap asked git
// for the repo from inside the worktree it had just deleted, and then asked for `-d`,
// whose own merge check cannot see a squash merge. Both faults are BELOW the injection
// seam and ABOVE the function under test in git_test.go. A whole-suite green build said
// nothing about either, through a fix (#746) aimed at this very command.
//
// So: the real ops, a real repo, a real squash merge — assert the BRANCH IS GONE, not
// that a stub was invoked.

func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_NOSYSTEM=1")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// squashMergedRepo builds the exact shape this repo ships every day: a clone with an
// `origin`, a feature branch pushed to it, and that branch SQUASH-merged into main and
// pushed — so the branch tip is not an ancestor of any base, and the branch is checked
// out in a linked worktree. Returns the main checkout and the worktree path.
func squashMergedRepo(t *testing.T, branch string) (repo, worktree string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", remote).CombinedOutput(); err != nil {
		t.Skipf("git init -b unsupported: %v\n%s", err, out)
	}
	repo = filepath.Join(root, "repo")
	if out, err := exec.Command("git", "clone", "-q", remote, repo).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	write(t, repo, "base.txt", "base")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "base")
	git(t, repo, "push", "-q", "origin", "main")

	git(t, repo, "checkout", "-qb", branch)
	write(t, repo, "feature.txt", "work")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature work")
	git(t, repo, "push", "-q", "origin", branch)

	// The squash merge. `git merge --squash` + commit reproduces byte-for-byte what
	// GitHub's "Squash and merge" leaves behind: one new commit on main whose TREE
	// equals the branch tip's, and a branch tip that is an ancestor of nothing.
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "merge", "-q", "--squash", branch)
	git(t, repo, "commit", "-qm", "squash: "+branch+" (#1)")
	git(t, repo, "push", "-q", "origin", "main")
	git(t, repo, "remote", "set-head", "origin", "main")

	// Pin the premise: after a squash merge the branch tip is an ancestor of nothing.
	// This is what makes `git branch -d` refuse it, and what made the ancestor-only
	// probe call it unmerged.
	ancestor := exec.Command("git", "-C", repo, "merge-base", "--is-ancestor", branch, "main")
	ancestor.Env = gitEnv()
	if ancestor.Run() == nil {
		t.Fatal("the fixture is wrong: a squash-merged branch must NOT be an ancestor of main")
	}

	worktree = filepath.Join(root, "wt", "feat-x")
	git(t, repo, "worktree", "add", "-q", worktree, branch)
	return repo, worktree
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Env = gitEnv()
	return cmd.Run() == nil
}

// The regression, end to end: a squash-merged branch in a linked worktree, reaped
// through the REAL git ops. The observed failure was `✓ reaped:` listing the session and
// the worktree, no branch line, exit 0 — and the branch still sitting in the repo.
func TestReapLive_SquashMergedBranchIsActuallyDeleted(t *testing.T) {
	repo, wt := squashMergedRepo(t, "feat/x")
	task := dispatch.Task{ID: "live1", Worktree: wt, Branch: "feat/x"}

	res := planAndReap(task, false, false, liveReapOps())

	if !res.Reaped {
		t.Fatalf("a clean, squash-merged dispatch must reap; blocked by %v", res.BlockedBy)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("no step should fail, got %v", res.Failed)
	}
	if !hasAction(res, "deleted branch feat/x") {
		t.Errorf("actions = %v, want the branch delete reported", res.Actions)
	}
	if branchExists(t, repo, "feat/x") {
		t.Fatal("feat/x survived the reap — the branch step ran and did nothing")
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("the worktree should be gone too")
	}
}

// --keep-branch keeps it, through the real ops as well: the ONE outcome where a
// surviving branch is correct must stay distinguishable from the bug.
func TestReapLive_KeepBranchLeavesItAlone(t *testing.T) {
	repo, wt := squashMergedRepo(t, "feat/x")
	task := dispatch.Task{ID: "live2", Worktree: wt, Branch: "feat/x"}

	res := planAndReap(task, false, true, liveReapOps())

	if !res.Reaped || len(res.Failed) != 0 {
		t.Fatalf("--keep-branch reap should succeed cleanly: %+v", res)
	}
	if !branchExists(t, repo, "feat/x") {
		t.Fatal("--keep-branch deleted the branch")
	}
}

// An UNMERGED branch must still be refused — the fix forces the delete, and forcing
// past the gate instead of behind it would turn reap into a data-loss tool. The gate
// must both stop it AND say what to do (#746's contract, now asserted at the reap level
// rather than only on BranchMerged).
func TestReapLive_UnmergedBranchIsRefusedWithAWayOut(t *testing.T) {
	repo, wt := squashMergedRepo(t, "feat/x")
	// A second branch that never landed anywhere.
	git(t, repo, "branch", "feat/never", "main")
	other := filepath.Join(filepath.Dir(wt), "feat-never")
	git(t, repo, "worktree", "add", "-q", other, "feat/never")
	write(t, other, "unlanded.txt", "unlanded")
	git(t, other, "add", "-A")
	git(t, other, "commit", "-qm", "unlanded work")

	task := dispatch.Task{ID: "live3", Worktree: other, Branch: "feat/never"}
	res := planAndReap(task, false, false, liveReapOps())

	if res.Reaped {
		t.Fatal("an unmerged branch must not be reaped")
	}
	if !branchExists(t, repo, "feat/never") {
		t.Fatal("a blocked reap must touch nothing")
	}
	reason := strings.Join(res.BlockedBy, " | ")
	if reason == "" {
		t.Fatal("a refusal must say why — silence is the failure mode this fix is about")
	}
	// Either verdict is legitimate here (`gh` may or may not answer for a local
	// remote), but BOTH must name a way forward rather than just stopping.
	if !strings.Contains(reason, "not merged") &&
		!(strings.Contains(reason, "cannot confirm") && strings.Contains(reason, "--keep-branch")) {
		t.Errorf("blocked_by = %q, want either a plain 'not merged' or a 'cannot confirm … --keep-branch/--abandon'", reason)
	}
}

func hasAction(res reapResult, want string) bool {
	for _, a := range res.Actions {
		if strings.Contains(a, want) {
			return true
		}
	}
	return false
}
