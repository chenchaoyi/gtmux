package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
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
	killPane       func(pane string) error // reclaim only a verified bare pane
	removeWorktree func(wt string, force bool) error
	deleteBranch   func(repo, branch string, force bool) error
}

// reapResult is the outcome of a reclamation attempt.
type reapResult struct {
	ResourceClosed bool     `json:"resource_closed"`
	Reaped         bool     `json:"reaped"`
	BlockedBy      []string `json:"blocked_by,omitempty"`
	Actions        []string `json:"actions,omitempty"`
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
		err := ops.killSession(t.Session)
		res.ResourceClosed = err == nil
		record(err, "killed session "+t.Session,
			"could not kill session "+t.Session)
	case t.Session == "" && t.Pane != "":
		// A manually created pane can share its window with another live pane.
		// Missing identity verification is a failure, never a successful no-op.
		err := errors.New("pane identity was not verified")
		if ops.killPane != nil {
			err = ops.killPane(t.Pane)
		}
		res.ResourceClosed = err == nil
		record(err, "killed pane "+t.Pane,
			"could not kill pane "+t.Pane)
	}
	// Never remove a worktree that may still be used by the resource we failed
	// to close. Preserve its ledger and report the failed step for recovery.
	if len(res.Failed) > 0 {
		return res
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
		removeWorktree: dispatch.RemoveWorktree,
		deleteBranch:   dispatch.DeleteBranch,
	}
}

// barePaneIdentity binds the bare-pane fallback to the tmux server, pane process,
// and cwd observed when its worktree/branch context was derived.
type barePaneIdentity struct {
	serverPID string
	pane      string
	panePID   string
	cwd       string
}

func readBarePaneIdentity(pane string) (barePaneIdentity, error) {
	out, err := tmux.Run("display-message", "-p", "-t", pane,
		"#{pid}\t#{pane_id}\t#{pane_pid}\t#{pane_current_path}\t#{pane_dead}")
	if err != nil {
		return barePaneIdentity{}, fmt.Errorf("target pane is no longer live: %w", err)
	}
	parts := strings.Split(out, "\t")
	if len(parts) != 5 || parts[0] == "" || parts[1] != pane || parts[2] == "" || parts[3] == "" || parts[4] != "0" {
		return barePaneIdentity{}, fmt.Errorf("target pane %s is missing or has invalid identity", pane)
	}
	return barePaneIdentity{parts[0], parts[1], parts[2], parts[3]}, nil
}

func killVerifiedBarePane(expected barePaneIdentity, task dispatch.Task) error {
	current, err := readBarePaneIdentity(expected.pane)
	if err != nil {
		return err
	}
	if current != expected {
		return fmt.Errorf("target pane %s changed identity or cwd before reap", expected.pane)
	}
	// The branch can change while the pane stays in the same directory. Recheck
	// the context used by the worktree/merge gate before closing anything.
	wt, branch, linked, ok := dispatch.WorktreeContext(current.cwd)
	if want := barePaneTask(expected.pane, wt, branch, linked && ok); want.Worktree != task.Worktree || want.Branch != task.Branch {
		return fmt.Errorf("target pane %s changed worktree or branch before reap", expected.pane)
	}
	if task.Worktree != "" {
		// A linked worktree is a shared filesystem resource. A second pane using
		// it must not be left running in a directory this reap removes.
		panes, err := tmux.Run("list-panes", "-a", "-F", "#{pane_id}\t#{pane_current_path}")
		if err != nil {
			return fmt.Errorf("cannot verify other panes before removing worktree: %w", err)
		}
		for _, line := range strings.Split(panes, "\n") {
			parts := strings.Split(line, "\t")
			if len(parts) != 2 {
				return errors.New("cannot verify other panes before removing worktree")
			}
			if parts[0] == expected.pane {
				continue
			}
			rel, err := filepath.Rel(task.Worktree, parts[1])
			if err != nil {
				return fmt.Errorf("cannot compare pane %s with worktree: %w", parts[0], err)
			}
			if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				return fmt.Errorf("worktree is still used by pane %s", parts[0])
			}
		}
	}
	if current, err = readBarePaneIdentity(expected.pane); err != nil {
		return err
	} else if current != expected {
		return fmt.Errorf("target pane %s changed identity or cwd before reap", expected.pane)
	}
	_, err = tmux.Run("kill-pane", "-t", expected.pane)
	return err
}

// reapTaskFromPane builds a synthetic reap Task for a MANUAL pane (no ledger entry)
// from just its live pane: the enclosing linked worktree + branch are reclaimed under
// the SAME safety gate, and only that pane is killed. ok is false when the pane
// is not live. A pane outside a linked worktree yields a pane-only reclaim.
func reapTaskFromPane(pane string) (dispatch.Task, barePaneIdentity, bool) {
	identity, err := readBarePaneIdentity(pane)
	if err != nil {
		return dispatch.Task{}, barePaneIdentity{}, false
	}
	wt, branch, isLinked, ok := dispatch.WorktreeContext(identity.cwd)
	if !ok {
		return dispatch.Task{Pane: pane}, identity, true // not a git repo → pane only
	}
	return barePaneTask(pane, wt, branch, isLinked), identity, true
}

// barePaneTask is the pure synthesis: a linked worktree is reclaimed (worktree+branch,
// gated); otherwise (the main checkout, or a detached HEAD) only the pane is killed.
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
	var bareIdentity barePaneIdentity
	if !ok {
		// Not in the ledger — a live pane can be reclaimed without owning its window.
		t, bareIdentity, ok = reapTaskFromPane(target)
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
				"gtmux reap: --snooze 只能用于已登记的派活，裸 pane 不行")
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
		diag.Did("act.reap.snooze", t.Pane, diag.OK, "silenced a reap suggestion", "task", t.ID, "until", until)
		if asJSON {
			b, _ := json.MarshalIndent(map[string]any{"snoozed": true, "snooze_until": until}, "", "  ")
			fmt.Println(string(b))
		} else {
			i18n.Say("• snoozed reap suggestions for this task", "• 已静默该任务的回收建议")
		}
		return 0
	}

	identity := verifiedReapIdentity(t)
	ops := liveReapOps()
	if bareIdentity.pane != "" {
		ops.killPane = func(pane string) error { return killVerifiedBarePane(bareIdentity, t) }
	}
	res := planAndReap(t, abandon, keepBranch, ops)
	if res.Reaped {
		// Journaled BEFORE the ledger entry goes: the task file is about to be the
		// only other record of this dispatch, and it is deleted on the next line.
		id := t.ID
		if id == "" {
			id = "(bare pane)"
		}
		identity.TaskID, identity.ResourceClosed, identity.Failed = id, res.ResourceClosed, res.Failed
		events.AuditReapResult(identity, strings.Join(res.Actions, "; "), time.Now().Unix())
	}
	if !res.Reaped {
		// Report-only: the gate refused (unclean worktree, unmerged branch). The reap
		// that did happen is recorded with its journal record above.
		diag.Did("act.reap", t.Pane, diag.Refused, "a dispatch was not reclaimed", "task", t.ID,
			"reason", "gate", "blockedBy", strings.Join(res.BlockedBy, ","))
	}
	if res.Reaped && len(res.Failed) == 0 && t.ID != "" { // preserve recovery information for partial failures
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
			i18n.Sae("⚠ but these steps failed; reclaim them by hand:",
				"⚠ 但以下步骤失败了，需要手动收尾：")
			for _, f := range res.Failed {
				fmt.Println("  · " + f)
			}
		}
	} else {
		i18n.Sae("✗ not reaped, blocked by:", "✗ 未回收，被以下项阻止：")
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

// verifiedReapIdentity samples the resource before kill. A historic resume record
// alone cannot label a replacement process, so only an exact binding receipt wins.
func verifiedReapIdentity(t dispatch.Task) events.ReapContext {
	v := events.ReapContext{TaskID: t.ID, Pane: t.Pane, Session: t.Session}
	live, ok := resume.LiveCodexBindingTarget(t.Pane)
	if !ok {
		return v
	}
	r, ok := resume.Load(live.Loc)
	if !ok || r.Agent != "codex" {
		return v
	}
	b, ok := native.BindingFor(r.SessionID, r.HomeID)
	if ok && reapBindingMatches(b, live) {
		v.Loc, v.AgentSession, v.BindingID = live.Loc, r.SessionID, b.ID
	}
	return v
}

func reapBindingMatches(b native.Binding, live resume.CodexBindingTarget) bool {
	return b.SessionID != "" && b.Pane == live.Pane && b.Loc == live.Loc && b.Cwd == live.Cwd &&
		b.PID == live.PID && b.PanePID == live.PanePID && live.PID > 0 && live.PanePID > 0
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
