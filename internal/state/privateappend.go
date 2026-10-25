package state

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// OpenPrivateAppend holds the parent directory fd while opening a regular,
// owned leaf. A symlink swap cannot redirect the append into another file.
func OpenPrivateAppend(path string) (*os.File, error) {
	if managedRoot(path) != "" {
		f, err := OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, PrivateFile)
		if err != nil {
			return nil, err
		}
		// Diagnostic append has always capped owner execution too; generic
		// policy IO preserves owner bits for executable tools.
		info, err := f.Stat()
		if err == nil && info.Mode().Perm()&0100 != 0 {
			err = f.Chmod(info.Mode().Perm() & PrivateFile)
		}
		if err != nil {
			f.Close()
			return nil, err
		}
		return f, nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, PrivateDir); err != nil {
		return nil, err
	}
	parent, err := privateDirectory(dir, true)
	if err != nil {
		return nil, err
	}
	defer unix.Close(parent)
	fd, err := unix.Openat(parent, filepath.Base(path), unix.O_WRONLY|unix.O_CREAT|unix.O_APPEND|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, uint32(PrivateFile))
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err == nil {
		err = checkOwnedRegular(info)
	}
	if err == nil && info.Mode().Perm() != info.Mode().Perm()&PrivateFile {
		err = f.Chmod(info.Mode().Perm() & PrivateFile)
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
func privateDirectory(path string, narrow bool) (int, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, err
	}
	if err := checkPrivateDirectory(fd, narrow); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	return fd, nil
}
func checkPrivateDirectory(fd int, narrow bool) error {
	var info unix.Stat_t
	err := unix.Fstat(fd, &info)
	if err == nil && int(info.Uid) != os.Geteuid() {
		err = fmt.Errorf("directory ownership requires review")
	}
	if err == nil && narrow && uint32(info.Mode)&0o077 != 0 {
		err = unix.Fchmod(fd, uint32(info.Mode)&0o700)
	}
	return err
}

func checkOwnedRegular(info fs.FileInfo) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular private file")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("file ownership requires review")
	}
	return nil
}

// OpenPrivateRead does not chmod or follow a log leaf/parent symlink.
func OpenPrivateRead(path string) (*os.File, error) {
	parent, err := privateDirectory(filepath.Dir(path), false)
	if err != nil {
		return nil, err
	}
	defer unix.Close(parent)
	fd, err := unix.Openat(parent, filepath.Base(path), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err == nil {
		err = checkOwnedRegular(info)
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// CheckPrivateDirectory verifies the final directory without following links or
// changing permissions, including when it contains no history files yet.
func CheckPrivateDirectory(path string) error {
	fd, err := privateDirectory(path, false)
	if err != nil {
		return err
	}
	return unix.Close(fd)
}
