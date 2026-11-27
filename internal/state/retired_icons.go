package state

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/assets"
)

// Known names alone are insufficient: only retire exact built-in bytes, with
// a single link and private ownership. Older app-derived thumbnails may differ.
type RetiredIcons struct{ files []workflowCacheFile }

func (p RetiredIcons) Count() int { return len(p.files) }

func InspectRetiredIcons() (RetiredIcons, error) {
	var result RetiredIcons
	if err := CheckStorageLayout(); err != nil {
		return result, err
	}
	queue := NotifyDir()
	if err := checkLayoutDir(queue); err == nil {
		entries, err := boundedWorkflowDirectory(queue)
		if err != nil {
			return result, err
		}
		if len(entries) > 0 {
			return result, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	for _, key := range assets.AgentIconKeys() {
		known := assets.AgentIcon(key)
		paths := []string{filepath.Join(CacheDir(), "notify-icons", key+".png")}
		if key == "claude" {
			paths = append(paths, IconPath())
		}
		for _, path := range paths {
			id, exists, err := workflowPathObject(path)
			if err != nil {
				return result, err
			}
			if !exists || id.Mode != 0 {
				continue
			}
			f, err := OpenPrivateRead(path)
			if err != nil {
				return result, err
			}
			info, err := f.Stat()
			if err != nil {
				f.Close()
				return result, err
			}
			if info.Size() != int64(len(known)) || info.Mode().Perm() != 0600 {
				f.Close()
				continue
			}
			data, err := io.ReadAll(io.LimitReader(f, int64(len(known)+1)))
			_ = f.Close()
			if err != nil {
				return result, err
			}
			if bytes.Equal(data, known) {
				result.files = append(result.files, workflowCacheFile{path: path, id: id, bytes: info.Size(), modified: info.ModTime()})
			}
		}
	}
	return result, nil
}

// Doctor supplies an offline writer check. Hold the queue lease while checking
// that no old request still needs these paths and while removing reviewed files.
func (p RetiredIcons) Prune(checkWriters func() error) (int, error) {
	if err := checkWriters(); err != nil {
		return 0, err
	}
	lease, err := TryLease("notify-queue")
	if err != nil {
		return 0, err
	}
	defer lease.Close()
	fresh, err := InspectRetiredIcons()
	if err != nil {
		return 0, err
	}
	have := make(map[workflowCacheFile]bool, len(fresh.files))
	for _, f := range fresh.files {
		have[f] = true
	}
	count := 0
	for _, f := range p.files {
		if have[f] {
			ok, err := removeCacheStage(f)
			if err != nil {
				return count, err
			}
			if ok {
				count++
			}
		}
	}
	if count > 0 {
		dir := filepath.Join(CacheDir(), "notify-icons")
		id, present, err := workflowPathObject(dir)
		if err != nil {
			return count, err
		}
		if present {
			items, err := boundedWorkflowDirectory(dir)
			if err != nil {
				return count, err
			}
			if len(items) == 0 {
				if err := removeRecoveryDirectory(dir, id); err != nil {
					return count, err
				}
			}
		}
	}
	return count, nil
}
