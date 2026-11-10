package codexbridge

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

const admissionBudget = 4 * time.Second

// The listener can receive the child's connection before cmd.Start returns.
// Closing this channel publishes the exact child PID without an arbitrary sleep.
type launchGate struct {
	ready chan struct{}
	once  sync.Once
	pid   int
}

func newLaunchGate() *launchGate      { return &launchGate{ready: make(chan struct{})} }
func (g *launchGate) publish(pid int) { g.once.Do(func() { g.pid = pid; close(g.ready) }) }
func (g *launchGate) wait(ctx context.Context) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-g.ready:
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if g.pid <= 1 {
			return 0, &resume.FrontendAdmissionError{Code: "launch-unavailable"}
		}
		return g.pid, nil
	}
}

func admissionPause(ctx context.Context) error {
	t := time.NewTimer(50 * time.Millisecond)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Every check uses the same context. Never repeat an uncertain mutation: these
// checks are read-only and occur before any protocol frames are forwarded.
func awaitFrontend(ctx context.Context, check func(context.Context) (resume.CodexFrontend, error), pause func(context.Context) error) (resume.CodexFrontend, int, error) {
	for attempt := 1; attempt <= 4; attempt++ {
		if err := ctx.Err(); err != nil {
			return resume.CodexFrontend{}, attempt - 1, err
		}
		f, err := check(ctx)
		if ctx.Err() != nil {
			return resume.CodexFrontend{}, attempt, ctx.Err()
		}
		if err == nil {
			return f, attempt, nil
		}
		var refusal *resume.FrontendAdmissionError
		if !errors.As(err, &refusal) || !refusal.Retryable || attempt == 4 {
			return resume.CodexFrontend{}, attempt, err
		}
		if err := pause(ctx); err != nil {
			return resume.CodexFrontend{}, attempt, err
		}
	}
	panic("unreachable admission attempt")
}

func admissionReason(err error) string {
	var refusal *resume.FrontendAdmissionError
	if errors.As(err, &refusal) {
		return refusal.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "admission-deadline"
	}
	if errors.Is(err, context.Canceled) {
		return "admission-cancelled"
	}
	return "admission-unavailable"
}

func admissionStatus(err error) int {
	var refusal *resume.FrontendAdmissionError
	if errors.As(err, &refusal) && !refusal.Retryable {
		return http.StatusForbidden
	}
	return http.StatusServiceUnavailable
}
