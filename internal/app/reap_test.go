package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// spyOps records what was called and lets each check be steered.
type spyOps struct {
	dirty      bool
	dirtyErr   error
	merged     bool
	mergedErr  error
	killed     bool
	winKilled  bool
	removed    bool
	branchGone bool

	// The reclamation ops can FAIL. They used to be stubbed as unfailable (`return
	// nil`, always), which is exactly why every reap test stayed green while the real
	// branch delete died on every single invocation: the tests asserted the step was
	// CALLED, and nothing downstream of the call existed to be wrong.
	deleteBranchErr error
	removeErr       error

	// What deleteBranch was actually handed — the dir matters as much as the branch
	// (it must be the repo, resolved while the worktree still existed) and so does
	// force (a squash merge is invisible to `git branch -d`).
	deleteDir   string
	deleteForce bool
	order       []string // call order, to pin "resolve before remove"
}

func (s *spyOps) ops() reapOps {
	return reapOps{
		worktreeDirty: func(string) (bool, error) { return s.dirty, s.dirtyErr },
		branchMerged:  func(string, string) (bool, error) { return s.merged, s.mergedErr },
		mainRepo: func(wt string) string {
			s.order = append(s.order, "mainRepo")
			return "/repo"
		},
		killSession: func(string) error { s.killed = true; return nil },
		killWindow:  func(string) error { s.winKilled = true; return nil },
		removeWorktree: func(string, bool) error {
			s.order = append(s.order, "removeWorktree")
			s.removed = s.removeErr == nil
			return s.removeErr
		},
		deleteBranch: func(dir, _ string, force bool) error {
			s.order = append(s.order, "deleteBranch")
			s.deleteDir, s.deleteForce = dir, force
			s.branchGone = s.deleteBranchErr == nil
			return s.deleteBranchErr
		},
	}
}

func worktreeTask() dispatch.Task {
	return dispatch.Task{
		ID: "t1", Pane: "%1", Session: "sess", OwnSession: true,
		Worktree: "/wt/feat-x", Branch: "feat/x",
	}
}

func TestReap_DirtyWorktree_ReportOnly(t *testing.T) {
	s := &spyOps{dirty: true, merged: true}
	res := planAndReap(worktreeTask(), false, false, s.ops())
	if res.Reaped {
		t.Fatalf("dirty worktree must not be reaped")
	}
	if s.killed || s.removed || s.branchGone {
		t.Fatalf("a blocked reap must touch nothing: %+v", s)
	}
	if len(res.BlockedBy) == 0 {
		t.Fatalf("must report what blocks it")
	}
}

func TestReap_UnmergedBranch_ReportOnly(t *testing.T) {
	s := &spyOps{dirty: false, merged: false}
	res := planAndReap(worktreeTask(), false, false, s.ops())
	if res.Reaped || s.removed {
		t.Fatalf("unmerged branch must be report-only")
	}
}

func TestReap_MergeStateUnknown_FailsSafe(t *testing.T) {
	s := &spyOps{dirty: false, mergedErr: errors.New("no default branch")}
	res := planAndReap(worktreeTask(), false, false, s.ops())
	if res.Reaped {
		t.Fatalf("unknown merge state must fail safe (report-only)")
	}
}

func TestReap_Abandon_Overrides(t *testing.T) {
	s := &spyOps{dirty: true, merged: false}
	res := planAndReap(worktreeTask(), true, false, s.ops())
	if !res.Reaped {
		t.Fatalf("--abandon must override the gate")
	}
	if !s.killed || !s.removed || !s.branchGone {
		t.Fatalf("--abandon should kill+remove+delete: %+v", s)
	}
}

func TestReap_CleanMerged_Reaps(t *testing.T) {
	s := &spyOps{dirty: false, merged: true}
	res := planAndReap(worktreeTask(), false, false, s.ops())
	if !res.Reaped {
		t.Fatalf("clean+merged should reap, blocked=%v", res.BlockedBy)
	}
	if !s.killed || !s.removed || !s.branchGone {
		t.Fatalf("clean reap should kill+remove+delete: %+v", s)
	}
}

func TestReap_KeepBranch(t *testing.T) {
	s := &spyOps{dirty: false, merged: true}
	planAndReap(worktreeTask(), false, true, s.ops())
	if s.branchGone {
		t.Fatalf("--keep-branch must not delete the branch")
	}
}

// --keep-branch never deletes the branch, so an unmerged branch's commits stay
// reachable via the kept ref — the merge gate must not block the worktree
// reclaim in that case (only a dirty worktree should still block it).
func TestReap_KeepBranch_SkipsMergeGate(t *testing.T) {
	s := &spyOps{dirty: false, merged: false}
	res := planAndReap(worktreeTask(), false, true, s.ops())
	if !res.Reaped {
		t.Fatalf("--keep-branch should reap an unmerged branch's worktree, blocked=%v", res.BlockedBy)
	}
	if !s.removed {
		t.Fatalf("--keep-branch should still remove the worktree: %+v", s)
	}
	if s.branchGone {
		t.Fatalf("--keep-branch must not delete the branch: %+v", s)
	}
}

// A dirty worktree must still block the reap even with --keep-branch — that
// gate is about uncommitted work, not the branch's merge state.
func TestReap_KeepBranch_StillBlocksOnDirty(t *testing.T) {
	s := &spyOps{dirty: true, merged: false}
	res := planAndReap(worktreeTask(), false, true, s.ops())
	if res.Reaped {
		t.Fatalf("--keep-branch must not bypass the dirty-worktree gate")
	}
}

func TestReap_NoWorktree_JustSession(t *testing.T) {
	// A plain --pane dispatch (no worktree) reaps by killing only an owned session.
	s := &spyOps{}
	task := dispatch.Task{ID: "t2", Session: "sess", OwnSession: true}
	res := planAndReap(task, false, false, s.ops())
	if !res.Reaped || !s.killed {
		t.Fatalf("a no-worktree owned dispatch should kill its session")
	}
	if s.removed || s.branchGone {
		t.Fatalf("no worktree/branch to remove")
	}
}

func TestReap_ReusedPane_DoesNotKillSession(t *testing.T) {
	// A reused pane (OwnSession=false) must never kill the user's session.
	s := &spyOps{}
	task := dispatch.Task{ID: "t3", Session: "user-sess", OwnSession: false}
	res := planAndReap(task, false, false, s.ops())
	if !res.Reaped {
		t.Fatalf("should still succeed (nothing to reclaim)")
	}
	if s.killed {
		t.Fatalf("must NOT kill a session spawn did not create")
	}
}

// barePaneTask: a linked worktree is reclaimed; the main checkout / detached HEAD is
// window-only.
func TestBarePaneTask(t *testing.T) {
	linked := barePaneTask("%9", "/wt/feat-y", "feat/y", true)
	if linked.Pane != "%9" || linked.Worktree != "/wt/feat-y" || linked.Branch != "feat/y" || linked.Session != "" {
		t.Fatalf("linked worktree task = %+v", linked)
	}
	main := barePaneTask("%9", "/repo", "main", false)
	if main.Worktree != "" || main.Branch != "" || main.Pane != "%9" {
		t.Fatalf("main-checkout pane should be window-only: %+v", main)
	}
	det := barePaneTask("%9", "/wt/x", "HEAD", true)
	if det.Branch != "" {
		t.Fatalf("detached HEAD must not delete a branch: %+v", det)
	}
}

// A bare-pane reap of a MANUAL window kills the WINDOW (never a session) under the same
// gate, and reclaims its worktree/branch.
func TestReap_BarePane_KillsWindowNotSession(t *testing.T) {
	s := &spyOps{dirty: false, merged: true}
	task := barePaneTask("%28", "/wt/menubar-width", "feat/menubar-width", true)
	res := planAndReap(task, false, false, s.ops())
	if !res.Reaped {
		t.Fatalf("clean+merged bare pane should reap, blocked=%v", res.BlockedBy)
	}
	if s.killed {
		t.Fatalf("bare-pane reap must NOT kill a session")
	}
	if !s.winKilled || !s.removed || !s.branchGone {
		t.Fatalf("bare-pane reap should kill window + remove worktree + delete branch: %+v", s)
	}
}

func TestReap_BarePane_DirtyReportOnly(t *testing.T) {
	s := &spyOps{dirty: true, merged: true}
	task := barePaneTask("%28", "/wt/x", "feat/x", true)
	res := planAndReap(task, false, false, s.ops())
	if res.Reaped || s.winKilled || s.removed {
		t.Fatalf("a dirty bare-pane worktree must be report-only: reaped=%v %+v", res.Reaped, s)
	}
}

// The branch's repo MUST be resolved before the worktree is removed. `git branch` runs
// from the main repo, and reap resolved it by asking git from inside the worktree it had
// just deleted — `fatal: cannot change to '<worktree>'`, on every reap, for every branch.
// (spawn's rollback already resolves first; MainRepo is exported for exactly this.)
func TestReap_ResolvesTheRepoBeforeRemovingTheWorktree(t *testing.T) {
	s := &spyOps{dirty: false, merged: true}
	planAndReap(worktreeTask(), false, false, s.ops())
	if got := strings.Join(s.order, ","); got != "mainRepo,removeWorktree,deleteBranch" {
		t.Fatalf("call order = %q, want the repo resolved BEFORE the worktree is removed", got)
	}
	if s.deleteDir != "/repo" {
		t.Errorf("deleteBranch dir = %q, want the resolved repo — a removed worktree is not a dir git can cd into", s.deleteDir)
	}
}

// `git branch -d` re-checks the merge itself, and its check only accepts an ancestor of
// HEAD/upstream — it structurally cannot see a SQUASH merge, which is what this repo (and
// GitHub by default) produces. So once reap's own, strictly stronger gate has confirmed
// the merge, the delete must force, or git's weaker re-check silently overrules it.
func TestReap_MergedGateForcesTheDelete(t *testing.T) {
	s := &spyOps{dirty: false, merged: true}
	planAndReap(worktreeTask(), false, false, s.ops())
	if !s.deleteForce {
		t.Fatal("a gate-confirmed merge must force the delete; `-d` cannot see a squash merge")
	}
}

// …but only because a gate RAN. A branch with no worktree never reaches the merge gate,
// so nothing has vouched for it and `git branch -d`'s own check stays the last word.
func TestReap_NoGate_DoesNotForceTheDelete(t *testing.T) {
	s := &spyOps{}
	task := dispatch.Task{ID: "t1", Session: "sess", OwnSession: true, Branch: "feat/x"}
	planAndReap(task, false, false, s.ops())
	if !s.branchGone {
		t.Fatal("the branch step should still run")
	}
	if s.deleteForce {
		t.Fatal("no gate ran — forcing here would delete an unvouched-for branch")
	}
}

// A reclamation step that FAILS must say so. `if op(...) == nil { record it }` records a
// success and is silent about a failure, so the reap that could not delete its branch
// printed a ✓, omitted the branch line, and exited 0 — the user's only signal that
// anything was wrong was a line that wasn't there.
func TestReap_FailedStepIsReportedNotSwallowed(t *testing.T) {
	s := &spyOps{dirty: false, merged: true, deleteBranchErr: errors.New("the branch is not fully merged")}
	res := planAndReap(worktreeTask(), false, false, s.ops())
	if len(res.Failed) != 1 || !strings.Contains(res.Failed[0], "feat/x") ||
		!strings.Contains(res.Failed[0], "not fully merged") {
		t.Fatalf("a failed step must name the branch and git's reason, got %v", res.Failed)
	}
	for _, a := range res.Actions {
		if strings.Contains(a, "deleted branch") {
			t.Fatalf("a failed delete must not be reported as an action: %v", res.Actions)
		}
	}
}

// A live pane not inside a worktree (main checkout) → the window is reclaimed, nothing
// git-side is touched.
func TestReap_BarePane_WindowOnly(t *testing.T) {
	s := &spyOps{}
	task := barePaneTask("%5", "/repo", "main", false)
	res := planAndReap(task, false, false, s.ops())
	if !res.Reaped || !s.winKilled {
		t.Fatalf("window-only bare pane should reap the window")
	}
	if s.removed || s.branchGone || s.killed {
		t.Fatalf("window-only reap must touch no worktree/branch/session: %+v", s)
	}
}
