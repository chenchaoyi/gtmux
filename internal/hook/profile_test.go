package hook

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agentprofile"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

func TestProfileObserverRetainsMetadataWithoutLifecycle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GEMINI_CLI_HOME", "")
	raw := `{"session_id":"profile-only","llm_request":{"model":"gemini-example","config":{"thinkingConfig":{"thinkingBudget":128}}}}`
	if !EventIsMapped("gemini", "Profile") || classify("gemini", "Profile", "").Lifecycle != "" {
		t.Fatal("observer must be handled without lifecycle authority")
	}
	if Run(strings.NewReader(raw), []string{"--agent", "gemini", "Profile"}) != 0 {
		t.Fatal("observer failed")
	}
	p := agentprofile.New("gemini", "profile-only", false)
	agentprofile.Current(&p, "")
	found := false
	for _, f := range p.Facts {
		if f.Key == "model" && f.Value == "gemini-example" {
			found = true
		}
	}
	if !found {
		t.Fatal("registered observer discarded metadata", p)
	}
}

func TestAgyPreDetachLifecycleCapturesLaunchHostAndProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGY_HOME", "")
	t.Setenv("AGY_CONFIG_DIR", "")
	t.Setenv("TMUX_PANE", "")

	origStdout := agyHookStdout
	origStart := startHookWorker
	origLaunch := observeLaunchHost
	defer func() {
		agyHookStdout = origStdout
		startHookWorker = origStart
		observeLaunchHost = origLaunch
	}()

	var buf bytes.Buffer
	agyHookStdout = &buf
	launchObserved := 0
	observeLaunchHost = func(commands []string) launchhost.Evidence {
		launchObserved++
		return launchhost.Evidence{Kind: "terminal", Name: "Ghostty"}
	}
	// Simulate detach succeeding so only pre-detach observation runs.
	startHookWorker = func(_ []string, _ []byte, origin launchhost.Evidence, _ launchhost.AgentProcess) bool {
		if origin.Name != "Ghostty" {
			t.Errorf("pre-detach PreInvocation lost launch host: %+v", origin)
		}
		return true
	}

	raw := `{"conversationId":"agy-chain-1","cwd":"/tmp/work","modelName":"gemini-4-carbon-medium","invocationNum":1,"hookEventName":"PreInvocation","lastUserInput":"hello"}`
	if rc := Run(strings.NewReader(raw), []string{"--agent", "agy", "PreInvocation"}); rc != 0 {
		t.Fatalf("PreInvocation rc = %d", rc)
	}
	if launchObserved != 1 {
		t.Fatalf("observeLaunchHost calls = %d, want 1", launchObserved)
	}
	p := agentprofile.New("agy", "agy-chain-1", false)
	agentprofile.Current(&p, "")
	if len(p.Chain) == 0 || p.ChainAt == 0 {
		t.Fatalf("PreInvocation failed to capture pre-detach execution chain: %+v", p)
	}
}
