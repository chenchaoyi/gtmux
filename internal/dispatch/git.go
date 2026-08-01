package dispatch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// git worktree helpers, centralized here (cgo-free — shelled out) so `gtmux spawn`,
// `gtmux reap`, and the reap-suggest sweep all share ONE implementation of "is it
// dirty / merged / how do I remove it".

// gitOutput runs git in dir and returns trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// gitRun runs git in dir, discarding output.
func gitRun(dir string, args ...string) error {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Run()
}

// SanitizeBranch makes a branch name safe as a single path element.
func SanitizeBranch(b string) string {
	return strings.NewReplacer("/", "-", ":", "-", " ", "-").Replace(strings.Trim(b, "/"))
}

// Worktree describes what AddWorktree acquired — and, for the rollback path, how much
// of it this call is responsible for.
type Worktree struct {
	Path      string
	Branch    string
	Reused    bool // an existing worktree already served this branch; we adopted it
	NewBranch bool // the branch did not exist and was created here
}

// AddWorktree acquires a git worktree for branch off the repo containing dir, creating
// the branch if it doesn't exist.
//
// It is IDEMPOTENT by design: a worktree that already serves that branch is REUSED
// rather than reported as a failure. The old behaviour turned every retry of a dispatch
// that had failed after creating its worktree into `exit status 128` — the second
// failure was caused entirely by the first one's leftovers, so re-running the identical
// command could never converge. A path occupied by something that is NOT a worktree for
// this branch stays a hard error: silently adopting an unrelated directory would be
// worse than the 128 it replaces.
func AddWorktree(dir, branch string) (Worktree, error) {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	top, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return Worktree{}, fmt.Errorf("not a git repository: %s", dir)
	}
	// Already checked out somewhere → adopt it (this is the retry path).
	if p, ok := worktreeForBranch(top, branch); ok {
		return Worktree{Path: resolvePath(p), Branch: branch, Reused: true}, nil
	}
	base := os.Getenv("GTMUX_WORKTREE_DIR")
	if base == "" {
		base = top + "-wt"
	}
	path := filepath.Join(base, SanitizeBranch(branch))
	if _, statErr := os.Stat(path); statErr == nil {
		return Worktree{}, fmt.Errorf("%s already exists but is not a worktree for %s", path, branch)
	}
	exists := gitRun(top, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch) == nil
	if exists {
		err = gitRun(top, "worktree", "add", path, branch)
	} else {
		err = gitRun(top, "worktree", "add", "-b", branch, path)
	}
	if err != nil {
		return Worktree{}, err
	}
	return Worktree{Path: resolvePath(path), Branch: branch, NewBranch: !exists}, nil
}

// resolvePath normalizes a worktree path through its symlinks. Both acquisition paths
// must agree byte-for-byte: the CREATE path builds the name from the repo root while the
// REUSE path reads it back from `git worktree list`, and on macOS those differ by the
// /var → /private/var symlink alone. Since the ledger matches a resumable attempt by this
// exact string, an unnormalized pair would make a retry miss its own worktree and start
// a second session — the very failure this is here to prevent.
func resolvePath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil && r != "" {
		return r
	}
	return filepath.Clean(p)
}

// worktreeForBranch returns the LINKED worktree currently checked out on branch, if any.
// The main working tree is deliberately excluded — `--worktree main` from a repo sitting
// on main must not "reuse" (and later hand to reap) the main checkout itself.
func worktreeForBranch(top, branch string) (string, bool) {
	out, err := gitOutput(top, "worktree", "list", "--porcelain")
	if err != nil {
		return "", false
	}
	cur := ""
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			if strings.TrimPrefix(line, "branch ") == "refs/heads/"+branch &&
				cur != "" && resolvePath(cur) != resolvePath(top) {
				return cur, true
			}
		}
	}
	return "", false
}

// WorktreeContext resolves, from a directory, the enclosing git worktree root, its
// branch, and whether it is a LINKED worktree (safe to `git worktree remove`) vs the
// main checkout. ok is false when dir is not inside a git repo. Used by `gtmux reap`
// to reclaim a manually-created window that has no ledger entry — from just its pane.
func WorktreeContext(dir string) (worktree, branch string, isLinked, ok bool) {
	top, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return "", "", false, false
	}
	branch, _ = gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD")
	gitDir, _ := gitOutput(dir, "rev-parse", "--path-format=absolute", "--git-dir")
	commonDir, _ := gitOutput(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	// A linked worktree's git-dir (…/.git/worktrees/<name>) differs from the shared
	// common dir (…/.git); the main checkout's are identical.
	isLinked = gitDir != "" && commonDir != "" && gitDir != commonDir
	return top, branch, isLinked, true
}

// WorktreeDirty reports whether a worktree has uncommitted changes.
func WorktreeDirty(wt string) (bool, error) {
	out, err := gitOutput(wt, "status", "--porcelain")
	return out != "", err
}

// BranchMerged reports whether branch is merged into the repo's default branch.
// A regular merge makes the branch tip an ANCESTOR of the base — checked first.
// A SQUASH merge (GitHub's default, and what this repo uses) does NOT: GitHub
// rewrites the branch's commits into one new commit on the base, so the branch
// tip is never an ancestor even though the work landed — the ancestor-only check
// used to misreport a squash-merged branch as "not merged" and block a safe
// reap (incident: PR #420 landed as 58c2bef, reap still refused it). Two more
// checks catch that case; either one is sufficient: (1) a commit on the base
// since branch's merge-base whose TREE is identical to the branch tip's — that's
// exactly what a clean squash-merge commit produces; (2) if `gh` is available
// and resolves a PR for this branch, its MERGED state is authoritative
// regardless of local history (catches a squash onto a base commit the branch
// didn't fork from cleanly). Errors only when the default branch itself can't
// be determined, so a caller can fail SAFE (treat unknown as not-merged).
func BranchMerged(wt, branch string) (bool, error) {
	base := defaultBranch(wt)
	if base == "" {
		return false, fmt.Errorf("cannot determine the default branch")
	}
	if gitRun(wt, "merge-base", "--is-ancestor", branch, base) == nil {
		return true, nil
	}
	if squashMerged(wt, branch, base) {
		return true, nil
	}
	if prMerged(wt, branch) {
		return true, nil
	}
	return false, nil
}

// squashMerged reports whether branch was squash-merged into base: some commit
// reachable from base (since branch's merge-base) has a tree identical to the
// branch tip's tree — the content a clean squash-merge commit produces.
func squashMerged(wt, branch, base string) bool {
	tip, err := gitOutput(wt, "rev-parse", branch+"^{tree}")
	if err != nil || tip == "" {
		return false
	}
	mergeBase, err := gitOutput(wt, "merge-base", branch, base)
	if err != nil || mergeBase == "" {
		return false
	}
	trees, err := gitOutput(wt, "log", "--format=%T", mergeBase+".."+base)
	if err != nil {
		return false
	}
	for _, tree := range strings.Split(trees, "\n") {
		if tree != "" && tree == tip {
			return true
		}
	}
	return false
}

// ghOutput runs `gh` in dir and returns trimmed stdout.
func ghOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// prMerged asks GitHub CLI whether branch's associated PR has state MERGED.
// false (not an error) whenever `gh` isn't installed, isn't authenticated, or
// finds no PR for the branch — those are all "inconclusive", not "not merged",
// and BranchMerged already has a safe false default for that case.
func prMerged(wt, branch string) bool {
	state, err := ghOutput(wt, "pr", "view", branch, "--json", "state", "-q", ".state")
	return err == nil && state == "MERGED"
}

// RemoveWorktree removes a linked worktree (from the main repo).
func RemoveWorktree(wt string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wt)
	return gitRun(mainRepo(wt), args...)
}

// DeleteBranch deletes a branch (from the main repo). force → -D.
func DeleteBranch(wt, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return gitRun(mainRepo(wt), "branch", flag, branch)
}

// MainRepo is mainRepo for callers outside the package — a rollback has to resolve the
// main repo BEFORE it removes the worktree, since afterwards there is no directory left
// to ask.
func MainRepo(wt string) string { return mainRepo(wt) }

// mainRepo returns the main working tree for a linked worktree (parent of the
// shared git dir), so worktree/branch commands run from the main repo.
func mainRepo(wt string) string {
	common, err := gitOutput(wt, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common == "" {
		return wt
	}
	return filepath.Dir(common)
}

// defaultBranch resolves the repo's default branch (origin/HEAD → main → master).
func defaultBranch(wt string) string {
	if head, err := gitOutput(wt, "rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil && head != "" {
		return strings.TrimPrefix(head, "origin/")
	}
	for _, b := range []string{"main", "master"} {
		if gitRun(wt, "rev-parse", "--verify", "--quiet", "refs/heads/"+b) == nil {
			return b
		}
	}
	return ""
}
