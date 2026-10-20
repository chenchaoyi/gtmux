package hook

import (
	"github.com/chenchaoyi/gtmux/internal/agentprofile"
	"strings"
	"testing"
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
