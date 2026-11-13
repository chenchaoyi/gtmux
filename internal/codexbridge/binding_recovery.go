package codexbridge

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const bindingRecoveryBudget = 30 * time.Second
const bindingRecoveryAttempts = 8

type pendingRootBinding struct {
	id, cwd  string
	deadline time.Time
	next     time.Time
	attempts int
}

// The connection serializes observer and heartbeat access here. No method emits
// a protocol frame, searches for a thread, or trusts inherited pane metadata.
type frontendBinding struct {
	mu         sync.Mutex
	attachment resume.CodexFrontend
	current    func() bool
	bind       func(resume.CodexFrontend, string, string) (resume.CodexFrontend, error)
	refresh    func(resume.CodexFrontend) (resume.CodexFrontend, error)
	end        func(resume.CodexFrontend)
	report     func(string, error, int)
	now        func() time.Time
	pending    *pendingRootBinding
	suspended  bool
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
	publish    func(func()) error
}

func newFrontendBinding(f resume.CodexFrontend, current func() bool, report func(string, error, int)) *frontendBinding {
	return makeFrontendBinding(context.Background(), f, current, report)
}

func makeFrontendBinding(parent context.Context, f resume.CodexFrontend, current func() bool, report func(string, error, int)) *frontendBinding {
	ctx, cancel := context.WithCancel(parent)
	b := &frontendBinding{attachment: f, current: current, ctx: ctx, cancel: cancel, refresh: resume.RefreshCodexFrontend, end: resume.EndCodexFrontend, report: report, now: time.Now}
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		return resume.BindCodexFrontendContext(ctx, f, id, cwd)
	}
	b.publish = func(commit func()) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !current() {
			return errors.New("frontend connection retired")
		}
		commit()
		return nil
	}
	return b
}

func newConnectionBinding(ctx context.Context, f resume.CodexFrontend, epoch *connectionEpoch, report func(string, error, int)) *frontendBinding {
	b := makeFrontendBinding(ctx, f, func() bool { return epoch.current(f.Sequence) }, report)
	b.publish = func(commit func()) error { return epoch.publish(b.ctx, f.Sequence, commit) }
	return b
}

// A new selection retires an unresolved older root even before its response.
func (b *frontendBinding) selectRoot() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = nil
}

func (b *frontendBinding) resetHome() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = nil
	b.suspended = true
}

func (b *frontendBinding) stage(id, cwd string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.ctx.Err() != nil || !b.current() {
		return
	}
	now := b.now()
	b.pending = &pendingRootBinding{id: id, cwd: cwd, deadline: now.Add(bindingRecoveryBudget), next: now}
	// Do not keep the old root apparently current after an observed switch.
	b.suspended = true
	b.attempt()
}

// attempt is called with mu held. It retries publication of the same observed
// root, never an RPC or task input. Every attempt repeats Bind's strict checks.
func (b *frontendBinding) attempt() {
	p := b.pending
	if p == nil || b.closed || b.ctx.Err() != nil || !b.current() {
		b.pending = nil
		return
	}
	now := b.now()
	if !now.Before(p.deadline) || p.attempts >= bindingRecoveryAttempts {
		b.pending = nil
		b.log("expired", errors.New("frontend binding recovery budget exhausted"), p.attempts)
		return
	}
	if now.Before(p.next) {
		return
	}
	var err error
	committed, expired := false, false
	permitErr := b.publish(func() {
		if !b.now().Before(p.deadline) || p.attempts >= bindingRecoveryAttempts {
			expired = true
			return
		}
		if b.ctx.Err() != nil {
			err = b.ctx.Err()
			return
		}
		p.attempts++
		var f resume.CodexFrontend
		f, err = b.bind(b.attachment, p.id, p.cwd)
		var receipt *resume.CodexBindingReceiptError
		if err == nil || errors.As(err, &receipt) {
			// Keep the attachment commit inside the same epoch permission as IO.
			b.attachment, b.pending, b.suspended = f, nil, false
			committed = true
		}
	})
	if permitErr != nil {
		b.pending = nil
		b.log("retired", permitErr, p.attempts)
		return
	}
	if expired {
		b.pending = nil
		b.log("expired", errors.New("frontend binding recovery budget exhausted"), p.attempts)
		return
	}
	if committed {
		if err != nil {
			b.log("receipt", err, p.attempts)
		} else if p.attempts > 1 {
			b.log("ready", nil, p.attempts)
		}
		return
	}
	var proof *resume.FrontendBindingError
	retry := errors.Is(err, state.ErrLeaseHeld) || errors.As(err, &proof) && proof.Retryable
	if !retry {
		b.pending = nil
		b.log("failed", err, p.attempts)
		return
	}
	b.log("failed", err, p.attempts)
	// Start pacing after the completed check, not before potentially slow IO.
	delay := time.Second << min(p.attempts-1, 3)
	p.next = b.now().Add(delay)
}

func (b *frontendBinding) log(kind string, err error, attempts int) {
	if b.report != nil {
		b.report(kind, err, attempts)
	}
}

func (b *frontendBinding) tick() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.ctx.Err() != nil || !b.current() {
		b.pending = nil
		return
	}
	if b.pending != nil {
		b.attempt()
		return
	}
	if !b.suspended && b.attachment.SessionID != "" {
		_ = b.publish(func() {
			if b.ctx.Err() != nil {
				return
			}
			if f, e := b.refresh(b.attachment); e == nil {
				b.attachment = f
			}
		})
	}
}

func (b *frontendBinding) close() {
	// Signal cancellation before waiting for an in-flight publication's mutex.
	b.cancel()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed, b.suspended, b.pending = true, true, nil
	b.end(b.attachment)
}
