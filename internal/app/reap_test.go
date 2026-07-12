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
	removed    bool
	branchGone bool
}

func (s *spyOps) ops() reapOps {
	return reapOps{
		worktreeDirty:  func(string) (bool, error) { return s.dirty, s.dirtyErr },
		branchMerged:   func(string, string) (bool, error) { return s.merged, s.mergedErr },
		killSession:    func(string) error { s.killed = true; return nil },
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
