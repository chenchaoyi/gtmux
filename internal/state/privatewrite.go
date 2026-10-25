package state

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// WritePrivateFile publishes bytes atomically within a gtmux-managed root. Unlike
// WriteFile, it cannot retain an existing public mode; unlike a filesystem copy,
// it never imports the source mode. Temporary bytes are private from creation.
// Existing links, special files and foreign-owned targets require review.
func WritePrivateFile(path string, data []byte) error {
	parent, err := privateWriteParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	name := filepath.Base(path)
	var dest unix.Stat_t
	if err := unix.Fstatat(parent, name, &dest, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		if dest.Mode&unix.S_IFMT != unix.S_IFREG || int(dest.Uid) != os.Geteuid() {
			return fmt.Errorf("private destination requires review: %s", path)
		}
	} else if err != unix.ENOENT {
		return err
	}
	// O_EXCL and the held parent fd also protect cleanup/publication from path
	// replacement. No chmod or rename can follow a leaf into another tree.
	tmp := ".private-" + rand.Text()
	fd, err := unix.Openat(parent, tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, uint32(PrivateFile))
	if err != nil {
		return err
	}
	defer unix.Unlinkat(parent, tmp, 0)
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return unix.Renameat(parent, tmp, parent, name)
}

// Only directories at/below the selected managed root are narrowed. The existing
// privateDirectory ownership check is shared with diagnostic append/read writers.
// Walk using directory fds so symlinked intermediate directories are rejected too.
func privateWriteParent(path string) (int, error) {
	path = filepath.Clean(path)
	root := ""
	for _, candidate := range []string{Dir(), ConfigDir()} {
		if strings.HasPrefix(path, candidate+string(filepath.Separator)) {
			root = candidate
			break
		}
	}
	if root == "" {
		return -1, fmt.Errorf("not a gtmux private path: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(root), PrivateDir); err != nil {
		return -1, err
	}
	parent, err := privateDirectory(filepath.Dir(root), false)
	if err != nil {
		return -1, err
	}
	rel, err := filepath.Rel(filepath.Dir(root), filepath.Dir(path))
	if err != nil {
		unix.Close(parent)
		return -1, err
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if err := unix.Mkdirat(parent, part, uint32(PrivateDir)); err != nil && err != unix.EEXIST {
			unix.Close(parent)
			return -1, err
		}
		next, err := unix.Openat(parent, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		unix.Close(parent)
		if err != nil {
			return -1, err
		}
		if err := checkPrivateDirectory(next, true); err != nil {
			unix.Close(next)
			return -1, err
		}
		parent = next
	}
	return parent, nil
}
