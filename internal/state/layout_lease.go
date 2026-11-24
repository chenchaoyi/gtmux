package state

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// EnterStorageCommand keeps command routing fixed until that command exits.
func EnterStorageCommand() (*Lease, error) {
	lease, err := acquireLayoutLease(unix.LOCK_SH)
	if err != nil {
		return nil, err
	}
	if err := CheckStorageLayout(); err != nil {
		lease.Close()
		return nil, err
	}
	return lease, nil
}

func acquireLayoutLease(mode int) (*Lease, error) {
	file, err := OpenPrivateLock(filepath.Join(Dir(), "lease-storage-layout.lock"))
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), mode|unix.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrLeaseHeld
		}
		return nil, err
	}
	return &Lease{file: file}, nil
}
