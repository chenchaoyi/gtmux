package state

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

type PrivateInspection struct {
	Wide, Unverified int
	Example          string
}

// InspectPrivateTree never follows or repairs symlinks. Unreadable or foreign
// paths remain unverified instead of being silently counted as owner-only.
func InspectPrivateTree(root string) PrivateInspection {
	var result PrivateInspection
	note := func(path string, wide bool) {
		if result.Example == "" {
			result.Example = path
		}
		if wide {
			result.Wide++
		} else {
			result.Unverified++
		}
	}
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path != root || !os.IsNotExist(walkErr) {
				note(path, false)
			}
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			note(path, false)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			note(path, false)
			return nil
		}
		if !entry.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || int(st.Uid) != os.Geteuid() {
			note(path, false)
			return nil
		}
		if info.Mode().Perm()&0o077 != 0 {
			note(path, true)
		}
		return nil
	})
	return result
}
