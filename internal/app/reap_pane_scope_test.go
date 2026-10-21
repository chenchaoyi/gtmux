package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func privateReapTmux(t *testing.T) string {
	t.Helper()
	if tmux.Bin == "" {
		t.Skip("tmux not available")
	}
	sock, err := os.MkdirTemp("/tmp", "gt-reap-") // short Unix socket path
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(sock, "home"))
	t.Setenv("TMPDIR", sock)
	t.Setenv("TMUX_TMPDIR", sock)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("GTMUX_LANG", "en")
	if err := os.Mkdir(filepath.Join(sock, "home"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sock) })
	return sock
}

func startReapSession(t *testing.T, session, cwd string) string {
	t.Helper()
	if _, err := tmux.Run("new-session", "-d", "-s", session, "-c", cwd, "sleep 300"); err != nil {
		t.Fatalf("start isolated tmux fixture: %v", err)
	}
	t.Cleanup(func() { _, _ = tmux.Run("kill-session", "-t", session) })
	pane := tmux.Display(session, "#{pane_id}")
	if pane == "" {
		t.Fatal("fixture pane missing")
	}
	return pane
}

// A real tmux window with two live processes catches collateral closure that a
// mocked kill operation cannot: the bare-pane fallback must close only its target.
func TestReapLive_BarePaneKeepsSiblingProcess(t *testing.T) {
	sock := privateReapTmux(t)
	const session = "reap-scope-test"
	target := startReapSession(t, session, sock)
	sibling, err := tmux.Run("split-window", "-d", "-P", "-F", "#{pane_id}", "-t", target, "-c", sock, "sleep 300")
	if err != nil || target == "" || sibling == "" {
		t.Fatalf("split isolated window: target=%q sibling=%q err=%v", target, sibling, err)
	}
	siblingPID, err := strconv.Atoi(tmux.Display(sibling, "#{pane_pid}"))
	if err != nil || siblingPID <= 0 {
		t.Fatalf("sibling pane PID: %d, %v", siblingPID, err)
	}
	if code := cmdReap([]string{target, "--keep-branch", "--json"}); code != 0 {
		t.Fatalf("bare-pane reap exited %d", code)
	}
	if got := tmux.Display(target, "#{pane_id}"); got != "" {
		t.Fatalf("target %s survived: %q", target, got)
	}
	if got := tmux.Display(sibling, "#{pane_id}"); got != sibling {
		t.Fatalf("sibling %s disappeared with target: %q", sibling, got)
	}
	if got := tmux.Display(sibling, "#{pane_pid}"); got != strconv.Itoa(siblingPID) {
		t.Fatalf("sibling process changed: before=%d after=%q", siblingPID, got)
	}
	if err := syscall.Kill(siblingPID, 0); err != nil {
		t.Fatalf("sibling process %d is not alive: %v", siblingPID, err)
	}
}

func TestReapLive_BarePaneRejectsChangedAndMissingTarget(t *testing.T) {
	sock := privateReapTmux(t)
	pane := startReapSession(t, "reap-identity-test", sock)
	task := dispatch.Task{Pane: pane}
	expected, err := readBarePaneIdentity(pane)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmux.Run("respawn-pane", "-k", "-t", pane, "sleep 300"); err != nil {
		t.Fatalf("replace fixture process: %v", err)
	}
	if current, err := readBarePaneIdentity(pane); err != nil || current.panePID == expected.panePID {
		t.Fatalf("fixture did not change pane process: before=%+v after=%+v err=%v", expected, current, err)
	}
	if err := killVerifiedBarePane(expected, task); err == nil || !strings.Contains(err.Error(), "changed identity") {
		t.Fatalf("changed target must be refused, got %v", err)
	}
	if got := tmux.Display(pane, "#{pane_id}"); got != pane {
		t.Fatalf("replacement pane was closed: %q", got)
	}
	if _, err := tmux.Run("kill-pane", "-t", pane); err != nil {
		t.Fatal(err)
	}
	ops := liveReapOps()
	ops.killPane = func(string) error { return killVerifiedBarePane(expected, task) }
	res := planAndReap(task, false, false, ops)
	if res.ResourceClosed || len(res.Failed) != 1 || !strings.Contains(res.Failed[0], "no longer live") {
		t.Fatalf("missing target must report failed closure: %+v", res)
	}
	if code := cmdReap([]string{pane, "--json"}); code != 1 {
		t.Fatalf("missing bare pane must not resolve as a target; exit=%d", code)
	}
}

func TestReapLive_BarePaneKeepsSharedWorktreeUntilOtherPaneLeaves(t *testing.T) {
	privateReapTmux(t)
	repo, wt := squashMergedRepo(t, "feat/x")
	target := startReapSession(t, "reap-shared-worktree-test", wt)
	sibling, err := tmux.Run("split-window", "-d", "-P", "-F", "#{pane_id}", "-t", target, "-c", wt, "sleep 300")
	if err != nil {
		t.Fatal(err)
	}
	if code := cmdReap([]string{target, "--keep-branch", "--json"}); code != 1 {
		t.Fatalf("shared worktree must block bare-pane reap; exit=%d", code)
	}
	if tmux.Display(target, "#{pane_id}") != target || tmux.Display(sibling, "#{pane_id}") != sibling || !branchExists(t, repo, "feat/x") {
		t.Fatal("shared worktree refusal changed a pane or branch")
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("shared worktree was removed: %v", err)
	}
	if _, err := tmux.Run("kill-pane", "-t", sibling); err != nil {
		t.Fatal(err)
	}
	if code := cmdReap([]string{target, "--keep-branch", "--json"}); code != 0 {
		t.Fatalf("bare-pane reap after sibling leaves exited %d", code)
	}
	if tmux.Display(target, "#{pane_id}") != "" || !branchExists(t, repo, "feat/x") {
		t.Fatal("target pane should close while --keep-branch preserves the branch")
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("private worktree should be removed, stat err=%v", err)
	}
}

func TestReapLive_BarePaneDisappearsAfterGatePreservesWorktree(t *testing.T) {
	privateReapTmux(t)
	repo, wt := squashMergedRepo(t, "feat/x")
	pane := startReapSession(t, "reap-lost-worktree-test", wt)
	task := barePaneTask(pane, wt, "feat/x", true)
	expected, err := readBarePaneIdentity(pane)
	if err != nil {
		t.Fatal(err)
	}
	ops := liveReapOps()
	ops.killPane = func(string) error {
		// Deterministically remove the target after the worktree gate, before
		// reap's late identity check. Only this private fixture pane is touched.
		if _, err := tmux.Run("kill-pane", "-t", pane); err != nil {
			return err
		}
		return killVerifiedBarePane(expected, task)
	}
	res := planAndReap(task, false, true, ops)
	if res.ResourceClosed || len(res.Failed) != 1 || !strings.Contains(res.Failed[0], "no longer live") {
		t.Fatalf("lost target must report failed closure: %+v", res)
	}
	if _, err := os.Stat(wt); err != nil || !branchExists(t, repo, "feat/x") {
		t.Fatalf("failed closure must preserve worktree and branch: stat err=%v", err)
	}
}

func TestReapLive_DispatchKeepsGateAndOwnedSessionLifecycle(t *testing.T) {
	privateReapTmux(t)
	repo, wt := squashMergedRepo(t, "feat/x")
	pane := startReapSession(t, "reap-owned-test", wt)
	other := startReapSession(t, "reap-unowned-test", repo)
	task := dispatch.Task{ID: "owned", Pane: pane, Session: "reap-owned-test", OwnSession: true,
		Worktree: wt, Branch: "feat/x"}
	write(t, wt, "uncommitted.txt", "keep me")
	blocked := planAndReap(task, false, false, liveReapOps())
	if blocked.Reaped || len(blocked.BlockedBy) == 0 || tmux.Display(pane, "#{pane_id}") != pane || !branchExists(t, repo, "feat/x") {
		t.Fatalf("dirty dispatch must leave its session and worktree intact: %+v", blocked)
	}
	if err := os.Remove(filepath.Join(wt, "uncommitted.txt")); err != nil {
		t.Fatal(err)
	}
	res := planAndReap(task, false, false, liveReapOps())
	if !res.Reaped || !res.ResourceClosed || len(res.Failed) != 0 {
		t.Fatalf("clean merged dispatch should close owned session and worktree: %+v", res)
	}
	if tmux.Display(pane, "#{pane_id}") != "" || tmux.Display(other, "#{pane_id}") != other {
		t.Fatal("owned session closure must leave the unrelated session running")
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) || branchExists(t, repo, "feat/x") {
		t.Fatalf("merged dispatch worktree/branch not reclaimed: worktree err=%v", err)
	}
}
