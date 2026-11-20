package state

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"golang.org/x/sys/unix"
)

var legacyProfileLockName = regexp.MustCompile(`^lease-profile-[0-9a-f]{64}\.json\.lock$`)

// LegacyProfileLockCount counts eligible legacy lock files without acquiring a lease.
func LegacyProfileLockCount() (count int, err error) {
	err = visitLegacyProfileLocks(func(_ int, _ string, _ unix.Stat_t) error {
		count++
		return nil
	})
	return count, err
}

// PruneLegacyProfileLocks requires all legacy writers to have been retired.
// Flock can detect a current holder, but a legacy process might have opened
// the inode and not yet acquired its lock.
func PruneLegacyProfileLocks() (removed, busy int, err error) {
	err = visitLegacyProfileLocks(func(dir int, name string, original unix.Stat_t) error {
		err := removeLegacyProfileLock(dir, name, original)
		switch {
		case err == nil:
			removed++
		case errors.Is(err, ErrLeaseHeld):
			busy++
		case !errors.Is(err, unix.ENOENT):
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	})
	return removed, busy, err
}

func visitLegacyProfileLocks(visit func(int, string, unix.Stat_t) error) error {
	path := Dir()
	fd, err := privateDirectory(path, false)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(fd), path)
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		if !legacyProfileLockName.MatchString(name) {
			continue
		}
		var info unix.Stat_t
		err := unix.Fstatat(fd, name, &info, unix.AT_SYMLINK_NOFOLLOW)
		if errors.Is(err, unix.ENOENT) {
			continue
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if isLegacyProfileLock(info) {
			if err := visit(fd, name, info); err != nil {
				return err
			}
		}
	}
	return nil
}

func isLegacyProfileLock(info unix.Stat_t) bool {
	return info.Mode&unix.S_IFMT == unix.S_IFREG && info.Size == 0 &&
		int(info.Uid) == os.Geteuid() && info.Nlink == 1
}

func removeLegacyProfileLock(dir int, name string, original unix.Stat_t) error {
	fd, err := unix.Openat(dir, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	var opened unix.Stat_t
	if err := unix.Fstat(fd, &opened); err != nil {
		return err
	}
	if opened.Dev != original.Dev || opened.Ino != original.Ino || !isLegacyProfileLock(opened) {
		return errors.New("legacy lock changed during cleanup")
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return ErrLeaseHeld
		}
		return err
	}
	var current unix.Stat_t
	if err := unix.Fstatat(dir, name, &current, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if current.Dev != opened.Dev || current.Ino != opened.Ino || !isLegacyProfileLock(current) {
		return errors.New("legacy lock changed during cleanup")
	}
	return unix.Unlinkat(dir, name, 0)
}
