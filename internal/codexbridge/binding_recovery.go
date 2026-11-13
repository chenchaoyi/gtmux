package codexbridge

import (
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
}

func newFrontendBinding(f resume.CodexFrontend, current func() bool, report func(string, error, int)) *frontendBinding {
	return &frontendBinding{attachment: f, current: current, bind: func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		return resume.BindCodexFrontend(f, id, cwd)
	}, refresh: resume.RefreshCodexFrontend, end: resume.EndCodexFrontend, report: report, now: time.Now}
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
	if b.closed || !b.current() {
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
	if p == nil || b.closed || !b.current() {
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
	p.attempts++
	f, err := b.bind(b.attachment, p.id, p.cwd)
	var receipt *resume.CodexBindingReceiptError
	if err == nil || errors.As(err, &receipt) {
		b.attachment, b.pending, b.suspended = f, nil, false
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
	if b.closed || !b.current() {
		b.pending = nil
		return
	}
	if b.pending != nil {
		b.attempt()
		return
	}
	if !b.suspended && b.attachment.SessionID != "" {
		if f, e := b.refresh(b.attachment); e == nil {
			b.attachment = f
		}
	}
}

func (b *frontendBinding) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed, b.suspended, b.pending = true, true, nil
	b.end(b.attachment)
}
