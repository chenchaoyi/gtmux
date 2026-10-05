package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// The note spawn prints when a new --worktree branch starts somewhere other than the
// default branch's tip: where it started, by how much, and how to start from the tip.
func TestWorktreeBaseNote(t *testing.T) {
	if en, zh := worktreeBaseNote("feat/x", "/wt/feat-x", dispatch.BranchBase{Ref: "main", Commit: "abc1234", Upstream: "origin/main"}); en != "" || zh != "" {
		t.Fatalf("a base at the tip must say nothing, got %q / %q", en, zh)
	}
	if en, _ := worktreeBaseNote("feat/x", "/wt/feat-x", dispatch.BranchBase{Commit: "abc1234", Behind: 3}); en != "" {
		t.Fatalf("no default branch to compare with must say nothing, got %q", en)
	}

	en, zh := worktreeBaseNote("hq/review", "/wt/hq-review", dispatch.BranchBase{Ref: "fix/old", Commit: "2ece543", Upstream: "origin/main", Behind: 120, Ahead: 1})
	for _, want := range []string{
		"hq/review starts from fix/old @ 2ece543",
		"120 commits behind origin/main and 1 commit not on it, as of the last fetch.",
		// The branch exists by the time this prints: the fix now is a move, and creating
		// first is for next time.
		"To move it onto origin/main: git -C /wt/hq-review rebase --onto origin/main 2ece543.",
		"Next time, create the branch first (git branch hq/review origin/main)",
	} {
		if !strings.Contains(en, want) {
			t.Errorf("en note missing %q:\n%s", want, en)
		}
	}
	for _, want := range []string{"fix/old @ 2ece543", "比 origin/main 落后 120 个提交，有 1 个提交不在 origin/main 上（按上次 fetch）", "git -C /wt/hq-review rebase --onto origin/main 2ece543", "下次先建好分支（git branch hq/review origin/main）"} {
		if !strings.Contains(zh, want) {
			t.Errorf("zh note missing %q:\n%s", want, zh)
		}
	}

	// Detached, and a local default branch: no "@", and no fetch to speak of.
	en, zh = worktreeBaseNote("feat/y", "/wt/feat-y", dispatch.BranchBase{Commit: "def5678", Upstream: "main", Behind: 1})
	if !strings.Contains(en, "feat/y starts from def5678 (") || !strings.Contains(en, "1 commit behind main.") || strings.Contains(en, "fetch") {
		t.Errorf("detached/local en note = %q", en)
	}
	if strings.Contains(zh, "fetch") || strings.Contains(zh, "不在") {
		t.Errorf("detached/local zh note = %q", zh)
	}
}
