package resume

import (
	"context"
	"os"
	"slices"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// FrontendAdmissionError is deliberately value-free: it can be safely logged or
// shown in the terminal without arguments, prompt text or transport credentials.
type FrontendAdmissionError struct {
	Code      string
	Retryable bool
}

func (e *FrontendAdmissionError) Error() string { return "frontend admission: " + e.Code }

var frontendLaunchPane = codexterminal.ReadLaunchPane
var frontendLaunchObserve = codexterminal.ObserveLaunch
var frontendLaunchProcess = launchhost.ReadAgentContext

// BeginCodexFrontendFromLaunch checks one controlled launch, not any client
// discovered by recency. Admission is read-only; only a subsequent observed
// successful root-thread operation may publish a binding.
func BeginCodexFrontendFromLaunch(ctx context.Context, pane, instance string, peerPID, backendPID int, sequence uint64, childPID int) (CodexFrontend, error) {
	fail := func(code string, retry bool) (CodexFrontend, error) {
		return CodexFrontend{}, &FrontendAdmissionError{Code: code, Retryable: retry}
	}
	if frontendPath(instance) == "" || sequence == 0 || peerPID <= 1 || backendPID <= 1 || childPID <= 1 {
		return fail("invalid-request", false)
	}
	if ctx.Err() != nil {
		return fail("admission-deadline", false)
	}
	p, ok := frontendLaunchPane(ctx, pane)
	if !ok {
		return fail("pane-unavailable", false)
	}
	o := frontendLaunchObserve(ctx, p, childPID, os.Getpid(), peerPID)
	if o.Code != "" {
		return fail(o.Code, o.Retryable())
	}
	if o.PID != peerPID {
		return fail("peer-mismatch", false)
	}
	if o.Start == "" || o.RootStart == "" || o.TTY == "" || p.Loc == "" || p.Cwd == "" {
		return fail("terminal-metadata-unavailable", false)
	}
	if !slices.Contains(o.Ancestors, childPID) || !slices.Contains(o.Ancestors, os.Getpid()) {
		return fail("launch-mismatch", false)
	}
	b, exists := frontendLaunchProcess(ctx, os.Getpid())
	backend, backendExists := frontendLaunchProcess(ctx, backendPID)
	if !exists || b.Start == "" || !backendExists || backend.Start == "" {
		return fail("process-unavailable", true)
	}
	if b.PID != os.Getpid() || backend.PID != backendPID || backend.Comm != "codex" {
		return fail("backend-mismatch", false)
	}
	if ctx.Err() != nil {
		return fail("admission-deadline", false)
	}
	now := time.Now().UnixMilli()
	t := CodexBindingTarget{Pane: pane, Loc: p.Loc, Cwd: p.Cwd, PID: o.PID, PanePID: p.PID, ClientStart: o.Start, RootStart: o.RootStart, TTY: o.TTY}
	return CodexFrontend{Instance: instance, HomeID: transcript.CodexHomeID(), BridgePID: b.PID,
		BridgeStart: b.Start, StartedAt: now, UpdatedAt: now, Sequence: sequence, BackendPID: backendPID, BackendStart: backend.Start, CodexBindingTarget: t}, nil
}
