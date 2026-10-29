package hook

import (
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"strings"
	"testing"
	"time"
)

func TestCodexSubagentHookCannotEndParentOrChangeItsProfile(t *testing.T) {
	hermeticEnv(t)
	if err := state.WriteMarker(state.ActivePath("%27"), "parent-root"); err != nil {
		t.Fatal(err)
	}
	raw := `{"session_id":"parent-root","agent_id":"child-thread","cwd":"/child","hook_event_name":"Stop","model":"child-model"}`
	Run(strings.NewReader(raw), []string{"--agent", "codex", "--detached", "Stop"})
	if got := state.ReadMarker(state.ActivePath("%27")); got != "parent-root" {
		t.Fatal("child ended parent", got)
	}
	if _, ok := native.Load("parent-root", transcript.CodexHomeID()); ok {
		t.Fatal("child created parent native observation")
	}
}

func TestCodexFrontendHookUsesRootInsteadOfShellCwdAndInheritedPane(t *testing.T) {
	hermeticEnv(t)
	old := codexHookFrontend
	t.Cleanup(func() { codexHookFrontend = old })
	f := resume.CodexFrontend{BackendPID: 200, BackendStart: "shared-birth"}
	f.Pane = "%27"
	codexHookFrontend = func(home, sid string) (resume.CodexFrontend, bool) {
		return f, home == transcript.CodexHomeID() && sid == "root"
	}
	oldProcess := bindCodexHookProcess
	t.Cleanup(func() { bindCodexHookProcess = oldProcess })
	bindCodexHookProcess = func(string, string, string, launchhost.AgentProcess, int64) string {
		t.Fatal("verified frontend fell through to cwd guessing")
		return ""
	}
	b, _ := json.Marshal(map[string]any{"session_id": "root", "cwd": "/saved/other-workspace", "prompt": "synthetic root prompt"})
	// Use the same captured, detached source identity as the real hook worker.
	t.Setenv(agentProcessEnv, launchhost.EncodeAgent(launchhost.AgentProcess{PID: 200, Comm: "codex", Start: "shared-birth", ObservedAt: time.Now().Unix()}))
	Run(strings.NewReader(string(b)), []string{"--agent", "codex", "--detached", "UserPromptSubmit"})
	if got := state.ReadMarker(state.ActivePath("%27")); got != "root" {
		t.Fatal("root hook lost its verified pane", got)
	}
}
