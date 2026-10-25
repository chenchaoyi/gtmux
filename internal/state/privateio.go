package state

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// IsManagedPath reports whether policy IO owns the supplied destination.
func IsManagedPath(path string) bool { return managedRoot(path) != "" }

// MkdirAll applies the private-root policy locally, even with a permissive umask
// or an existing wide directory. Caller-selected paths outside the two managed
// roots retain os semantics (external settings, exports and executable installs).
func MkdirAll(path string, perm os.FileMode) error {
	if managedRoot(path) == "" {
		return os.MkdirAll(path, perm)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	fd, err := privateWriteParent(filepath.Join(path, ".ensure-directory"))
	if err != nil {
		return err
	}
	return unix.Close(fd)
}

// OpenFile keeps inode/append/exclusive/flock semantics. Narrow before truncating
// or writing so an existing public inode cannot expose new private bytes. Owner
// bits are retained, including executable HQ tools. Links and special files fail.
func OpenFile(path string, flags int, perm os.FileMode) (*os.File, error) {
	// Read-only metadata inspection must not turn into an implicit repair.
	if flags&(os.O_WRONLY|os.O_RDWR|os.O_CREATE) == 0 {
		return os.OpenFile(path, flags, perm)
	}
	if managedRoot(path) == "" {
		return os.OpenFile(path, flags, perm)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	parent, err := privateWriteParent(absolute)
	if err != nil {
		return nil, fmt.Errorf("private parent: %w", err)
	}
	defer unix.Close(parent)
	fd, err := openPrivateAt(parent, filepath.Base(absolute), flags, uint32(perm.Perm()&0700))
	if err != nil {
		return nil, &os.PathError{Op: "open private", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err == nil {
		err = checkOwnedRegular(info)
	}
	if err == nil && info.Mode().Perm()&0077 != 0 {
		err = f.Chmod(info.Mode().Perm() & 0700)
	}
	if err == nil && flags&unix.O_NONBLOCK == 0 {
		err = unix.SetNonblock(fd, false)
	}
	if err == nil && flags&os.O_TRUNC != 0 {
		err = f.Truncate(0)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// WriteFile preserves the existing overwrite contract. Atomic publishers use
// WritePrivateFile instead; appenders and lock holders use OpenFile.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	f, err := OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// CreateTemp and MkdirTemp keep the standard random/exclusive 0600/0700 leaves,
// and ensure their managed parent chain is private before creating anything.
func CreateTemp(dir, pattern string) (*os.File, error) {
	if managedRoot(dir) != "" {
		if err := MkdirAll(dir, PrivateDir); err != nil {
			return nil, err
		}
	}
	return os.CreateTemp(dir, pattern)
}
func MkdirTemp(dir, pattern string) (string, error) {
	if managedRoot(dir) != "" {
		if err := MkdirAll(dir, PrivateDir); err != nil {
			return "", err
		}
	}
	return os.MkdirTemp(dir, pattern)
}

// openPrivateAt retries only ENOENT during creation, with a finite bound and the
// same held parent. Real missing parents, links and permission errors stay errors.
func openPrivateAt(parent int, name string, flags int, perm uint32) (int, error) {
	var err error
	var fd int
	// Concurrent O_CREAT|O_NOFOLLOW opens can race a negative lookup on APFS.
	// Retry only creation's ENOENT, against the same held directory descriptor.
	for attempt := 0; attempt < 8; attempt++ {
		fd, err = unix.Openat(parent, name, (flags&^os.O_TRUNC)|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, perm)
		if err != unix.ENOENT || flags&os.O_CREATE == 0 {
			break
		}
	}
	return fd, err
}
