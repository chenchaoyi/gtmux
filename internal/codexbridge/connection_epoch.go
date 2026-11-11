package codexbridge

import "sync/atomic"

// Reserving an attempt must not silence a still-valid incumbent. Only a
// successful transport upgrade can publish the next active connection epoch.
type connectionEpoch struct {
	issued atomic.Uint64
	active atomic.Uint64
}

func (e *connectionEpoch) reserve() uint64 { return e.issued.Add(1) }

func (e *connectionEpoch) activate(id uint64) bool {
	if id == 0 || id > e.issued.Load() {
		return false
	}
	for {
		previous := e.active.Load()
		if id <= previous {
			return false
		}
		if e.active.CompareAndSwap(previous, id) {
			return true
		}
	}
}

func (e *connectionEpoch) current(id uint64) bool { return id > 0 && e.active.Load() == id }
