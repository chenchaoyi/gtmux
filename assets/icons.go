// Package assets holds committed, EMBEDDED static assets shipped inside the gtmux
// binary. agent-icons/<key>.png are the per-agent identity icons — the vendors'
// OFFICIAL marks, committed for ONE purpose: to IDENTIFY which coding agent a pane
// is running (nominative use), never as gtmux's own branding. Provenance +
// attribution per file live in agent-icons/SOURCES.md. This is the deliberate
// exception to the "no third-party marks in the repo" rule — see CLAUDE.md §6.
//
// Embedding (not runtime-fetching) is what makes it out-of-box + stable: the icons
// ship with `gtmux update` and the serve hands them to every surface (mobile / web
// / menu-bar) from one source, no network, no per-machine setup.
package assets

import (
	"embed"
	"strings"
)

//go:embed agent-icons/*.png
var agentIconsFS embed.FS

// AgentIcon returns the committed PNG for an agent key ("opencode"/"gemini"/…), or
// nil when none is bundled (the caller then falls back to the installed-app icon or
// the neutral monogram).
func AgentIcon(key string) []byte {
	if key == "" {
		return nil
	}
	b, err := agentIconsFS.ReadFile("agent-icons/" + key + ".png")
	if err != nil {
		return nil
	}
	return b
}

// AgentIconKeys lists the keys that have a committed icon (for the install-time drop
// into ~/.config/gtmux/icons so the local menu-bar surface picks them up too).
func AgentIconKeys() []string {
	entries, err := agentIconsFS.ReadDir("agent-icons")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if n := e.Name(); strings.HasSuffix(n, ".png") {
			out = append(out, strings.TrimSuffix(n, ".png"))
		}
	}
	return out
}
