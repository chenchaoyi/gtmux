package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// RunLock is a "this long job is already in flight" guard, shared by every entry point.
//
// It exists because `gtmux restore` had NO guard at all: two runs in parallel each opened
// the whole working set, so a user who clicked the menu bar's restore row twice — which is
// exactly what a person does when a slow action gives no feedback — got every session and
// tab twice. The in-app fix (disable the row while it runs) cannot cover the CLI, the
// phone, or a second click that races the first, so the floor belongs here.
//
// A lock is a FILE holding the owner's pid and start time. Two things make it safe to
// leave behind: a lock whose process is gone is taken over, and so is one older than its
// max age — a crash must not wedge the feature until someone deletes a file they have
// never heard of.
type RunLock struct {
	path string
}

// NewRunLock names a lock by job ("restore"), under the state dir.
func NewRunLock(job string) *RunLock {
	return &RunLock{path: filepath.Join(Dir(), "run-"+job+".lock")}
}

// Acquire takes the lock, or reports who holds it.
//
// Returns held=false with the holder's age when someone else is running. The caller
// decides what to say — refusing loudly is right for a person at a terminal, and a no-op
// is right for a click.
func (l *RunLock) Acquire(maxAge time.Duration) (held bool, heldFor time.Duration) {
	if pid, started, ok := l.read(); ok {
		age := time.Since(started)
		if processAlive(pid) && age < maxAge {
			return false, age
		}
		// Stale: the owner is gone, or it has run past any plausible duration. Taking it
		// over is the only behaviour that cannot wedge: a crashed restore must not make
		// the feature refuse forever.
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return true, 0 // cannot lock → do NOT block the job; the guard is a courtesy
	}
	body := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix())
	if err := os.WriteFile(l.path, []byte(body), 0o644); err != nil {
		return true, 0
	}
	return true, 0
}

// Release drops the lock. Safe to call when it was never held.
func (l *RunLock) Release() { _ = os.Remove(l.path) }

func (l *RunLock) read() (pid int, started time.Time, ok bool) {
	b, err := os.ReadFile(l.path)
	if err != nil {
		return 0, time.Time{}, false
	}
	f := strings.Fields(string(b))
	if len(f) < 2 {
		return 0, time.Time{}, false
	}
	pid, _ = strconv.Atoi(f[0])
	sec, _ := strconv.ParseInt(f[1], 10, 64)
	if pid <= 0 || sec <= 0 {
		return 0, time.Time{}, false
	}
	return pid, time.Unix(sec, 0), true
}

// processAlive reports whether a pid is still running. Signal 0 asks the kernel without
// delivering anything — the standard "is it there" probe.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
