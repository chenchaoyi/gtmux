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

// AddWorktree adds a git worktree for branch off the repo containing dir, creating
// the branch if it doesn't exist. Returns (path, branch).
func AddWorktree(dir, branch string) (string, string, error) {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	top, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return "", "", fmt.Errorf("not a git repository: %s", dir)
	}
	base := os.Getenv("GTMUX_WORKTREE_DIR")
	if base == "" {
		base = top + "-wt"
	}
	path := filepath.Join(base, SanitizeBranch(branch))
	exists := gitRun(top, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch) == nil
	if exists {
		err = gitRun(top, "worktree", "add", path, branch)
	} else {
		err = gitRun(top, "worktree", "add", "-b", branch, path)
	}
	if err != nil {
		return "", "", err
	}
	return path, branch, nil
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

// BranchMerged reports whether branch is merged into the repo's default branch
// (an ancestor of it). Errors when the default branch can't be determined, so a
// caller can fail SAFE (treat unknown as not-merged).
func BranchMerged(wt, branch string) (bool, error) {
	base := defaultBranch(wt)
	if base == "" {
		return false, fmt.Errorf("cannot determine the default branch")
	}
	return gitRun(wt, "merge-base", "--is-ancestor", branch, base) == nil, nil
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
