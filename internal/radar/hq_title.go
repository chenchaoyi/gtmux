package radar

import (
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// The shared task label follows Codex's saved name on the current HQ binding,
// including subsequent user renames. A stale location record is insufficient.
func applyHQThreadTitles(panes []Pane, frontend func(string) (resume.CodexFrontend, bool)) {
	home := transcript.CodexHomeID()
	var titles map[string]string
	for i := range panes {
		p := &panes[i]
		if p.source != "tmux" || p.role != "supervisor" || agents.KeyForLabel(p.Agent) != "codex" {
			continue
		}
		a, ok := frontend(p.PaneID)
		if !ok || a.HomeID != home || a.Pane != p.PaneID {
			continue
		}
		if titles == nil {
			titles = transcript.SessionTitles("codex")
		}
		if name := titles[a.SessionID]; name != "" {
			p.Task = name
		}
	}
}
