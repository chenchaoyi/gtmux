package native

import "github.com/chenchaoyi/gtmux/internal/codexterminal"

var managedCodexBackend = codexterminal.ManagedBackend

// SharedCodexBackend is positive evidence that this observation's exact process
// incarnation is the shared backend. It does not establish a living frontend,
// controller, input route or turn. Missing or changed process evidence is unknown.
func SharedCodexBackend(r Record) bool {
	if r.Agent != "codex" || r.PID <= 1 || r.Comm != "codex" || r.ProcessStart == "" {
		return false
	}
	matches := func() bool {
		p, ok := readAgentProcess(r.PID)
		return ok && p.PID == r.PID && p.Comm == r.Comm && p.Start == r.ProcessStart
	}
	return matches() && managedCodexBackend(r.PID) && matches()
}
