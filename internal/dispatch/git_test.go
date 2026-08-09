package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// gitEnv is a deterministic identity for the throwaway repos below.
func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
}

// TestBranchMerged_SquashMergeOnTheRemote is the failure the tree-identity check was
// supposed to catch and didn't: the squash lands on the REMOTE, and nothing pulls it
// into the local `main` — `gh pr merge` doesn't, and under a worktree layout the local
// `main` belongs to a different checkout that may be pinned for other work. The base
// therefore has to be the remote-TRACKING ref. Measured on the branch that hit this:
// against local `main` the scan range was EMPTY and the verdict was "not merged";
// against `origin/main` the branch tip's tree matched the squash commit exactly.
func TestBranchMerged_SquashMergeOnTheRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// An "upstream" repo plus a clone — the shape a real dispatch worktree has. Clone
	// BEFORE upstream moves anywhere, so origin/HEAD resolves to origin/main.
	upstream := t.TempDir()
	if out, err := exec.Command("git", "-C", upstream, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("git init -b unsupported: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(upstream, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(upstream, "add", ".")
	run(upstream, "commit", "-m", "init")

	clone := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", "-q", upstream, clone).CombinedOutput(); err != nil {
		t.Skipf("git clone unsupported here: %v\n%s", err, out)
	}
	if got := defaultBranch(clone); got != "origin/main" {
		t.Fatalf("defaultBranch(clone) = %q, want origin/main — the base must be the remote ref", got)
	}
	run(clone, "checkout", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(clone, "g"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(clone, "add", ".")
	run(clone, "commit", "-m", "feat work")

	if merged, err := BranchMerged(clone, "feat/x"); merged {
		t.Fatalf("BranchMerged before the merge = true (err=%v); want not-merged", err)
	}

	// The squash-merge happens UPSTREAM. The local `main` never learns about it — that is
	// the whole point: judging against it would say "not merged" forever.
	run(upstream, "fetch", clone, "feat/x")
	run(upstream, "merge", "--squash", "FETCH_HEAD")
	run(upstream, "commit", "-m", "squash merge feat/x (#1)")

	FetchBase(clone) // what `gtmux reap` does before judging

	local, _ := gitOutput(clone, "rev-parse", "main")
	remote, _ := gitOutput(clone, "rev-parse", "origin/main")
	if local == remote {
		t.Fatal("test setup is not exercising the bug: local main must still be behind")
	}
	if merged, err := BranchMerged(clone, "feat/x"); err != nil || !merged {
		t.Fatalf("BranchMerged after the upstream squash = %v, %v; want true, nil "+
			"(the base must be origin/main, not the stale local main)", merged, err)
	}
}

// defaultBranch must name the REMOTE-TRACKING ref when there is one, and only fall back
// to a local branch for a repo that has no remote default at all (where local history
// is the whole story — and where every fixture repo lives).
func TestDefaultBranch_PrefersTheRemoteTrackingRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	solo := t.TempDir()
	if out, err := exec.Command("git", "-C", solo, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("git init -b unsupported: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(solo, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-C", solo}, args...)...)
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if got := defaultBranch(solo); got != "main" {
		t.Errorf("no remote: defaultBranch = %q, want the local %q", got, "main")
	}
	// FetchBase must be a no-op for a local base (nothing to fetch, no hang).
	FetchBase(solo)
}

// prMergeState is three-valued because "gh could not answer" is not "not merged".
// Folding them was the second half of the failure: `gh` sits in /opt/homebrew/bin, a
// launchd-started gtmux inherits /usr/bin:/bin:/usr/sbin:/sbin, and the resulting
// "not found" silently became the whole verdict.
func TestPrMergeState_UnavailableIsNotNo(t *testing.T) {
	dir := t.TempDir()
	orig := ghLook
	ghLook = func() string { return "" } // a machine where gh is not installed
	t.Cleanup(func() { ghLook = orig })

	if got := prMergeState(dir, "any-branch"); got != mergeUnknown {
		t.Errorf("gh unavailable → %v, want mergeUnknown (never mergeNo)", got)
	}
	// And the caller must surface that as an ERROR, not as a confident "not merged".
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	// A repo whose base is a remote ref, so local evidence is NOT the whole story.
	for _, args := range [][]string{{"init", "-b", "main"}, {"remote", "add", "origin", "https://example.invalid/x.git"}} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "init"},
		{"update-ref", "refs/remotes/origin/main", "HEAD"}, {"checkout", "-b", "feat/y"}} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "g"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "wip"}} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	merged, err := BranchMerged(repo, "feat/y")
	if merged {
		t.Fatal("an unverifiable branch must not read as merged")
	}
	if err == nil {
		t.Fatal("no probe could judge — that must surface as an error, not a bare false")
	}
	if !strings.Contains(err.Error(), "cannot confirm") || !strings.Contains(err.Error(), "--keep-branch") {
		t.Errorf("the error must say it could not confirm and name the way out, got %q", err)
	}
}

// `git branch -d` runs its OWN merge check, and that check only accepts an ancestor of
// HEAD/upstream — it cannot see a SQUASH merge. So DeleteBranch's force flag is not just
// "the user said --abandon": it is how a caller whose gate already established the merge
// (BranchMerged, which accepts squash-equivalence and gh's PR state) stops git's weaker
// re-check from silently overruling it. Without this, `gtmux reap` passed its gate and
// the branch survived anyway, on every squash-merged branch — the repo's normal case.
func TestDeleteBranch_SquashMergedNeedsForce(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = gitEnv()
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
	run(repo, "checkout", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(repo, "g"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", ".")
	run(repo, "commit", "-m", "work")
	run(repo, "checkout", "main")
	run(repo, "merge", "--squash", "feat/x")
	run(repo, "commit", "-m", "squash: feat/x")

	// Unforced: git refuses, and the error must CARRY git's reason. `exit status 1`
	// alone is what let this failure hide — it says nothing a user could act on.
	err := DeleteBranch(repo, "feat/x", false)
	if err == nil {
		t.Fatal("`git branch -d` should refuse a squash-merged branch (it is an ancestor of nothing)")
	}
	if !strings.Contains(err.Error(), "not fully merged") {
		t.Errorf("the error must carry git's own reason, got %q", err)
	}

	// Forced (what a passed merge gate authorizes): actually gone.
	if err := DeleteBranch(repo, "feat/x", true); err != nil {
		t.Fatalf("forced delete: %v", err)
	}
	if exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", "refs/heads/feat/x").Run() == nil {
		t.Fatal("feat/x survived a forced delete")
	}
}

// DeleteBranch must accept an ALREADY-RESOLVED main repo, not only a linked worktree —
// reap has to resolve it before removing the worktree (afterwards there is no directory
// left to ask), so mainRepo has to be idempotent on its own output.
func TestDeleteBranch_AcceptsAnAlreadyResolvedRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("git init -b unsupported: %v\n%s", err, out)
	}
	if resolved := mainRepo(mainRepo(repo)); resolved != mainRepo(repo) {
		t.Fatalf("mainRepo is not idempotent: %q vs %q", resolved, mainRepo(repo))
	}
	// And a path that no longer exists resolves to nothing useful — which is exactly
	// why the resolution cannot be deferred until after the worktree is removed.
	gone := filepath.Join(t.TempDir(), "removed-worktree")
	if got := mainRepo(gone); got != gone {
		t.Errorf("mainRepo(%q) = %q; a vanished worktree can only fall back to itself", gone, got)
	}
}
