package hq

import (
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Housekeeping keeps gtmux's two roots private and free of what nothing reads any more.
// serve runs it on start and the disk-hygiene sweep runs it every 30 minutes (openspec
// change `diagnostics`, design 3).

// retiredFiles are files under the data root that no code reads: the perception
// feed's spool and its bookkeeping, left behind when retire-perception-spool removed
// the layer, and the hook's and restore's own logs, whose entries now go to the store.
// hq-feed/ itself stays, because last-distill and last-self-check in it are live
// cadence markers.
var retiredFiles = []string{
	filepath.Join("hq-feed", "spool.jsonl"),
	filepath.Join("hq-feed", "cursor"),
	filepath.Join("hq-feed", "heartbeat"),
	filepath.Join("hq-feed", "restart-fails"),
	"hook.log",
	"restore.log",
}

// movedCaches are the icon caches that now live under cache/. The old ones are deleted
// rather than moved: they regrow on the next render.
var movedCaches = []string{"icon-cache", "agent-icons", "notify-icons"}

// emptyRetiredDirs are directories that are retired only when empty.
var emptyRetiredDirs = []string{"briefs"}

// Housekeep narrows both roots, removes retired files and runs the log store's
// retention. Every change is recorded in the log.
func Housekeep() {
	lg := diag.For("hygiene")
	for _, root := range []string{state.Dir(), state.ConfigDir()} {
		if changed := state.Narrow(root); len(changed) > 0 {
			lg.Act("act.narrow", "system", root, diag.OK,
				"removed group and other permissions under a gtmux root",
				"count", len(changed), "example", changed[0].Path)
		}
	}
	base := state.Dir()
	for _, rel := range retiredFiles {
		p := filepath.Join(base, rel)
		if fi, err := os.Stat(p); err == nil && os.Remove(p) == nil {
			lg.Act("act.cleanup", "system", rel, diag.OK, "removed a retired file",
				"bytes", fi.Size(), "reason", "retired")
		}
	}
	for _, name := range movedCaches {
		p := filepath.Join(base, name)
		if _, err := os.Stat(p); err == nil {
			n := dirBytes(p)
			if os.RemoveAll(p) == nil {
				lg.Act("act.cleanup", "system", name, diag.OK, "removed an icon cache that moved to cache/",
					"bytes", n, "reason", "moved")
			}
		}
	}
	for _, name := range emptyRetiredDirs {
		_ = os.Remove(filepath.Join(base, name)) // succeeds only when empty
	}
	diag.Cleanup()
}

// RetiredPresent lists the retired files and moved caches still on disk, for doctor.
func RetiredPresent() []string {
	var out []string
	base := state.Dir()
	for _, rel := range append(append([]string{}, retiredFiles...), movedCaches...) {
		if _, err := os.Stat(filepath.Join(base, rel)); err == nil {
			out = append(out, rel)
		}
	}
	return out
}

func dirBytes(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, e := d.Info(); e == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}
