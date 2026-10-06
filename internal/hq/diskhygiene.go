// Disk hygiene (resource-watch): gtmux bounds its OWN on-disk footprint. The two paths
// that actually grow without limit are (a) the always-on LaunchAgent logs
// `~/.local/share/gtmux/{serve,tunnel,selftunnel}.log` — launchd's StandardOut/ErrPath,
// which launchd NEVER rotates — and (b) the phone-upload sink `uploads/`, written on every
// /api/upload and never trimmed. (The event journal + feed spool are already bounded by
// their own rotation, so they are deliberately NOT touched here.) A time-gated sweep on the
// serve slow-tick caps the logs to a recent tail and prunes the uploads dir by age + size.
package hq

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const (
	// logMaxBytes / logKeepBytes cap a launchd log: over 8 MB it is trimmed to its last
	// ~2 MB, so the file stays bounded while the O_APPEND writer keeps appending after it.
	logMaxBytes  int64 = 8 << 20
	logKeepBytes int64 = 2 << 20

	// uploadsMaxAge / uploadsMaxTotal bound the phone-upload sink: drop anything older than
	// 7 days, then, if still over 200 MB, delete oldest-first until under the cap.
	uploadsMaxAge         = 7 * 24 * time.Hour
	uploadsMaxTotal int64 = 200 << 20

	// markerMaxAge ages out the per-pane EPHEMERAL churn markers of DEAD panes. A pane
	// tmux still has keeps its markers whatever their age: frame and cpu are rewritten
	// each sample, but goalchanged is written only when a goal changes and sends only when
	// something is sent, so a live pane quiet for two weeks lost them to the mtime alone
	// (%12, 2026-10-06). Only the mtime decides when tmux cannot be read.
	markerMaxAge = 14 * 24 * time.Hour

	// noSizeCap disables pruneDir's size trimming so it only ages entries out (used for the
	// marker dirs, which have no size budget — just a staleness cutoff).
	noSizeCap int64 = 1 << 62

	// hygieneInterval throttles the sweep — housekeeping, not a per-tick concern.
	hygieneInterval int64 = 30 * 60
)

// churnMarkerDirs are the per-pane ephemeral marker dirs that accumulate a file per pane
// and never clean up a dead pane's leftover. Deliberately NOT resume/usage/usagewarn —
// those back the digest + idle-since computation and must not be aged out.
var churnMarkerDirs = []string{"frame", "cpu", "goalchanged", "sends"}

// LaunchdCaptures are the launchd StandardOut/ErrPath redirects, which launchd never
// rotates: the ones under logs/ and, until each plist is regenerated, the legacy names in
// the data root.
func LaunchdCaptures() []string {
	out, _ := filepath.Glob(filepath.Join(state.LogsDir(), "*.stderr"))
	for _, name := range state.LegacyCaptures {
		out = append(out, filepath.Join(state.Dir(), name))
	}
	return out
}

// hygieneLastPath records the last hygiene sweep time (unix seconds) so the sweep runs at
// most once per hygieneInterval even though the slow-tick fires every 20 s.
func hygieneLastPath() string { return filepath.Join(state.Dir(), "disk-hygiene-last") }

// diskHygieneSweep caps the never-rotated launchd logs and prunes the uploads dir. It is
// time-gated (≤ 1/30 min), best-effort (every step tolerates a missing path / I/O error),
// and silent — housekeeping emits no HQ nudge.
func diskHygieneSweep(now int64) {
	last, _ := strconv.ParseInt(state.ReadMarker(hygieneLastPath()), 10, 64)
	if now-last < hygieneInterval {
		return
	}
	_ = state.WriteInt64Marker(hygieneLastPath(), now)

	base := state.Dir()
	nowT := time.Unix(now, 0)
	// 1) launchd daemon logs — unrotated StandardOut/ErrPath redirects. Cap to the tail.
	for _, p := range LaunchdCaptures() {
		_ = trimFileTail(p, logMaxBytes, logKeepBytes)
	}
	// 2) phone uploads — never pruned. Age out old files, then LRU-trim to a size budget.
	_ = pruneDir(filepath.Join(base, "uploads"), uploadsMaxAge, uploadsMaxTotal, nowT, nil)
	// 3) per-pane ephemeral churn markers of DEAD panes — age them out (size uncapped).
	live := livePanes()
	for _, name := range churnMarkerDirs {
		_ = pruneDir(filepath.Join(base, name), markerMaxAge, noSizeCap, nowT, func(file string) bool {
			return live[strings.TrimSuffix(file, ".json")] // "%12", or the send record "%12.json"
		})
	}
	// 4) privacy and the log store: other processes write under the roots with their own
	// umask (the HQ agent writes its board), so modes are re-narrowed every sweep.
	Housekeep()
}

// trimFileTail caps path at maxBytes by rewriting it to keep only its last keepBytes
// (keepBytes should be ≤ maxBytes), dropping the partial first line so the retained head
// starts clean. It is a no-op when the file is under the cap or missing. Best-effort: an
// O_APPEND writer (launchd) keeps appending after the retained tail; at most one write
// racing the rewrite is lost, which is acceptable for a log.
func trimFileTail(path string, maxBytes, keepBytes int64) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Size() <= maxBytes {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(-keepBytes, io.SeekEnd); err != nil {
		return err
	}
	tail, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	// Drop the partial leading line so the retained head begins at a record boundary.
	if i := bytes.IndexByte(tail, '\n'); i >= 0 && i+1 <= len(tail) {
		tail = tail[i+1:]
	}
	return os.WriteFile(path, tail, 0o600)
}

// livePanes is the set of panes tmux has; empty when it cannot be read. A variable so a
// test can stand in for tmux.
var livePanes = tmux.LivePaneIDs

// pruneDir bounds a directory: it deletes top-level entries older than maxAge (by mtime),
// then, if the surviving entries still total more than maxTotal bytes, deletes them
// oldest-first until under the cap. An entry keep reports true for is left alone (nil
// keeps nothing). A no-op for a missing/empty dir. Best-effort; it does not recurse (the
// uploads sink is flat).
func pruneDir(dir string, maxAge time.Duration, maxTotal int64, now time.Time, keep func(name string) bool) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	type item struct {
		path string
		size int64
		mod  time.Time
	}
	var kept []item
	for _, e := range ents {
		if e.IsDir() {
			continue // flat sink; never touch nested dirs
		}
		if keep != nil && keep(e.Name()) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if now.Sub(fi.ModTime()) > maxAge {
			_ = os.Remove(p) // aged out
			continue
		}
		kept = append(kept, item{path: p, size: fi.Size(), mod: fi.ModTime()})
	}
	var total int64
	for _, it := range kept {
		total += it.size
	}
	if total <= maxTotal {
		return nil
	}
	// Over the size cap: delete oldest-first until under it.
	sort.Slice(kept, func(i, j int) bool { return kept[i].mod.Before(kept[j].mod) })
	for _, it := range kept {
		if total <= maxTotal {
			break
		}
		if os.Remove(it.path) == nil {
			total -= it.size
		}
	}
	return nil
}
