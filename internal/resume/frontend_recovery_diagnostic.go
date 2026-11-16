package resume

import "syscall"

// FrontendRecoveryKind describes producer evidence, never authority to bind,
// resend input or start a session. A live process alone does not prove retries.
type FrontendRecoveryKind string

const (
	FrontendRecoveryUnknown     FrontendRecoveryKind = "unknown"
	FrontendRecoveryStopped     FrontendRecoveryKind = "stopped"
	FrontendRecoveryUnconfirmed FrontendRecoveryKind = "alive-unconfirmed"
)

var frontendRecoveryGone = func(pid int) bool {
	return pid > 0 && syscall.Kill(pid, 0) == syscall.ESRCH
}

func LoadFrontendRecoveryKind(loc string) FrontendRecoveryKind {
	r, ok := Load(loc)
	if !ok || r.Agent != "codex" || r.Frontend == "" {
		return FrontendRecoveryUnknown
	}
	f, ok := readFrontend(r.Frontend)
	if !ok || f.Loc != loc || f.HomeID != r.HomeID || f.SessionID != r.SessionID {
		return FrontendRecoveryUnknown
	}
	p, ok := frontendProcess(f.BridgePID)
	if !ok {
		if frontendRecoveryGone(f.BridgePID) {
			return FrontendRecoveryStopped
		}
		return FrontendRecoveryUnknown // Timeout or unreadable metadata is not an exit.
	}
	if p.PID != f.BridgePID || p.Start != f.BridgeStart {
		return FrontendRecoveryStopped
	}
	return FrontendRecoveryUnconfirmed
}
