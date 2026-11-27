package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Existing messages contain absolute upload paths. Existing launchd profiles
// likewise open their stderr files by the original log path. Only these exact
// directory aliases are part of the layout contract. Ordinary private writes
// keep using the physical destination with no-follow directory descriptors.
type purposeReference struct {
	key          string
	link, target layoutEntry
}

func referencePaths(key string) (string, string, error) {
	switch key {
	case "uploads", "logs":
		old, canonical, _ := purposeEndpoints(key, true)
		return old, canonical, nil
	default:
		return "", "", errors.New("unregistered storage compatibility reference")
	}
}

func inspectPurposeReference(key string, expected layoutEntry) (purposeReference, error) {
	old, canonical, err := referencePaths(key)
	if err != nil {
		return purposeReference{}, err
	}
	id, found, err := workflowPathObject(canonical)
	if err != nil {
		return purposeReference{}, err
	}
	expected.Name = ""
	if !found || expected.Mode != uint32(os.ModeDir) || id != expected {
		return purposeReference{}, fmt.Errorf("storage reference target changed: %s", key)
	}
	target, err := os.Readlink(old)
	if err != nil {
		return purposeReference{}, err
	}
	rel, err := filepath.Rel(Dir(), canonical)
	if err != nil || target != rel {
		return purposeReference{}, fmt.Errorf("storage reference points elsewhere: %s", key)
	}
	var stat unix.Stat_t
	if err := unix.Lstat(old, &stat); err != nil {
		return purposeReference{}, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFLNK || int(stat.Uid) != os.Geteuid() || stat.Nlink != 1 {
		return purposeReference{}, fmt.Errorf("storage reference identity requires review: %s", key)
	}
	return purposeReference{key: key, link: layoutEntry{Dev: uint64(stat.Dev), Inode: uint64(stat.Ino)}, target: id}, nil
}

// Run under exclusive admission after moving the verified source. Symlinkat
// never replaces an existing object. Retry accepts only the exact relationship.
func publishPurposeReference(key string, expected layoutEntry) (purposeReference, error) {
	old, canonical, err := referencePaths(key)
	if err != nil {
		return purposeReference{}, err
	}
	expected.Name = ""
	id, ok, err := workflowPathObject(canonical)
	if err != nil {
		return purposeReference{}, err
	}
	if !ok || id != expected || id.Mode != uint32(os.ModeDir) {
		return purposeReference{}, errors.New("invalid storage reference target")
	}
	parent, err := privateDirectory(Dir(), false)
	if err != nil {
		return purposeReference{}, err
	}
	defer unix.Close(parent)
	rel, err := filepath.Rel(Dir(), canonical)
	if err != nil {
		return purposeReference{}, err
	}
	if err := unix.Symlinkat(rel, parent, filepath.Base(old)); err != nil && !errors.Is(err, unix.EEXIST) {
		return purposeReference{}, err
	}
	ref, err := inspectPurposeReference(key, expected)
	if err != nil {
		return purposeReference{}, err
	}
	return ref, unix.Fsync(parent)
}

func (ref purposeReference) retire() error {
	current, err := inspectPurposeReference(ref.key, ref.target)
	if err != nil {
		return err
	}
	if current != ref {
		return errors.New("storage reference changed during review")
	}
	parent, err := privateDirectory(Dir(), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	var st unix.Stat_t
	if err := unix.Fstatat(parent, ref.key, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFLNK || uint64(st.Dev) != ref.link.Dev || uint64(st.Ino) != ref.link.Inode {
		return errors.New("storage reference identity changed")
	}
	if err := unix.Unlinkat(parent, ref.key, 0); err != nil {
		return err
	}
	return unix.Fsync(parent)
}
