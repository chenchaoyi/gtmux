package codexbridge

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Reserving an attempt must not silence a still-valid incumbent. Only a
// successful transport upgrade can publish the next active connection epoch.
type connectionEpoch struct {
	issued      atomic.Uint64
	active      atomic.Uint64
	once        sync.Once
	publication chan struct{}
}

func (e *connectionEpoch) reserve() uint64 { return e.issued.Add(1) }

func (e *connectionEpoch) activate(id uint64) bool {
	ok, _ := e.activateContext(context.Background(), id)
	return ok
}

func (e *connectionEpoch) acquire(ctx context.Context) (func(), error) {
	e.once.Do(func() { e.publication = make(chan struct{}, 1) })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case e.publication <- struct{}{}:
		unlock := func() { <-e.publication }
		if err := ctx.Err(); err != nil {
			unlock()
			return nil, err
		}
		if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			unlock()
			return nil, context.DeadlineExceeded
		}
		return unlock, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *connectionEpoch) activateContext(ctx context.Context, id uint64) (bool, error) {
	if id == 0 || id > e.issued.Load() {
		return false, nil
	}
	unlock, err := e.acquire(ctx)
	if err != nil {
		return false, err
	}
	defer unlock()
	for {
		previous := e.active.Load()
		if id <= previous {
			return false, nil
		}
		if e.active.CompareAndSwap(previous, id) {
			return true, nil
		}
	}
}

// Activation and the entire Bind/attachment commit share this permit. A newer
// connection cannot become active halfway through an incumbent publication.
func (e *connectionEpoch) publish(ctx context.Context, id uint64, commit func()) error {
	unlock, err := e.acquire(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if !e.current(id) {
		return errors.New("frontend connection retired")
	}
	commit()
	return nil
}

func (e *connectionEpoch) current(id uint64) bool { return id > 0 && e.active.Load() == id }
