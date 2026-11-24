package state

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// An empty private directory is an atomic layout marker. The directory-only
// operations cannot replace or unlink a regular file, a symlink, or its data.
func readLayoutMarker() (layoutEntry, bool, error) {
	parent, err := privateDirectory(Dir(), false)
	if errors.Is(err, os.ErrNotExist) {
		return layoutEntry{}, false, nil
	}
	if err != nil {
		return layoutEntry{}, false, err
	}
	defer unix.Close(parent)
	fd, err := unix.Openat(parent, layoutMarker, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err == unix.ENOENT {
		return layoutEntry{}, false, nil
	}
	if err != nil {
		return layoutEntry{}, false, fmt.Errorf("invalid grouped storage layout marker: %w", err)
	}
	marker := os.NewFile(uintptr(fd), filepath.Join(Dir(), layoutMarker))
	defer marker.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return layoutEntry{}, false, err
	}
	id, err := layoutDirectoryIdentity(stat)
	if err != nil {
		return layoutEntry{}, false, fmt.Errorf("invalid grouped storage layout marker: %w", err)
	}
	if names, err := marker.Readdirnames(1); len(names) != 0 || !errors.Is(err, io.EOF) {
		return layoutEntry{}, false, errors.New("grouped storage layout marker must be empty")
	}
	return id, true, nil
}

func layoutDirectoryIdentity(stat unix.Stat_t) (layoutEntry, error) {
	if int(stat.Uid) != os.Geteuid() || stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&0o077 != 0 {
		return layoutEntry{}, errors.New("storage directory identity requires review")
	}
	return layoutEntry{Dev: uint64(stat.Dev), Inode: uint64(stat.Ino), Mode: uint32(os.ModeDir)}, nil
}

func layoutDirectoryAt(parent int, name string) (layoutEntry, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(parent, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return layoutEntry{}, err
	}
	return layoutDirectoryIdentity(stat)
}

func createLayoutMarker() error {
	if _, present, err := readLayoutMarker(); err != nil || present {
		return err
	}
	parent, err := privateDirectory(Dir(), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	return unix.Mkdirat(parent, layoutMarker, uint32(PrivateDir))
}

func removeLayoutMarker(expected layoutEntry) error {
	id, present, err := readLayoutMarker()
	if err != nil || !present {
		return err
	}
	if id != expected {
		return errors.New("grouped storage layout marker identity changed")
	}
	parent, err := privateDirectory(Dir(), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	if current, err := layoutDirectoryAt(parent, layoutMarker); err != nil || current != expected {
		return errors.New("grouped storage layout marker changed before rollback")
	}
	return unix.Unlinkat(parent, layoutMarker, unix.AT_REMOVEDIR)
}
