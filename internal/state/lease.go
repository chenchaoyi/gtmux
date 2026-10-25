package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var ErrLeaseHeld = errors.New("another process holds the lease")

// Lease is a kernel-owned, non-expiring process lease. Its file stays in place:
// unlinking a live flock target would allow two owners to lock different inodes.
// Process exit releases the lock even when no deferred cleanup can run.
type Lease struct{ file *os.File }

func TryLease(name string) (*Lease, error) {
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid lease name")
	}
	if err := MkdirAll(Dir(), 0700); err != nil {
		return nil, err
	}
	f, err := OpenFile(filepath.Join(Dir(), "lease-"+name+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrLeaseHeld
		}
		return nil, err
	}
	return &Lease{file: f}, nil
}

func (l *Lease) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f.Close()
}
