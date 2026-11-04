package resume

import (
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func sameResumeOwner(a, b Record) bool {
	a.RecoverySessionID, a.RecoveryCwd, a.RecoveryAt = "", "", 0
	b.RecoverySessionID, b.RecoveryCwd, b.RecoveryAt = "", "", 0
	return a == b
}

// Live identity changes immediately; only verified durable history advances the
// separate checkpoint. A missing new thread must never silently resume its parent.
func withRecoveryCheckpoint(r, previous Record) Record {
	if r.Agent != "codex" {
		return r
	}
	if previous.Agent == r.Agent && previous.HomeID == r.HomeID && previous.Cwd == r.Cwd {
		r.RecoverySessionID, r.RecoveryCwd, r.RecoveryAt = previous.RecoverySessionID, previous.RecoveryCwd, previous.RecoveryAt
		if r.RecoverySessionID == "" && previous.SessionID != r.SessionID {
			if info := Recovery(previous); info.State == transcript.RecoveryReady {
				r.RecoverySessionID, r.RecoveryCwd, r.RecoveryAt = previous.SessionID, info.Cwd, time.Now().Unix()
			}
		}
	}
	if info := Recovery(r); info.State == transcript.RecoveryReady {
		r.RecoverySessionID, r.RecoveryCwd = r.SessionID, info.Cwd
		if r.RecoverySessionID != previous.RecoverySessionID || r.RecoveryAt == 0 {
			r.RecoveryAt = time.Now().Unix()
		}
	}
	return r
}
