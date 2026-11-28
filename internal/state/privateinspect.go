package state

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
			if !isAllowedManagedSymlink(path) {
				note(path, false)
			}
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

// isAllowedManagedSymlink permits only the managed Codex frontend socket alias
// (<codex-frontends>/client-*/frontend.sock -> /tmp/codex-daemon-<uid>/<hash>)
// created by codexbridge so active Codex panes do not trigger false file-mode
// warnings in doctor. All other symlinks remain unverified.
func isAllowedManagedSymlink(path string) bool {
	if filepath.Base(path) != "frontend.sock" {
		return false
	}
	parent := filepath.Dir(path)
	if !strings.HasPrefix(filepath.Base(parent), "client-") {
		return false
	}
	if !sameManagedDir(filepath.Dir(parent), RootStorePath("codex-frontends")) {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Geteuid() {
		return false
	}
	target, err := os.Readlink(path)
	if err != nil || !filepath.IsAbs(target) || filepath.Clean(target) != target {
		return false
	}
	base := filepath.Base(target)
	if base == "" || base == "." || base == ".." {
		return false
	}
	daemonDir := "codex-daemon-" + strconv.Itoa(os.Geteuid())
	targetDir := filepath.Dir(target)
	if targetDir == filepath.Join("/tmp", daemonDir) {
		return true
	}
	if evalTmp, err := filepath.EvalSymlinks("/tmp"); err == nil && targetDir == filepath.Join(evalTmp, daemonDir) {
		return true
	}
	return false
}

func sameManagedDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ea, errA := filepath.EvalSymlinks(a)
	eb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ea == eb
}
