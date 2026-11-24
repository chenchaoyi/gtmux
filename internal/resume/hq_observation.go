package resume

import (
	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// MatchesLegacyHQClient rechecks the exact incarnation that supplied the witness.
func MatchesLegacyHQClient(pane string, rec Record) bool {
	w := rec.HQObservation
	switch rec.Agent {
	case "codex":
		return VerifyDirectCodexCaller(pane, rec.HomeID, rec.SessionID, w.ClientPID, w.ClientBirth)
	case "claude":
		if w.ClientTTY == "" || w.Generation == "" || tmux.PaneGeneration(pane) != w.Generation {
			return false
		}
		p, ok := codexterminal.ForegroundClaude(pane, 0)
		return ok && p.PID == w.ClientPID && p.Start == w.ClientBirth && p.TTY == w.ClientTTY &&
			tmux.PaneGeneration(pane) == w.Generation
	default:
		return false
	}
}
