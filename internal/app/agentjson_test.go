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

	// The supervisor (中控) session carries role:"supervisor"; normal rows omit it.
	sb, _ := json.Marshal(agentJSON{PaneID: "%9", Status: "idle", Source: "tmux", Role: "supervisor"})
	if !strings.Contains(string(sb), `"role":"supervisor"`) {
		t.Errorf("supervisor row missing role field: %s", sb)
	}
	if strings.Contains(string(b), `"role"`) {
		t.Errorf("normal agent JSON should omit role: %s", b)
	}

	// An idle row with background work still running carries the bg modifier.
	gb, _ := json.Marshal(agentJSON{
		PaneID: "%6", Status: "idle", Source: "tmux",
		Bg: true, BgCount: 2, BgText: "npm run dev",
	})
	for _, key := range []string{`"bg":true`, `"bg_count":2`, `"bg_text":"npm run dev"`} {
		if !strings.Contains(string(gb), key) {
			t.Errorf("bg agents --json missing %s in %s", key, gb)
		}
	}
	// A plain idle row omits the bg fields (omitempty), so a bg-unaware consumer is
	// unaffected.
	if strings.Contains(string(b), `"bg"`) || strings.Contains(string(b), `"bg_count"`) {
		t.Errorf("non-bg agent JSON should omit the bg fields: %s", b)
	}

	// A pane scrolled into tmux copy/view-mode (input-locked) carries in_mode:true.
	mb, _ := json.Marshal(agentJSON{PaneID: "%7", Status: "waiting", Source: "tmux", InMode: true})
	if !strings.Contains(string(mb), `"in_mode":true`) {
		t.Errorf("input-locked agents --json missing in_mode field: %s", mb)
	}
	// A pane not in a mode omits in_mode (omitempty).
	if strings.Contains(string(b), `"in_mode"`) {
		t.Errorf("non-locked agent JSON should omit in_mode: %s", b)
	}
}
