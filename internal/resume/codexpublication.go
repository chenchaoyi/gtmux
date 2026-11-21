package resume

import (
	"errors"
	"os"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// All location writers share the installed bridge's existing kernel lease.
func tryResumePublication() (*state.Lease, error) {
	const interval = 10 * time.Millisecond
	const attempts = 5
	for attempt := 1; ; attempt++ {
		lease, err := state.TryLease("codex-frontend-binding")
		if !errors.Is(err, state.ErrLeaseHeld) || attempt >= attempts {
			return lease, err
		}
		time.Sleep(interval)
	}
}

// lockCodexPublication joins Save/Bind/Refresh's kernel-owned publication lease.
// Preparation may read history or processes without holding this global lease;
// every index field must still match before any prepared result is published.
// The caller retains the lease through publication and retirement side effects.
func lockCodexPublication(loc string, expected, proposed Record) (*state.Lease, error) {
	lease, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		return nil, err
	}
	current, _ := Load(loc)
	if current != expected {
		lease.Close()
		return nil, errors.New("resume record changed while preparing Codex publication")
	}
	if err := guardLegacyCodexPublication(loc, current, proposed); err != nil {
		lease.Close()
		return nil, err
	}
	return lease, nil
}

// Only legacy publishers use this guard. A heartbeat deadline revokes live
// input authority but does not retire a managed publisher's ownership.
func guardLegacyCodexPublication(loc string, current, proposed Record) error {
	if current.Agent != "codex" || current.Frontend == "" ||
		current.HomeID == proposed.HomeID && current.SessionID == proposed.SessionID {
		return nil
	}
	if legacyCodexIndexReleased(loc, current) {
		return nil
	}
	return &FrontendBindingError{Code: "frontend-index-owned"}
}

func legacyCodexIndexReleased(loc string, current Record) bool {
	if current.FrontendRetired {
		return frontendPath(current.Frontend) != ""
	}
	f, ok := readStoredFrontend(current.Frontend)
	if !ok {
		return legacyUnlinkedFrontendStopped(current)
	}
	if f.HomeID != current.HomeID {
		return false
	}
	if f.Retired {
		return true
	}
	// The stored instance names the same bridge across root changes, including
	// a new root receipt written before its index could be committed.
	if frontendBridgeRecoveryKind(f) == FrontendRecoveryStopped {
		return true
	}
	if f.Loc == loc {
		return false
	}
	// A complete relocation permits the old address to be reused. A receipt
	// written before a failed destination-index write is insufficient.
	moved, exists := Load(f.Loc)
	return exists && !moved.FrontendRetired && moved.Agent == "codex" && moved.HomeID == f.HomeID &&
		moved.SessionID == f.SessionID && moved.Frontend == f.Instance && freshFrontend(f, true)
}

// Old frontends unlinked their bridge receipt on normal close. File absence
// alone is ambiguous. An exact native binding can prove the recorded terminal
// client is gone; PID reuse or unreadable evidence cannot prove that fact.
func legacyUnlinkedFrontendStopped(current Record) bool {
	path := frontendPath(current.Frontend)
	if path == "" || current.HomeID == "" {
		return false
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return false
	}
	bound, ok := native.BindingFor(current.SessionID, current.HomeID)
	return ok && bound.Kind == "frontend" && bound.ID == current.Frontend && bound.PID > 1 &&
		bound.BackendPID > 1 && bound.BackendStart != "" && frontendRecoveryGone(bound.PID)
}

// SaveObserved is the detached hook/one-shot entry. A short lease collision
// must not discard another agent's observed resume update without retrying.
func SaveObserved(loc string, r Record) error {
	if r.Agent == "codex" {
		return Save(loc, r)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := Save(loc, r)
		if !errors.Is(err, state.ErrLeaseHeld) || !time.Now().Before(deadline) {
			diag.ReportStoreFailure("observed-resume", err)
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
}
