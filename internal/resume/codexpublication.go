package resume

import (
	"errors"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// lockCodexPublication joins Save/Bind/Refresh's kernel-owned publication lease.
// Preparation may read history or processes without holding this global lease;
// every index field must still match before any prepared result is published.
// The caller retains the lease through publication and retirement side effects.
func lockCodexPublication(loc string, expected Record) (*state.Lease, error) {
	lease, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		return nil, err
	}
	current, _ := Load(loc)
	if current != expected {
		lease.Close()
		return nil, errors.New("resume record changed while preparing Codex publication")
	}
	return lease, nil
}
