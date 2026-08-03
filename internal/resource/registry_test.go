package resource

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// isAgentProcess now scans the registry's resource names. Every attributed agent
// must be recognized; the bare-semver rule still catches version-named processes.

func TestEveryRegistryResourceNameIsAgentProcess(t *testing.T) {
	names := agents.ResourceNames()
	if len(names) == 0 {
		t.Fatal("registry ResourceNames is empty")
	}
	for _, n := range names {
		if !isAgentProcess(n) {
			t.Errorf("registry attributes %q but isAgentProcess(%q)=false", n, n)
		}
	}
}

func TestVersionNamedProcessStillCounts(t *testing.T) {
	if !isAgentProcess("2.1.220") {
		t.Error("a bare-semver command (Claude reports its version) must count as an agent process")
	}
	if isAgentProcess("vim") {
		t.Error("vim is not an agent process")
	}
}
