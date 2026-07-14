package app

import (
	"errors"
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
}

func (s *spyOps) ops() reapOps {
	return reapOps{
		worktreeDirty:  func(string) (bool, error) { return s.dirty, s.dirtyErr },
		branchMerged:   func(string, string) (bool, error) { return s.merged, s.mergedErr },
		killSession:    func(string) error { s.killed = true; return nil },
		killWindow:     func(string) error { s.winKilled = true; return nil },
		removeWorktree: func(string, bool) error { s.removed = true; return nil },
		deleteBranch:   func(string, string, bool) error { s.branchGone = true; return nil },
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
