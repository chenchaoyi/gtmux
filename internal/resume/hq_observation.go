package resume

import (
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

// HQClaudeClient identifies an unwrapped Claude process that itself owns the pane.
// An unverified wrapper or detached host grants no client identity.
func HQClaudeClient(pane string) (launchhost.AgentProcess, bool) {
	pid, ok := codexterminal.ForegroundRoot(pane)
	if !ok {
		return launchhost.AgentProcess{}, false
	}
	p, ok := launchhost.ReadAgent(pid)
	return p, ok && p.PID == pid && p.Start != "" && (p.Comm == "claude" || agents.LooksLikeVersion(p.Comm))
}

// MatchesLegacyHQClient rechecks the exact incarnation that supplied the witness.
func MatchesLegacyHQClient(pane string, rec Record) bool {
	w := rec.HQObservation
	switch rec.Agent {
	case "codex":
		return VerifyDirectCodexCaller(pane, rec.HomeID, rec.SessionID, w.ClientPID, w.ClientBirth)
	case "claude":
		p, ok := HQClaudeClient(pane)
		return ok && p.PID == w.ClientPID && p.Start == w.ClientBirth
	default:
		return false
	}
}
