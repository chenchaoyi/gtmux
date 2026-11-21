package resume

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/state"
)

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
	f, ok := readFrontend(current.Frontend)
	if !ok {
		path := frontendPath(current.Frontend)
		if path == "" || state.CheckPrivateDirectory(filepath.Dir(path)) != nil {
			return false
		}
		_, err := os.Lstat(path)
		return errors.Is(err, os.ErrNotExist) // EndCodexFrontend unlinks the receipt.
	}
	if f.HomeID != current.HomeID {
		return false
	}
	if f.Loc != loc {
		// A completed relocation can leave a historical index at the old
		// address. A receipt written before a failed index write is not enough.
		moved, exists := Load(f.Loc)
		return exists && moved.Agent == "codex" && moved.HomeID == f.HomeID &&
			moved.SessionID == f.SessionID && moved.Frontend == f.Instance && freshFrontend(f, true)
	}
	return f.SessionID == current.SessionID && frontendBridgeRecoveryKind(f) == FrontendRecoveryStopped
}
