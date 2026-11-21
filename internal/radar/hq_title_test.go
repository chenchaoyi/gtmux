package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestHQAgentTaskUsesCurrentCodexName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	codexDir := t.TempDir()
	t.Setenv("CODEX_HOME", codexDir)
	homeID := transcript.CodexHomeID()
	panes := []Pane{
		{PaneID: "%1", Agent: "Codex", Task: "screen fallback", source: "tmux", cwd: state.HQHome(), role: "supervisor"},
		{PaneID: "%2", Agent: "Codex", Task: "worker task", source: "tmux"},
	}
	hqHome := state.HQHome()
	for _, tc := range []struct{ label, stamp, home, id, name, want string }{
		{"saved", hqHome, homeID, "hq-thread", "HQ notes", "HQ notes"},
		{"rename", hqHome, homeID, "hq-thread", "My HQ notes", "My HQ notes"},
		{"ordinary pane in HQ directory", "", homeID, "hq-thread", "Worker title", "screen fallback"},
		{"foreign stamp", t.TempDir(), homeID, "hq-thread", "Worker title", "screen fallback"},
		{"wrong home", hqHome, "foreign", "hq-thread", "Same ID in another home", "screen fallback"},
		{"wrong thread", hqHome, homeID, "another-root", "Stale label", "screen fallback"},
		{"no live connection", hqHome, homeID, "hq-thread", "Stale label", "screen fallback"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			a := resume.CodexFrontend{HomeID: tc.home, SessionID: tc.id, CodexBindingTarget: resume.CodexBindingTarget{Pane: "%1"}}
			frontend := func(pane string) (resume.CodexFrontend, bool) {
				if pane != "%1" {
					t.Fatalf("ordinary pane queried for HQ title: %s", pane)
				}
				return a, tc.label != "no live connection"
			}
			data, err := json.Marshal(map[string]string{"id": "hq-thread", "thread_name": tc.name})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(codexDir, "session_index.jsonl"), data, 0600); err != nil {
				t.Fatal(err)
			}
			rows := append([]Pane(nil), panes...)
			stamps := map[string]string{}
			if tc.stamp != "" {
				stamps["%1"] = tc.stamp
			}
			applyRolePrecedence(rows, stamps)
			applyHQThreadTitles(rows, stamps, frontend)
			data, err = agentsJSONBytes(rows)
			if err != nil {
				t.Fatal(err)
			}
			var payload []agentJSON
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 2 || payload[0].Role != "supervisor" || payload[0].Task != tc.want || payload[1].Task != panes[1].Task {
				t.Fatalf("unexpected shared payload: %s", data)
			}
		})
	}
}
