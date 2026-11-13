package resume

import (
	"strconv"
	"time"
)

// FrontendBindingError is value-free diagnostic evidence, never authority.
// Missing observations may be checked again; an observed different owner cannot.
type FrontendBindingError struct {
	Code      string
	Retryable bool
}

func (e *FrontendBindingError) Error() string { return "frontend binding: " + e.Code }
func frontendProof(f CodexFrontend, strict bool) error {
	fail := func(code string, retry bool) error { return &FrontendBindingError{code, retry} }
	age := time.Now().UnixMilli() - f.UpdatedAt
	if age < -1000 || age > 5000 {
		return fail("proof-expired", true)
	}
	p, ok := frontendProcess(f.BridgePID)
	if !ok {
		return fail("bridge-proof-unavailable", true)
	}
	if p.PID != f.BridgePID || p.Start != f.BridgeStart {
		return fail("bridge-changed", false)
	}
	if strict {
		backend, ok := frontendProcess(f.BackendPID)
		if !ok {
			return fail("backend-proof-unavailable", true)
		}
		if backend.PID != f.BackendPID || backend.Start != f.BackendStart {
			return fail("backend-changed", false)
		}
		target, ok := frontendTarget(f.Pane)
		if !ok {
			return fail("terminal-proof-unavailable", true)
		}
		if target != f.CodexBindingTarget {
			return fail("terminal-changed", false)
		}
		return nil
	}
	if frontendPaneView(f.Pane, "#{pane_id}\t#{pane_pid}\t#{session_name}:#{window_index}.#{pane_index}") != f.Pane+"\t"+strconv.Itoa(f.PanePID)+"\t"+f.Loc {
		return fail("pane-changed", false)
	}
	return nil
}
