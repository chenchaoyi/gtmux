package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestReapKillFailureDoesNotRemoveLiveWorktree(t *testing.T) {
	spy := &spyOps{merged: true}
	ops := spy.ops()
	ops.killSession = func(string) error { return errors.New("connection failed") }
	res := planAndReap(worktreeTask(), false, false, ops)
	if res.ResourceClosed || len(res.Failed) != 1 || spy.removed || spy.branchGone || len(spy.order) != 0 {
		t.Fatal("kill failure reclaimed live resources", res, spy)
	}
}

func TestReapSettlementKeepsFailedTaskAndReportsResourceClosure(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "closed", true: "failed kill"}[failed], func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TMUX", "")
			t.Setenv("TMUX_TMPDIR", t.TempDir())
			task := dispatch.Task{ID: "task-a", Pane: "%7", Session: "worker", OwnSession: true, Agent: "codex"}
			if err := dispatch.AddTask(task); err != nil {
				t.Fatal(err)
			}
			stub := filepath.Join(t.TempDir(), "tmux")
			rc := "0"
			if failed {
				rc = "1"
			}
			// No real tmux invocation: display returns no live target; only this owned
			// synthetic kill command can succeed/fail. No text or control keys are sent.
			script := "#!/bin/sh\ncase \"$*\" in *kill-session*) exit " + rc + ";; *) exit 0;; esac\n"
			if err := os.WriteFile(stub, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			_ = exec.Command(stub).Run()
			old := tmux.Bin
			tmux.Bin = stub
			t.Cleanup(func() { tmux.Bin = old })
			got := cmdReap([]string{task.ID, "--json"})
			if (got == 0) == failed {
				t.Fatal("wrong exit", got)
			}
			_, retained := dispatch.LoadTask(task.ID)
			if retained != failed {
				t.Fatal("partial failure lost recovery ledger", retained)
			}
			recs, _ := events.ReadSince(0)
			if len(recs) != 1 || recs[0].ResourceClosed == nil || *recs[0].ResourceClosed == failed || recs[0].TaskID != task.ID {
				t.Fatal("wrong closure receipt", recs)
			}
			if recs[0].AgentSession != "" || recs[0].BindingID != "" {
				t.Fatal("unverified thread was claimed", recs[0])
			}
		})
	}
}

func TestReapBindingRejectsReplacedAttachment(t *testing.T) {
	b := native.Binding{SessionID: "thread-a", Pane: "%7", Loc: "worker:0.0", Cwd: "/repo", PID: 200, PanePID: 100}
	live := resume.CodexBindingTarget{Pane: b.Pane, Loc: b.Loc, Cwd: b.Cwd, PID: b.PID, PanePID: b.PanePID}
	if !reapBindingMatches(b, live) {
		t.Fatal("exact receipt not joined")
	}
	for _, field := range []string{"pid", "pane pid", "loc", "pane", "cwd"} {
		other := live
		switch field {
		case "pid":
			other.PID++
		case "pane pid":
			other.PanePID++
		case "loc":
			other.Loc = "replacement:0.0"
		case "pane":
			other.Pane = "%8"
		case "cwd":
			other.Cwd = "/another"
		}
		if reapBindingMatches(b, other) {
			t.Fatal("replaced attachment accepted", field)
		}
	}
}
