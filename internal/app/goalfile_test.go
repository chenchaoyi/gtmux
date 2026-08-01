package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// The goal that broke the argv channel on 2026-08-01 — Chinese prose with backticked
// identifiers, `$`, both quote flavours and newlines. Under `"…"` a shell EXECUTES the
// backticked spans; the real dispatch died on
// `command substitution: syntax error near unexpected token 'done'`.
const nastyGoal = "把 `hqPlaybookVersion` 从 12 提到 13，并且：\n" +
	"1. 运行 `make check`（注意 $HOME、$(pwd) 与 \"双引号\" 和 '单引号'）\n" +
	"2. 千万别让 shell 执行 `for f in *; do echo $f; done`\n" +
	"3. 收尾"

func goalFileWith(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// --goal-file must hand the dispatch the file's bytes, unaltered. This is the assertion
// that matters: not "the command exited 0", but "the instruction is intact".
func TestSpawnGoal_FileIsByteExact(t *testing.T) {
	goal, rc := spawnGoal(goalFileWith(t, nastyGoal+"\n"), nil, nil)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if goal != nastyGoal {
		t.Fatalf("goal was mutated\n got: %q\nwant: %q", goal, nastyGoal)
	}
	if !strings.Contains(goal, "do echo $f; done") || strings.Count(goal, "\n") != 3 {
		t.Errorf("the shell-metacharacter body did not survive intact: %q", goal)
	}
}

func TestSpawnGoal_Stdin(t *testing.T) {
	goal, rc := spawnGoal("-", nil, strings.NewReader(nastyGoal+"\n"))
	if rc != 0 || goal != nastyGoal {
		t.Fatalf("stdin goal: rc=%d goal=%q", rc, goal)
	}
}

// A conflict is an ERROR, never a silent precedence rule — when a caller passes both,
// which one they meant is genuinely unknown, and guessing dispatches the wrong task.
func TestSpawnGoal_FileAndPositionalConflict(t *testing.T) {
	if _, rc := spawnGoal(goalFileWith(t, "from the file"), []string{"from", "argv"}, nil); rc == 0 {
		t.Fatal("--goal-file plus a positional goal must be refused")
	}
}

func TestSpawnGoal_PositionalStillWorks(t *testing.T) {
	goal, rc := spawnGoal("", []string{"fix", "the", "auth", "middleware"}, nil)
	if rc != 0 || goal != "fix the auth middleware" {
		t.Fatalf("positional goal: rc=%d goal=%q", rc, goal)
	}
	if _, rc := spawnGoal("", nil, nil); rc == 0 {
		t.Fatal("no goal at all must be a usage error")
	}
}

func TestSpawnGoal_EmptyAndMissingFile(t *testing.T) {
	if _, rc := spawnGoal(goalFileWith(t, "  \n\t\n"), nil, nil); rc == 0 {
		t.Error("a whitespace-only goal file must be refused")
	}
	if _, rc := spawnGoal(filepath.Join(t.TempDir(), "nope.txt"), nil, nil); rc == 0 {
		t.Error("a missing goal file must be refused")
	}
}

// The one-shot path used to shell-quote the goal onto the typed launch line AND collapse
// its whitespace, so a multi-line goal silently arrived as one line. It now stages to a
// file; the runner reads it back byte-exact.
func TestOneshotGoalStaging_RoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path, err := stageGoalFile(nastyGoal)
	if err != nil {
		t.Fatalf("stageGoalFile: %v", err)
	}
	got, err := readGoalFile(path)
	if err != nil {
		t.Fatalf("readGoalFile: %v", err)
	}
	if got != nastyGoal {
		t.Fatalf("one-shot goal mutated\n got: %q\nwant: %q", got, nastyGoal)
	}
	// The runner is the only consumer — a goal left on disk is a stale copy of an
	// instruction.
	if _, err := os.Stat(path); err == nil {
		t.Error("the staged goal file should be removed once read")
	}

	// A short single-line goal keeps the readable inline form.
	if _, err := stageGoalFile("run the tests"); err == nil {
		t.Error("a short single-line goal should stay inline, not be staged")
	}
	// A long single-line goal is staged (an unreadable command line helps nobody).
	long := strings.Repeat("走一遍回归 ", 60)
	if p, err := stageGoalFile(long); err != nil {
		t.Errorf("a long goal should be staged: %v", err)
	} else {
		_ = os.Remove(p)
	}
}

// The trailing-newline contract is shared by every entry point, so pin it here too:
// a heredoc's final newline is dropped, and nothing else is.
func TestGoalFile_TrailingNewlineContract(t *testing.T) {
	if got := dispatch.TrimPayload("a\nb\n"); got != "a\nb" {
		t.Errorf("TrimPayload = %q, want %q", got, "a\nb")
	}
	if got := dispatch.TrimPayload("a\n\n"); got != "a\n" {
		t.Errorf("only ONE trailing newline may be stripped, got %q", got)
	}
}
