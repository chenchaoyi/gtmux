package native

import (
	"syscall"
	"time"
)

var probeClosedFrontend = func(pid int) error { return syscall.Kill(pid, 0) }

// ClosedFrontendObservation proves that this exact, unchanged observation was
// present at a verified frontend binding whose client has since exited. It is
// not a turn-completion fact or authority to remove history, adopt, or send.
// Unknown liveness and any newer observation/binding defeat the proof.
func ClosedFrontendObservation(r Record) bool {
	if r.Agent != "codex" || r.HomeID == "" || r.Revision == "" || r.PID <= 1 || r.ProcessStart == "" ||
		r.Terminal != "" || r.LaunchHost != (Record{}).LaunchHost {
		return false
	}
	b, ok := BindingFor(r.SessionID, r.HomeID)
	if !ok || b.Kind != "frontend" || b.Observation != r || b.BackendPID != r.PID || b.BackendStart != r.ProcessStart ||
		b.Cwd != r.Cwd || b.PID <= 1 || b.PID == r.PID || b.BoundAt < r.UpdatedAt || b.BoundAt > time.Now().Unix()+1 {
		return false
	}
	if probeClosedFrontend(b.PID) != syscall.ESRCH {
		return false
	}
	current, ok := Load(r.SessionID, r.HomeID)
	if !ok || current != r {
		return false
	}
	latest, ok := BindingFor(r.SessionID, r.HomeID)
	return ok && latest == b
}
