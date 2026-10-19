package hook

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestAgentProcessCapturedBeforeDetachAndNeverGuessedByWorker(t *testing.T) {
	hermeticEnv(t)
	oldObserve, oldStart := observeAgentProcess, startHookWorker
	defer func() { observeAgentProcess, startHookWorker = oldObserve, oldStart }()
	expected := launchhost.AgentProcess{PID: 123, Comm: "codex", Start: "born", ObservedAt: time.Now().Unix()}
	var calls []string
	observeAgentProcess = func(commands []string) launchhost.AgentProcess {
		calls = append(calls, "capture")
		if !slices.Contains(commands, "codex") {
			t.Fatal(commands)
		}
		return expected
	}
	startHookWorker = func(args []string, payload []byte, host launchhost.Evidence, process launchhost.AgentProcess) bool {
		calls = append(calls, "detach")
		if process != expected {
			t.Fatal("lost pre-detach identity", process)
		}
		return true
	}
	raw := `{"session_id":"process-thread","cwd":"/test"}`
	if Run(strings.NewReader(raw), []string{"--agent", "codex", "Stop"}) != 0 || !slices.Equal(calls, []string{"capture", "detach"}) {
		t.Fatal(calls)
	}
	// Every detached lifecycle event must use the handed-off agent, never its own
	// reparented ancestry. Neither a launch-host name nor a real frontend is needed.
	calls = nil
	for _, kind := range []string{"valid", "missing", "expired"} {
		t.Run(kind, func(t *testing.T) {
			p := expected
			if kind == "expired" {
				p.ObservedAt -= 61
			}
			encoded := launchhost.EncodeAgent(p)
			if kind == "missing" {
				encoded = ""
			}
			t.Setenv(agentProcessEnv, encoded)
			if Run(strings.NewReader(raw), []string{"--agent", "codex", "--detached", "Stop"}) != 0 {
				t.Fatal("hook failed")
			}
			r, ok := native.Load("process-thread", transcript.CodexHomeID())
			if !ok {
				t.Fatal("native record missing")
			}
			if kind == "valid" {
				if r.PID != p.PID || r.Comm != p.Comm || r.ProcessStart != p.Start {
					t.Fatal(r)
				}
			} else if r.PID != 0 || r.Comm != "" || r.ProcessStart != "" {
				t.Fatal("invented worker identity", r)
			}
		})
	}
	if len(calls) != 0 {
		t.Fatal("detached worker probed its own ancestry", calls)
	}
}
