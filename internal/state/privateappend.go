package state

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// OpenPrivateAppend holds the parent directory fd while opening a regular,
// owned leaf. A symlink swap cannot redirect the append into another file.
func OpenPrivateAppend(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	// Protect the data root as well as the log directory against replacement by
	// other local users. Do not broaden owner permissions or change foreign files.
	for _, root := range []string{Dir(), ConfigDir()} {
		if dir == root || strings.HasPrefix(dir, root+string(filepath.Separator)) {
			if err := os.MkdirAll(root, PrivateDir); err != nil {
				return nil, err
			}
			fd, err := privateDirectory(root, true)
			if err != nil {
				return nil, err
			}
			_ = unix.Close(fd)
		}
	}
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
	var info unix.Stat_t
	err = unix.Fstat(fd, &info)
	if err == nil && int(info.Uid) != os.Geteuid() {
		err = fmt.Errorf("directory ownership requires review: %s", path)
	}
	if err == nil && narrow && uint32(info.Mode)&0o077 != 0 {
		err = unix.Fchmod(fd, uint32(info.Mode)&0o700)
	}
	if err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	return fd, nil
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
