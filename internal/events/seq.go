package events

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// seqPath is the persistent monotonic-sequence counter (last assigned seq, as
// decimal text). It backs the cursor + gap detection: every event carries a
// strictly increasing Seq so a consumer has a total order and a durable cursor
// position independent of file byte offsets (which rotation invalidates).
func seqPath() string { return filepath.Join(state.Dir(), "events.seq") }

// nextSeq atomically increments and returns the next sequence number, serialized
// across concurrent hook processes with an advisory file lock (flock — cgo-free on
// darwin/linux). It returns 0 on any failure so a caller treats "no seq" gracefully
// (Append keeps the record's seq unset, which reads as sequence-unknown). The lock
// is held only for the tiny read-increment-write, so contention is microseconds on
// an uncontended local file.
func nextSeq() int64 {
	// The counter file doubles as the lock target.
	f, err := os.OpenFile(seqPath(), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return 0
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return 0
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	buf := make([]byte, 32)
	n, _ := f.ReadAt(buf, 0)
	cur, _ := strconv.ParseInt(strings.TrimSpace(string(buf[:n])), 10, 64)
	next := cur + 1
	// Rewrite from the start (the value only grows, so length is non-decreasing).
	if _, err := f.WriteAt([]byte(strconv.FormatInt(next, 10)), 0); err != nil {
		return 0
	}
	return next
}
