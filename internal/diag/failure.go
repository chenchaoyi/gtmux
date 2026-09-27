package diag

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
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

var failureReport = struct {
	sync.Mutex
	last map[string]time.Time
}{last: make(map[string]time.Time)}

// ReportStoreFailure is the last-resort channel when a JSONL store cannot write.
// It deliberately sends only the store and filesystem error to stderr, never the
// entry's text. Repeated failures are capped at one line per store per minute.
// Doctor's write probe provides a later check even when stderr is unavailable.
func ReportStoreFailure(store string, err error) {
	if err == nil {
		return
	}
	failureReport.Lock()
	defer failureReport.Unlock()
	if time.Since(failureReport.last[store]) < time.Minute {
		return
	}
	failureReport.last[store] = time.Now()
	_, _ = fmt.Fprintf(os.Stderr, "gtmux: %s store write failed: %v; run gtmux doctor\n", store, err)
}
