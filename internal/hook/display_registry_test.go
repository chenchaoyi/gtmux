package hook

import (
	"reflect"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// agentDisplay (the hook-time known-agent gate) is now sourced from the agent
// registry. This pins the derivation so a new agent's display name flows here.

func TestAgentDisplayDerivedFromRegistry(t *testing.T) {
	if !reflect.DeepEqual(agentDisplay, agents.DisplayNames()) {
		t.Fatalf("agentDisplay=%v, registry DisplayNames=%v", agentDisplay, agents.DisplayNames())
	}
	if agentDisplay["opencode"] != "opencode" {
		t.Errorf("opencode display = %q, want opencode", agentDisplay["opencode"])
	}
	if agentDisplay["hermes-agent"] != "Hermes" {
		t.Errorf("hermes-agent display = %q, want Hermes", agentDisplay["hermes-agent"])
	}
}
