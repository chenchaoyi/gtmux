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
	backups, unclassified      int64
	largest                    string
	largestBytes               int64
	complete                   bool
	root                       string
}

// Measure retained deliverables separately. WalkDir never follows symlinks into
// other workspaces or shared dependencies; an unreadable path stays unknown.
func measureStateStorage(root string) storageFootprint {
	f := storageFootprint{complete: true, root: root}
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
		case "releases", "appstore":
			f.releases += info.Size()
		case "hq-snapshots":
			f.backups += info.Size()
		case "reports", "reviews", "verification":
			f.reports += info.Size()
		case "active", "waiting", "finished", "bg", "native", "resume", "logs", "cache", "events", "mine", "usage", "notify", "uploads", "locks", "codex-frontends", "codex-processes":
			f.working += info.Size()
			groups[group] += info.Size()
		default:
			f.unclassified += info.Size()
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
	note := fmt.Sprintf(i18n.Tr("Runtime %s · automatic backups %s · release recovery %s · verification archives %s · unclassified %s",
		"运行数据 %s · 自动备份 %s · 发布恢复 %s · 验收档案 %s · 待分类 %s"), humanBytes(f.working), humanBytes(f.backups), humanBytes(f.releases), humanBytes(f.reports), humanBytes(f.unclassified))
	status := stOK
	if f.working+f.unclassified >= diskRedBytes {
		status = stMiss
	} else if f.working+f.unclassified >= diskAmberBytes {
		status = stRec
	} else if f.releases+f.reports+f.backups >= diskAmberBytes {
		status = stInfo
	}
	if f.working+f.unclassified >= diskAmberBytes && f.largest != "" {
		note += fmt.Sprintf(i18n.Tr(" · largest runtime/unclassified item: %s (%s)", " · 较大的数据来源：%s（%s）"), f.largest, humanBytes(f.largestBytes))
	}
	if !f.complete {
		if status != stMiss {
			status = stRec
		}
		note += i18n.Tr(" · some paths could not be measured", " · 部分路径无法计量")
	}
	if f.root != "" {
		note += i18n.Tr(" · application directory: ", " · 应用目录：") + f.root
	}
	note += i18n.Tr(" · sources: hq-snapshots/; releases/, appstore/; reports/, reviews/, verification/ · backup policy: gtmux config snapshots", " · 来源：hq-snapshots/；releases/、appstore/；reports/、reviews/、verification/ · 备份规则：gtmux config snapshots")
	return dcheck{status, label, humanBytes(f.working + f.backups + f.releases + f.reports + f.unclassified), note}
}

func rowDiskUsage() dcheck { return measureStateStorage(state.Dir()).check() }
