package native

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

func TestExactProcessSnapshotLivenessAndGrace(t *testing.T) {
	old := readAgentProcess
	defer func() { readAgentProcess = old }()
	for _, kind := range []string{"alive", "birth changed", "command changed", "unreadable", "unreadable expired"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			now := time.Now().Unix()
			r := Record{SessionID: "snapshot", Agent: "codex", PID: os.Getpid(), Comm: "codex", ProcessStart: "born", State: "idle", UpdatedAt: now}
			readAgentProcess = func(pid int) (launchhost.AgentProcess, bool) {
				p := launchhost.AgentProcess{PID: pid, Comm: "codex", Start: "born"}
				switch kind {
				case "birth changed":
					p.Start = "another birth"
				case "command changed":
					p.Comm = "unrelated"
				case "unreadable", "unreadable expired":
					return p, false
				}
				return p, true
			}
			if err := Save(r); err != nil {
				t.Fatal(err)
			}
			at := now
			if kind == "alive" || kind == "unreadable expired" {
				at += int64(StaleAfter/time.Second) + 1
			}
			got := Live(at)
			want := kind == "alive" || kind == "unreadable"
			if (len(got) == 1) != want {
				t.Fatalf("rows=%+v want visible=%v", got, want)
			}
		})
	}
}

func TestExactSnapshotRealProcessExit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	p, ok := launchhost.ReadAgent(cmd.Process.Pid)
	if !ok {
		t.Fatal("could not inspect isolated child")
	}
	now := time.Now().Unix()
	if err := Save(Record{Agent: "codex", SessionID: "owned-child", PID: p.PID, Comm: p.Comm, ProcessStart: p.Start, State: "idle", UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if rows := Live(now); len(rows) != 1 {
		t.Fatal("live exact process missing", rows)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if rows := Live(now); len(rows) != 0 {
		t.Fatal("exited exact process remained", rows)
	}
}
