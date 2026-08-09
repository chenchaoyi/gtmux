package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// reapOps are the side-effecting operations reap performs. Injected so the safety
// gate + reclamation logic is unit-testable without a real repo/tmux server.
type reapOps struct {
	worktreeDirty func(wt string) (bool, error)
	branchMerged  func(wt, branch string) (bool, error)
	// mainRepo resolves the repo the branch lives in. It is a SEPARATE op because it
	// must run BEFORE removeWorktree: it answers by asking git from inside the
	// worktree, and once that directory is gone there is nothing left to ask.
	mainRepo       func(wt string) string
	killSession    func(session string) error
	killWindow     func(pane string) error // reclaim a manual window (bare-pane reap)
	removeWorktree func(wt string, force bool) error
	deleteBranch   func(repo, branch string, force bool) error
}

// reapResult is the outcome of a reclamation attempt.
type reapResult struct {
	Reaped    bool     `json:"reaped"`
	BlockedBy []string `json:"blocked_by,omitempty"`
	Actions   []string `json:"actions,omitempty"`
	// Failed names the reclamation steps that ran and did NOT succeed. Every one of
	// them used to be dropped on the floor — `if op(...) == nil { record it }` records
	// a success and says nothing at all about a failure — so a reap whose branch delete
	// died printed a ✓ with the branch line simply absent, and exited 0. Silence is the
	// one thing a destructive command must never do about work it did not do.
	Failed []string `json:"failed,omitempty"`
}

// planAndReap runs the safety gate FIRST (worktree clean + branch merged, unless
// --abandon) and only on a pass performs the reclamation. On a gate failure it
// returns what blocks it and touches NOTHING. Pure logic; ops injected.
func planAndReap(t dispatch.Task, abandon, keepBranch bool, ops reapOps) reapResult {
	mergeConfirmed := false
	if t.Worktree != "" && !abandon {
		var blocked []string
		if dirty, err := ops.worktreeDirty(t.Worktree); err != nil {
			blocked = append(blocked, "worktree status unknown: "+err.Error())
		} else if dirty {
			blocked = append(blocked, "worktree has uncommitted changes")
		}
		// --keep-branch never deletes the branch, only the worktree — so an
		// unmerged branch poses no data-loss risk here (its commits stay
		// reachable via the kept branch ref) and shouldn't block the reap.
		if t.Branch != "" && !keepBranch {
			if merged, err := ops.branchMerged(t.Worktree, t.Branch); err != nil {
				blocked = append(blocked, "merge state unknown: "+err.Error())
			} else if !merged {
				blocked = append(blocked, "branch '"+t.Branch+"' is not merged")
			} else {
				mergeConfirmed = true
			}
		}
		if len(blocked) > 0 {
			return reapResult{Reaped: false, BlockedBy: blocked}
		}
	}
	// Gate passed (or --abandon) → execute.
	res := reapResult{Reaped: true}
	record := func(err error, did, failed string) {
		if err == nil {
			res.Actions = append(res.Actions, did)
			return
		}
		res.Failed = append(res.Failed, failed+": "+err.Error())
	}
	switch {
	case t.OwnSession && t.Session != "":
		record(ops.killSession(t.Session), "killed session "+t.Session,
			"could not kill session "+t.Session)
	case t.Session == "" && t.Pane != "":
		// Bare-pane reap of a MANUAL window: kill the window, not a whole session
		// (which could hold sibling windows the user still wants).
		if ops.killWindow != nil {
			record(ops.killWindow(t.Pane), "killed window "+t.Pane,
				"could not kill window "+t.Pane)
		}
	}
	// Resolve the branch's repo while the worktree still EXISTS. Removing it first and
	// then asking git, from inside the directory just deleted, is how the branch step
	// came to die on `fatal: cannot change to '<worktree>'` — after every reap, for
	// every branch. (spawn's rollback already resolves first; MainRepo is exported for
	// exactly this ordering. reap simply never adopted it.)
	repo := t.Worktree
	if t.Worktree != "" && ops.mainRepo != nil {
		repo = ops.mainRepo(t.Worktree)
	}
	if t.Worktree != "" {
		record(ops.removeWorktree(t.Worktree, abandon), "removed worktree "+t.Worktree,
			"could not remove worktree "+t.Worktree)
	}
	if t.Branch != "" && !keepBranch {
		// Force whenever an authority stronger than `git branch -d`'s own check has
		// already spoken: --abandon (the user), or our merge gate, which accepts the
		// squash merge that `-d` structurally cannot see. Without this the gate could
		// pass and git would still refuse — which it did, on every squash-merged
		// branch. When NO gate ran (a branch with no worktree), `-d` stays the check.
		force := abandon || mergeConfirmed
		record(ops.deleteBranch(repo, t.Branch, force), "deleted branch "+t.Branch,
			"could not delete branch "+t.Branch)
	}
	return res
}

// liveReapOps wires planAndReap to real git/tmux (git ops centralized in dispatch).
func liveReapOps() reapOps {
	return reapOps{
		worktreeDirty: dispatch.WorktreeDirty,
		// Refresh the base ref before judging. A merge lands on the REMOTE and nothing
		// pulls it down — `gh pr merge` doesn't, and in a worktree layout the local
		// `main` belongs to another checkout that may be pinned for other work — so the
		// gate would otherwise decide "is this merged?" from facts predating the merge.
		// Only the reap COMMAND pays for it: the reap-suggest sweep runs on a hook and
		// calls BranchMerged directly, with no network.
		branchMerged: func(wt, branch string) (bool, error) {
			dispatch.FetchBase(wt)
			return dispatch.BranchMerged(wt, branch)
		},
		mainRepo: dispatch.MainRepo,
		killSession: func(session string) error {
			_, err := tmux.Run("kill-session", "-t", session)
			return err
		},
		killWindow: func(pane string) error {
			_, err := tmux.Run("kill-window", "-t", pane)
			return err
		},
		removeWorktree: dispatch.RemoveWorktree,
		deleteBranch:   dispatch.DeleteBranch,
	}
}

// reapTaskFromPane builds a synthetic reap Task for a MANUAL window (no ledger entry)
// from just its live pane: the enclosing linked worktree + branch are reclaimed under
// the SAME safety gate, and the window (not a session) is killed. ok is false when the
// pane is not live. A pane not inside a linked worktree yields a window-only reclaim.
func reapTaskFromPane(pane string) (dispatch.Task, bool) {
	if tmux.Display(pane, "#{pane_id}") != pane {
		return dispatch.Task{}, false // not a live pane
	}
	cwd := tmux.Display(pane, "#{pane_current_path}")
	wt, branch, isLinked, ok := dispatch.WorktreeContext(cwd)
	if !ok {
		return dispatch.Task{Pane: pane}, true // live pane, not a git repo → window only
	}
	return barePaneTask(pane, wt, branch, isLinked), true
}

// barePaneTask is the pure synthesis: a linked worktree is reclaimed (worktree+branch,
// gated); otherwise (the main checkout, or a detached HEAD) only the window is killed.
func barePaneTask(pane, worktree, branch string, isLinked bool) dispatch.Task {
	if !isLinked {
		return dispatch.Task{Pane: pane}
	}
	if branch == "HEAD" { // detached — nothing safe to delete
		branch = ""
	}
	return dispatch.Task{Pane: pane, Worktree: worktree, Branch: branch}
}

// cmdReap implements `gtmux reap <pane|task_id> [--abandon] [--keep-branch]
// [--snooze [--for <dur>]] [--json]`.
func cmdReap(args []string) int {
	var target string
	var abandon, keepBranch, snooze, asJSON bool
	var snoozeFor time.Duration
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			return reapUsage()
		case a == "--abandon":
			abandon = true
		case a == "--keep-branch":
			keepBranch = true
		case a == "--snooze":
			snooze = true
		case a == "--for":
			if i+1 < len(args) {
				i++
				snoozeFor, _ = time.ParseDuration(args[i])
			}
		case a == "--json":
			asJSON = true
		case strings.HasPrefix(a, "--"):
			i18n.Sae("gtmux reap: unknown option '"+a+"'", "gtmux reap: 未知选项 '"+a+"'")
			return 2
		default:
			target = a
		}
	}
	if target == "" {
		return reapUsage()
	}

	t, ok := resolveTask(target)
	if !ok {
		// Not in the ledger — if it's a live pane, reclaim a manual window from its
		// context (M1: closes the "no dispatch" gap for hand-made windows).
		t, ok = reapTaskFromPane(target)
	}
	if !ok {
		i18n.Sae("gtmux reap: no dispatch or live pane for '"+target+"'",
			"gtmux reap: '"+target+"' 既不是派活也不是活的 pane")
		return 1
	}

	// Snooze: silence the reap suggestion without touching anything (incident ⑧). Only a
	// tracked dispatch can be snoozed — a bare pane isn't a ledger suggestion.
	if snooze {
		if t.ID == "" {
			i18n.Sae("gtmux reap: --snooze needs a tracked dispatch, not a bare pane",
				"gtmux reap: --snooze 只能用于已登记的派活,裸 pane 不行")
			return 2
		}
		tune := dispatch.LoadTuning()
		ttl := tune.ReapSnoozeTTL
		if snoozeFor > 0 {
			ttl = int64(snoozeFor.Seconds())
		}
		until := time.Now().Unix() + ttl
		if ttl <= 0 {
			until = 0 // --for 0 clears the snooze
		}
		dispatch.SnoozeTask(t.ID, until)
		if asJSON {
			b, _ := json.MarshalIndent(map[string]any{"snoozed": true, "snooze_until": until}, "", "  ")
			fmt.Println(string(b))
		} else {
			i18n.Say("• snoozed reap suggestions for this task", "• 已静默该任务的回收建议")
		}
		return 0
	}

	res := planAndReap(t, abandon, keepBranch, liveReapOps())
	if res.Reaped && t.ID != "" { // a bare-pane reap has no ledger entry to clear
		dispatch.RemoveTask(t.ID)
	}

	if asJSON {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
	} else if res.Reaped {
		// Only claim a reap for work that actually happened — a ✓ over an empty list,
		// with every step in the ⚠ block below it, reads as a success with a footnote.
		if len(res.Actions) > 0 {
			i18n.Say("✓ reaped:", "✓ 已回收：")
			for _, a := range res.Actions {
				fmt.Println("  · " + a)
			}
		}
		if len(res.Failed) > 0 {
			i18n.Sae("⚠ but these steps failed — reclaim them by hand:",
				"⚠ 但以下步骤失败了 —— 需要手动收尾：")
			for _, f := range res.Failed {
				fmt.Println("  · " + f)
			}
		}
	} else {
		i18n.Sae("✗ not reaped — blocked by:", "✗ 未回收 —— 被以下项阻止：")
		for _, b := range res.BlockedBy {
			fmt.Println("  · " + b)
		}
	}
	// A partial reap is not a success. Exit 0 while a branch the user asked to reclaim
	// is still there is the same lie the missing report was.
	if res.Reaped && len(res.Failed) == 0 {
		return 0
	}
	return 1
}

// resolveTask finds a ledger entry by task id or by pane.
func resolveTask(target string) (dispatch.Task, bool) {
	if t, ok := dispatch.LoadTask(target); ok {
		return t, true
	}
	return dispatch.TaskForPane(target)
}

func reapUsage() int {
	i18n.Sae("usage: gtmux reap <pane|task_id> [--abandon] [--keep-branch] [--snooze [--for <dur>]] [--json]",
		"用法：gtmux reap <pane|task_id> [--abandon] [--keep-branch] [--snooze [--for <时长>]] [--json]")
	return 2
}
