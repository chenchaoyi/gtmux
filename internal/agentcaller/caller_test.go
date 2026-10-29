package agentcaller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func callerFixture(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_ACTOR", "")
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("TMUX_PANE", "%16")
	t.Setenv("CODEX_THREAD_ID", "worker-thread")
	t.Setenv("CODEX_SESSION_ID", "worker-thread")
	oldPanes, oldDriver, oldHQ, oldProcess := callerPanes, callerPaneDriver, callerHQPane, callerProcess
	t.Cleanup(func() {
		callerPanes, callerPaneDriver, callerHQPane, callerProcess = oldPanes, oldDriver, oldHQ, oldProcess
	})
	callerPanes = func() []string {
		return []string{"%16\tother:0.0\tother\t100", "%17\tworker:0.0\tworker\t101", "%18\thq:0.0\thq\t102"}
	}
	callerPaneDriver = func(string) string { return "codex" }
	callerHQPane = func() string { return "%18" }
	callerProcess = func() launchhost.AgentProcess {
		return launchhost.AgentProcess{PID: 900, Comm: "codex", Start: "daemon-start"}
	}
	d := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "29")
	if err := os.MkdirAll(d, 0700); err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{"worker-thread", "hq-thread", "unbound-thread", "child-thread", "desktop-thread"} {
		source := any("vscode")
		origin := "codex-tui"
		if sid == "child-thread" {
			source = map[string]any{"subagent": map[string]string{"parent_thread_id": "hq-thread"}}
		}
		if sid == "desktop-thread" {
			origin = "Codex Desktop"
		}
		b, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]any{"id": sid, "originator": origin, "source": source}})
		if err := os.WriteFile(filepath.Join(d, "rollout-2026-10-29T00-00-00-"+sid+".jsonl"), append(b, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for loc, sid := range map[string]string{"worker:0.0": "worker-thread", "hq:0.0": "hq-thread"} {
		if err := resume.Save(loc, resume.Record{Agent: "codex", HomeID: transcript.CodexHomeID(), SessionID: sid}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCallerUsesExecutingThreadInsteadOfInheritedPane(t *testing.T) {
	callerFixture(t)
	c, ok := Current()
	if !ok || c.Pane != "%17" || c.ThreadID != "worker-thread" || c.Session != "worker" {
		t.Fatal("inherited pane became source", c, ok)
	}
	if got := DiagnosticActor(); got != "agent:%17" {
		t.Fatal("audit actor inherited another pane", got)
	}
	t.Setenv("TMUX_PANE", "%18")
	if IsHQ() {
		t.Fatal("worker acquired HQ from inherited environment")
	}
	t.Setenv("CODEX_THREAD_ID", "hq-thread")
	t.Setenv("CODEX_SESSION_ID", "hq-thread")
	if !IsHQ() {
		t.Fatal("current HQ thread was rejected")
	}
}

func TestUnboundAndSubagentCallersNeverBorrowHQPane(t *testing.T) {
	for _, sid := range []string{"unbound-thread", "child-thread"} {
		t.Run(sid, func(t *testing.T) {
			callerFixture(t)
			t.Setenv("CODEX_THREAD_ID", sid)
			t.Setenv("TMUX_PANE", "%18")
			c, ok := Current()
			if !ok || c.ThreadID != sid || c.Pane != "" || c.Session == "" || IsHQ() {
				t.Fatal("unbound source borrowed a pane", c, ok)
			}
		})
	}
}

func TestInvalidRuntimeIdentityDoesNotFallBackToPane(t *testing.T) {
	callerFixture(t)
	t.Setenv("CODEX_THREAD_ID", "desktop-thread")
	t.Setenv("TMUX_PANE", "%18")
	if _, ok := Current(); ok {
		t.Fatal("Desktop metadata became terminal authority")
	}
	if IsHQ() {
		t.Fatal("invalid runtime acquired HQ")
	}
}

func TestOtherAgentDoesNotInheritParentCodexThread(t *testing.T) {
	callerFixture(t)
	callerProcess = func() launchhost.AgentProcess {
		return launchhost.AgentProcess{PID: 901, Comm: "claude", Start: "child"}
	}
	c, ok := Current()
	if !ok || c.ThreadID != "" || c.Pane != "%16" {
		t.Fatal("child agent was attributed to parent Codex", c)
	}
}
