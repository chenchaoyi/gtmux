package dispatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/toolpath"
)

// git worktree helpers, centralized here (cgo-free — shelled out) so `gtmux spawn`,
// `gtmux reap`, and the reap-suggest sweep all share ONE implementation of "is it
// dirty / merged / how do I remove it".

// gitOutput runs git in dir and returns trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// gitRun runs git in dir, discarding output. For PROBES, where a non-zero exit is an
// expected answer rather than a problem to report.
func gitRun(dir string, args ...string) error {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Run()
}

// gitRunLoud runs git in dir and folds its stderr INTO the error. For the destructive
// ops, whose failure a caller has to be able to explain: `exit status 1` alone told a
// user nothing about why their branch survived a reap, and `fatal: cannot change to
// '<path>'` says it exactly.
func gitRunLoud(dir string, args ...string) error {
	var stderr strings.Builder
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s", firstLine(msg))
		}
		return err
	}
	return nil
}

// firstLine keeps a git failure to its one meaningful line — the rest is `hint:` prose
// that would bury the reason in a terminal report.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
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
// didn't fork from cleanly).
//
// The RESULT IS THREE-WAY, not two. It returns an error when no probe could
// establish anything — which is a different fact from "not merged" and must not be
// reported as one. Both were once folded into `false`, and the two ways that goes
// wrong compounded: the git probes read a stale local base (see defaultBranch), and
// `gh` — by then the only probe left — was invisible under launchd's PATH, so its
// silent failure WAS the verdict. `gtmux reap` duly told the user a squash-merged
// branch had unmerged commits and refused to delete it. Callers still fail SAFE
// (err → don't reclaim); they can now say WHY, and the user gets something to fix.
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
	switch prMergeState(wt, branch) {
	case mergeYes:
		return true, nil
	case mergeNo:
		return false, nil // gh gave a real answer: the PR is open/closed, not merged
	}
	// No remote base to have merged it behind our back: local history is the WHOLE story,
	// so "not merged" here is a real answer rather than an absence of one. (defaultBranch
	// falls back to a local name only when the repo has no remote default branch at all.)
	if !strings.Contains(base, "/") {
		return false, nil
	}
	// NOTHING could judge. That is NOT the same fact as "not merged", and reporting it as
	// one is how a squash-merged branch got told it had unmerged commits: the git probes
	// were reading a stale base and `gh` — the only remaining probe — wasn't on the PATH
	// a launchd-started process inherits, so its failure silently became evidence.
	// Returning an error keeps the gate CLOSED (callers already treat err as "don't
	// reclaim") while saying the true thing.
	hint := "install/authenticate `gh` so the PR state can be read"
	if ghLook() != "" {
		hint = "`gh` is installed but could not answer (signed out, offline, or no PR for this branch)"
	}
	return false, fmt.Errorf("cannot confirm branch %q was merged into %s: no merge commit and no squash-equivalent tree there, and the PR state is unreadable — %s, or reap with --keep-branch / --abandon",
		branch, base, hint)
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

// ghOutput runs `gh` in dir and returns trimmed stdout. The binary is resolved through
// toolpath, NOT bare exec: a gtmux started by launchd (the serve LaunchAgent, the
// menu-bar app shelling out) inherits `/usr/bin:/bin:/usr/sbin:/sbin`, and Homebrew's
// `gh` is on neither prefix.
//
// It is a var so a test can simulate a machine without `gh` — the real toolpath search
// looks in fixed install dirs, so no amount of $PATH juggling can hide a gh that is
// actually installed on the developer's Mac.
var ghLook = func() string { return toolpath.Look("gh") }

func ghOutput(dir string, args ...string) (string, error) {
	bin := ghLook()
	if bin == "" {
		return "", exec.ErrNotFound
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// mergeAnswer is what a probe could establish — three values, because "I could not
// find out" is a different fact from "no", and collapsing them is the bug this triple
// exists to prevent.
type mergeAnswer int

const (
	mergeUnknown mergeAnswer = iota // no evidence either way — never treat as "no"
	mergeYes
	mergeNo
)

// prMergeState asks GitHub CLI whether branch's associated PR is MERGED.
//
// It answers mergeUnknown whenever `gh` could not produce a state — missing, signed
// out, offline, or no PR for that branch. The old version folded every one of those
// into `false`, so on a machine where `gh` merely wasn't on the PATH the answer to "was
// this merged?" became a confident NO. It is the last probe, so its silent failure was
// the whole verdict.
func prMergeState(wt, branch string) mergeAnswer {
	state, err := ghOutput(wt, "pr", "view", branch, "--json", "state", "-q", ".state")
	if err != nil {
		return mergeUnknown
	}
	switch state {
	case "MERGED":
		return mergeYes
	case "OPEN", "CLOSED":
		return mergeNo // a real answer: there IS a PR and it did not merge
	}
	return mergeUnknown
}

// RemoveWorktree removes a linked worktree (from the main repo).
func RemoveWorktree(wt string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wt)
	return gitRunLoud(mainRepo(wt), args...)
}

// DeleteBranch deletes a branch. force → -D.
//
// `dir` may be the linked worktree OR the main repo: mainRepo is idempotent on a main
// checkout, so a caller that already resolved it (which it MUST, if it removed the
// worktree first — see MainRepo) can pass that through unchanged.
//
// force is NOT merely "the user said --abandon". `git branch -d` runs its OWN merge
// check, and that check only accepts an ancestor of HEAD/upstream — it cannot see a
// SQUASH merge, which is what this repo (and GitHub by default) produces. So a caller
// whose own gate has already established the merge — reap's, which additionally accepts
// squash-equivalence and gh's authoritative PR state — must force, or git's weaker
// re-check silently overrules the stronger one that already passed.
func DeleteBranch(dir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return gitRunLoud(mainRepo(dir), "branch", flag, branch)
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

// defaultBranch resolves the ref to judge "merged into" AGAINST — the REMOTE-TRACKING
// ref (`origin/main`) whenever the repo has one, and only then the local branch.
//
// It used to strip the `origin/` prefix and hand back the local branch name, which made
// both git-side probes read a base that is stale exactly when it matters: a merge lands
// on the REMOTE, and nothing pulls it into the local `main` — `gh pr merge` does not,
// and in a worktree layout the local `main` belongs to a different checkout that may be
// pinned for other work. Measured on the branch that hit this: judged against the local
// `main`, the squash-equivalence scan had an EMPTY commit range and said "not merged";
// against `origin/main` the branch tip's tree matched the squash commit exactly.
//
// The local fallback still matters — a repo with no remote (and every test that builds
// one) has nothing else to compare against.
func defaultBranch(wt string) string {
	if head, err := gitOutput(wt, "rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil && head != "" {
		return head
	}
	for _, b := range []string{"main", "master"} {
		if gitRun(wt, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+b) == nil {
			return "origin/" + b
		}
		if gitRun(wt, "rev-parse", "--verify", "--quiet", "refs/heads/"+b) == nil {
			return b
		}
	}
	return ""
}

// FetchBase refreshes the remote-tracking ref BranchMerged judges against. Best-effort
// and bounded: a failure (offline, no remote, a local-only base) leaves the existing
// refs alone and the caller judges on what it has.
//
// It is deliberately NOT inside BranchMerged. The reap-suggest sweep calls that on a
// hook, per candidate, where a network round-trip has no business being; `gtmux reap`
// is rare, human-invoked and DESTRUCTIVE, so it pays one fetch to judge on current
// facts. Without it, fixing the base to `origin/main` only helps someone who happened
// to have fetched since the merge.
func FetchBase(wt string) {
	base := defaultBranch(wt)
	remote, ref, ok := strings.Cut(base, "/")
	if !ok || remote == "" || ref == "" {
		return // a local base — nothing to refresh
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", wt, "fetch", "--quiet", remote, ref)
	cmd.WaitDelay = 2 * time.Second
	_ = cmd.Run()
}
