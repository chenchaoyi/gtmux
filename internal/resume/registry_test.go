package resume

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// resumeArgv is now sourced from the agent registry. Every resumable agent the
// registry declares must be Resumable here, and a non-registry agent must not be.

func TestEveryRegistryResumableIsResumable(t *testing.T) {
	want := agents.ResumeArgv()
	if len(want) == 0 {
		t.Fatal("registry ResumeArgv is empty")
	}
	for key := range want {
		if !Resumable(key) {
			t.Errorf("registry declares %q resumable but Resumable(%q)=false", key, key)
		}
	}
	if Resumable("aider") {
		t.Error("aider has no resume argv in the registry; want not resumable")
	}
}
