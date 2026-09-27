package hook

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

func TestDedicatedSemanticsMatchRegistry(t *testing.T) {
	for _, key := range agents.DedicatedSemanticsKeys() {
		if _, ok := agentEventSemantics[key]; !ok {
			t.Errorf("agent %q declares dedicated event semantics but has no classifier table", key)
		}
	}
	for key := range agentEventSemantics {
		manifest, ok := agents.For(key)
		if !ok || !manifest.Semantics {
			t.Errorf("classifier table %q is not declared as dedicated semantics in the registry", key)
		}
	}
}
