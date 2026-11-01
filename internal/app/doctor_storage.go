package app

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

type storageFootprint struct {
	working, releases, reports int64
	largest                    string
	largestBytes               int64
	complete                   bool
}

// Measure retained deliverables separately. WalkDir never follows symlinks into
// other workspaces or shared dependencies; an unreadable path stays unknown.
func measureStateStorage(root string) storageFootprint {
	f := storageFootprint{complete: true}
	groups := map[string]int64{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == root && errors.Is(err, fs.ErrNotExist) {
				return nil // A new installation has no state yet.
			}
			f.complete = false
			return nil
		}
		if path == root && entry.Type()&fs.ModeSymlink != 0 {
			f.complete = false
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			f.complete = false
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			f.complete = false
			return nil
		}
		group := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		switch group {
		case "releases":
			f.releases += info.Size()
		case "reports":
			f.reports += info.Size()
		default:
			f.working += info.Size()
			groups[group] += info.Size()
		}
		return nil
	})
	if err != nil {
		f.complete = false
	}
	for name, size := range groups {
		if size > f.largestBytes || size == f.largestBytes && name < f.largest {
			f.largest, f.largestBytes = name, size
		}
	}
	return f
}

func (f storageFootprint) check() dcheck {
	label := i18n.Tr("gtmux disk usage", "gtmux 磁盘占用")
	note := fmt.Sprintf(i18n.Tr("Working state %s · releases and recovery %s · reports %s",
		"运行状态 %s · 发布与恢复 %s · 报告 %s"), humanBytes(f.working), humanBytes(f.releases), humanBytes(f.reports))
	status := stOK
	if f.working >= diskRedBytes {
		status = stMiss
	} else if f.working >= diskAmberBytes {
		status = stRec
	} else if f.releases+f.reports >= diskAmberBytes {
		status = stInfo
	}
	if f.working >= diskAmberBytes && f.largest != "" {
		note += fmt.Sprintf(i18n.Tr(" · largest working item: %s (%s)", " · 最大运行项目：%s（%s）"), f.largest, humanBytes(f.largestBytes))
	}
	if !f.complete {
		if status != stMiss {
			status = stRec
		}
		note += i18n.Tr(" · some paths could not be measured", " · 部分路径无法计量")
	}
	return dcheck{status, label, humanBytes(f.working + f.releases + f.reports), note}
}

func rowDiskUsage() dcheck { return measureStateStorage(state.Dir()).check() }
