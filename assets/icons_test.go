package assets

import "testing"

// The committed icons must actually embed (a mis-typed //go:embed or a missing file
// would ship an empty icon silently). opencode + gemini are the detected agents that
// have a committed icon today.
func TestAgentIconEmbedded(t *testing.T) {
	for _, k := range []string{"opencode", "gemini"} {
		if b := AgentIcon(k); len(b) == 0 {
			t.Errorf("committed icon for %q is missing or empty (check assets/agent-icons/%s.png + the //go:embed)", k, k)
		}
	}
	if AgentIcon("nope") != nil {
		t.Error("AgentIcon(nonexistent) must be nil so the caller falls back")
	}
	if AgentIcon("") != nil {
		t.Error("AgentIcon(\"\") must be nil")
	}
	if len(AgentIconKeys()) < 2 {
		t.Errorf("expected several committed icon keys, got %v", AgentIconKeys())
	}
}
