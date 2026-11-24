package diag

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// NewOpID gives one action a key shared by the journal, diagnostics and, when
// applicable, the knowledge ledger. It carries no pane, payload or personal data.
func NewOpID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	// A broken entropy source must not prevent the action from being recorded.
	return fmt.Sprintf("fallback-%x-%x-%x", time.Now().UnixNano(), os.Getpid(), fallbackID.Add(1))
}

var fallbackID atomic.Uint64

const storeWarningInterval = time.Minute

var failureReport = struct {
	sync.Mutex
	last map[string]time.Time
}{last: make(map[string]time.Time)}

type systemStoreWarnings struct {
	mu              sync.Mutex
	emit            func(string) bool
	last            map[string]time.Time
	fullSent        bool
	fullLastAttempt time.Time
}

var systemWarnings atomic.Pointer[systemStoreWarnings]

// RouteStoreFailuresToSystemLog protects the terminal owned by the interactive
// Codex client from diagnostics emitted by its bridge goroutines. The bridge
// installs the route before launching the client and restores it after joining
// those goroutines. A failure to reach syslog never falls back to the terminal.
func RouteStoreFailuresToSystemLog() func() {
	return routeStoreFailuresToSystemLog(writeSystemStoreWarning)
}

func routeStoreFailuresToSystemLog(emit func(string) bool) func() {
	warnings := &systemStoreWarnings{emit: emit, last: make(map[string]time.Time)}
	old := systemWarnings.Swap(warnings)
	return func() { systemWarnings.Store(old) }
}

func (w *systemStoreWarnings) report(store string, err error, at time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// System logs may have different readers from the private gtmux store.
	// Include only a store name and the OS reason, without paths or payloads.
	cause := "filesystem write unavailable"
	var errno syscall.Errno
	if errors.As(err, &errno) {
		cause = errno.Error()
	}
	message := fmt.Sprintf("%s store write failed: %s; run gtmux doctor", store, cause)
	if store == "diagnostics" && errors.Is(err, syscall.ENOSPC) {
		if w.fullSent || at.Sub(w.fullLastAttempt) < storeWarningInterval {
			return
		}
		w.fullLastAttempt = at
		w.fullSent = w.emit(message)
		return
	}
	if at.Sub(w.last[store]) >= storeWarningInterval {
		w.last[store] = at
		w.emit(message)
	}
}

func noteDiagnosticWriteSuccess() {
	if w := systemWarnings.Load(); w != nil {
		w.mu.Lock()
		w.fullSent = false
		w.fullLastAttempt = time.Time{}
		w.mu.Unlock()
	}
}

// ReportStoreFailure is the last-resort channel when a JSONL store cannot write.
// Ordinary commands report on stderr. An interactive bridge reports out of band.
// Doctor's write probe remains available independently of either channel.
func ReportStoreFailure(store string, err error) {
	if err == nil {
		return
	}
	if w := systemWarnings.Load(); w != nil {
		w.report(store, err, time.Now())
		return
	}
	failureReport.Lock()
	defer failureReport.Unlock()
	if time.Since(failureReport.last[store]) < storeWarningInterval {
		return
	}
	failureReport.last[store] = time.Now()
	_, _ = fmt.Fprintf(os.Stderr, "gtmux: %s store write failed: %v; run gtmux doctor\n", store, err)
}
