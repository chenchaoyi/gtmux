package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type CacheStaging struct {
	Count int
	Bytes int64
	files []workflowCacheFile
}

type workflowCacheFile struct {
	path     string
	id       layoutEntry
	bytes    int64
	modified time.Time
}

// InspectCacheStaging counts only abandoned atomic stages in rebuildable caches.
// Shared queue stages, attachments and arbitrary filenames are out of scope.
func InspectCacheStaging(now time.Time) (CacheStaging, error) {
	files, err := cacheStagingFiles(now)
	if err != nil {
		return CacheStaging{}, err
	}
	usage := CacheStaging{files: files}
	for _, file := range files {
		usage.Count++
		usage.Bytes += file.bytes
	}
	return usage, nil
}

func (plan CacheStaging) Prune() (int, error) {
	removed := 0
	for _, file := range plan.files {
		ok, err := removeCacheStage(file)
		if ok {
			removed++
		}
		if err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func cacheStagingFiles(now time.Time) ([]workflowCacheFile, error) {
	root := CacheDir()
	var files []workflowCacheFile
	for _, dir := range []string{root, filepath.Join(root, "agent-icons"), filepath.Join(root, "icon-cache"), filepath.Join(root, "notify-icons")} {
		if err := CheckPrivateDirectory(dir); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		entries, err := boundedWorkflowDirectory(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			suffix, found := strings.CutPrefix(entry.Name(), privateTempPrefix)
			if !found || !randomLayoutSuffix(suffix) || !entry.Type().IsRegular() {
				continue
			}
			info, err := entry.Info()
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || stat.Nlink != 1 || checkOwnedRegular(info) != nil || info.Mode().Perm() != PrivateFile ||
				now.Sub(info.ModTime()) < 24*time.Hour {
				continue
			}
			files = append(files, workflowCacheFile{
				path: filepath.Join(dir, entry.Name()), id: layoutEntry{Dev: uint64(stat.Dev), Inode: uint64(stat.Ino)},
				bytes: info.Size(), modified: info.ModTime(),
			})
		}
	}
	return files, nil
}

func removeCacheStage(file workflowCacheFile) (bool, error) {
	parent, err := privateDirectory(filepath.Dir(file.path), false)
	if err != nil {
		return false, err
	}
	defer unix.Close(parent)
	name := filepath.Base(file.path)
	var stat unix.Stat_t
	if err := unix.Fstatat(parent, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return false, nil
		}
		return false, err
	}
	if (layoutEntry{Dev: uint64(stat.Dev), Inode: uint64(stat.Ino)}) != file.id ||
		stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o777 != 0o600 ||
		stat.Nlink != 1 || int(stat.Uid) != os.Geteuid() ||
		stat.Size != file.bytes || !time.Unix(stat.Mtim.Unix()).Equal(file.modified) {
		return false, nil
	}
	if err := unix.Unlinkat(parent, name, 0); err != nil {
		return false, err
	}
	return true, unix.Fsync(parent)
}
