package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// The agents --json contract (DESIGN §14) must carry the terminal-generalization
// fields so the menu-bar app can render tmux vs native agents and relative time.
func TestAgentJSONContractFields(t *testing.T) {
	b, err := json.Marshal(agentJSON{
		PaneID: "%5", Session: "work", Status: "waiting",
		Source: "tmux", ActivityAt: 1700000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"pane_id"`, `"session"`, `"status"`, `"source"`, `"activity_at"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("agents --json missing contract key %s in %s", key, b)
		}
	}

	// A native agent carries project/terminal/tab.
	nb, _ := json.Marshal(agentJSON{
		Source: "native", Project: "diting", Terminal: "Ghostty", Tab: "diting — zsh",
		Agent: "Gemini", Status: "idle",
	})
	for _, key := range []string{`"source":"native"`, `"project":"diting"`, `"terminal":"Ghostty"`, `"tab":"diting — zsh"`} {
		if !strings.Contains(string(nb), key) {
			t.Errorf("native agents --json missing %s in %s", key, nb)
		}
	}

	// tmux agents omit the native-only fields (omitempty).
	if strings.Contains(string(b), `"project"`) || strings.Contains(string(b), `"terminal"`) {
		t.Errorf("tmux agent JSON should omit native-only fields: %s", b)
	}
}
