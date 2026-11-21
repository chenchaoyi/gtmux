package radar

import (
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// The shared task label follows Codex's saved name on the current stamped HQ
// binding. The inferred supervisor role also covers legacy wake routing.
func applyHQThreadTitles(panes []Pane, stamps map[string]string, frontend func(string) (resume.CodexFrontend, bool)) {
	hqHome, codexHome := state.HQHome(), transcript.CodexHomeID()
	var titles map[string]string
	for i := range panes {
		p := &panes[i]
		if p.source != "tmux" || agents.KeyForLabel(p.Agent) != "codex" || !hqpane.SameDir(stamps[p.PaneID], hqHome) {
			continue
		}
		a, ok := frontend(p.PaneID)
		if !ok || a.HomeID != codexHome || a.Pane != p.PaneID {
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
