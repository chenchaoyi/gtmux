package radar

import (
	"encoding/json"
	"strings"
	"testing"
)

// paneRowLine builds a panesSource field-line: id, session, window, pane, cwd,
// command, title, active, in_mode.
func paneRowLine(id, session, win, pane, cwd, cmd, title, active, inMode string) string {
	return strings.Join([]string{id, session, win, pane, cwd, cmd, title, active, inMode}, "\t")
}

func withPaneFixture(t *testing.T, lines []string, agents map[string]string, fn func()) {
	t.Helper()
	origPanes, origAgents := panesSource, agentPaneSet
	panesSource = func() []string { return lines }
	agentPaneSet = func() (map[string]string, map[string]bool, map[string]string) {
		set := map[string]bool{}
		icons := map[string]string{}
		for id, name := range agents {
			set[id] = true
			icons[id] = "/Applications/" + name + ".app" // stub official-icon hint
		}
		return agents, set, icons
	}
	defer func() { panesSource, agentPaneSet = origPanes, origAgents }()
	fn()
}

func TestGatherPanes_Tiers(t *testing.T) {
	lines := []string{
		paneRowLine("%1", "MP", "0", "0", "/p/mp", "claude", "✳ build the thing", "1", "0"),
		paneRowLine("%2", "MP", "0", "1", "/p/mp", "bash", "", "0", "0"),
		paneRowLine("%3", "MP", "1", "0", "/p/mp", "vim", "editing", "1", "1"),
	}
	// The radar classifies %1 as an agent (Claude Code); %2 and %3 are plain.
	agents := map[string]string{"%1": "Claude Code"}

	var rows []PaneRow
	withPaneFixture(t, lines, agents, func() { rows = GatherPanes() })

	if len(rows) != 3 {
		t.Fatalf("want 3 panes, got %d", len(rows))
	}
	byID := map[string]PaneRow{}
	for _, r := range rows {
		byID[r.PaneID] = r
	}
	if byID["%1"].Tier != "agent" || byID["%1"].Agent != "Claude Code" {
		t.Fatalf("%%1 should be an agent pane: %+v", byID["%1"])
	}
	// An agent pane carries the official-icon hint (so the pane browser shows the real
	// logo, not the monogram); a plain pane never does.
	if byID["%1"].Icon == "" {
		t.Fatalf("%%1 (agent) should carry an icon hint: %+v", byID["%1"])
	}
	if byID["%2"].Tier != "plain" || byID["%2"].Agent != "" || byID["%2"].Icon != "" {
		t.Fatalf("%%2 (bare shell) should be plain with no icon: %+v", byID["%2"])
	}
	if byID["%3"].Tier != "plain" {
		t.Fatalf("%%3 (vim) is a plain pane, not an agent: %+v", byID["%3"])
	}
	// Field fidelity: loc, cwd, active, in_mode, title.
	if byID["%1"].Loc != "MP:0.0" || byID["%1"].Cwd != "/p/mp" || !byID["%1"].Active {
		t.Fatalf("%%1 fields wrong: %+v", byID["%1"])
	}
	if !byID["%3"].InMode {
		t.Fatalf("%%3 is in copy-mode; in_mode should be true: %+v", byID["%3"])
	}
}

func TestPanesJSONBytes_AlwaysArray(t *testing.T) {
	withPaneFixture(t, nil, nil, func() {
		b, err := PanesJSONBytes()
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "[]" {
			t.Fatalf("empty pane set → [], got %q", b)
		}
		var rows []PaneRow
		if json.Unmarshal(b, &rows) != nil {
			t.Fatal("output must decode as a JSON array")
		}
	})
}
