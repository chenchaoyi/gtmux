package diag

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

// The store bounds itself. It does not wait for serve: the first entry of a new day,
// written by whichever process gets there, runs the cleanup, and serve's slow tick runs
// it too. A Mac where serve never starts is still cleaned.

// Growth limits. The retention window and the total cap are settable in config.json
// (`logs.retainDays`, `logs.maxMB`); the in-day guards are not, because they exist for
// the day something loops.
const (
	DefaultRetainDays = 30
	DefaultMaxMB      = 100
	SegmentCap        = 20 << 20 // a day file past this starts a new segment
	DebugDropAt       = 50 << 20 // past this in one day, debug entries are dropped
)

// Limits are the store's retention settings.
type Limits struct {
	RetainDays int
	MaxBytes   int64
}

// LoadLimits reads the settings, falling back to the defaults.
func LoadLimits() Limits {
	var c struct {
		Logs struct {
			RetainDays int `json:"retainDays"`
			MaxMB      int `json:"maxMB"`
		} `json:"logs"`
	}
	_ = usercfg.Load(&c)
	l := Limits{RetainDays: DefaultRetainDays, MaxBytes: DefaultMaxMB << 20}
	if c.Logs.RetainDays > 0 {
		l.RetainDays = c.Logs.RetainDays
	}
	if c.Logs.MaxMB > 0 {
		l.MaxBytes = int64(c.Logs.MaxMB) << 20
	}
	return l
}

// dayFileRe matches a day's first file and its in-day segments: 2026-09-19.jsonl,
// 2026-09-19.1.jsonl.
var dayFileRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})(?:\.(\d+))?\.jsonl$`)

// A StoreFile is one file of the store.
type StoreFile struct {
	Path    string
	Day     string
	Segment int
	Size    int64
}

// Files lists the store's files, oldest day first and in segment order within a day.
func Files(dir string) []StoreFile {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []StoreFile
	for _, e := range ents {
		m := dayFileRe.FindStringSubmatch(e.Name())
		if m == nil || e.IsDir() {
			continue
		}
		seg, _ := strconv.Atoi(m[2])
		var size int64
		if fi, err := e.Info(); err == nil {
			size = fi.Size()
		}
		out = append(out, StoreFile{Path: filepath.Join(dir, e.Name()), Day: m[1], Segment: seg, Size: size})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].Segment < out[j].Segment
	})
	return out
}

func segmentPath(dir, day string, seg int) string {
	if seg == 0 {
		return DayFile(dir, day)
	}
	return filepath.Join(dir, day+"."+strconv.Itoa(seg)+".jsonl")
}

// segmentFor picks the file an entry goes to today: the last segment, or a new one when
// that has passed SegmentCap. Opening a new segment records which component and event
// filled the last one, once, so doctor can point at the loop rather than only the size.
// Past DebugDropAt, debug entries are dropped for the rest of the day. It returns "" to
// drop the entry.
func segmentFor(dir, day string, e Entry) string {
	var total int64
	last, lastSize := 0, int64(-1)
	for _, f := range Files(dir) {
		if f.Day != day {
			continue
		}
		total += f.Size
		last, lastSize = f.Segment, f.Size
	}
	if total >= DebugDropAt && e.Level == Debug.String() {
		markOnce(dir, day, "debug-dropped", func() {
			appendLine(segmentPath(dir, day, last), Entry{Level: Warn.String(), Component: "log",
				Kind: KindDiag, Event: "log.debug.dropped",
				Msg:   "debug entries dropped for the rest of the day: the store passed its daily ceiling",
				Attrs: map[string]any{"bytes": total}})
		})
		return ""
	}
	if lastSize < SegmentCap {
		return segmentPath(dir, day, last)
	}
	full := segmentPath(dir, day, last)
	next := segmentPath(dir, day, last+1)
	markOnce(dir, day, "runaway-"+strconv.Itoa(last+1), func() {
		comp, ev, n := dominant(full)
		appendLine(next, Entry{Level: Warn.String(), Component: "log", Kind: KindDiag,
			Event: "log.runaway", Msg: "a day file passed its cap; this is who filled it",
			Attrs: map[string]any{"file": filepath.Base(full), "component": comp, "event": ev, "sample_share": n}})
	})
	return next
}

// dominant samples the last megabyte of a full segment and returns the component and
// event that wrote most of it, with their share of the sample in percent.
func dominant(path string) (string, string, int) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", 0
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil && fi.Size() > 1<<20 {
		_, _ = f.Seek(fi.Size()-(1<<20), io.SeekStart)
	}
	b, _ := io.ReadAll(f)
	counts := map[string]int{}
	total := 0
	for _, line := range strings.Split(string(b), "\n") {
		var e struct{ Component, Event string }
		if json.Unmarshal([]byte(line), &e) != nil || e.Event == "" {
			continue
		}
		counts[e.Component+"\x00"+e.Event]++
		total++
	}
	best, bn := "", 0
	for k, n := range counts {
		if n > bn {
			best, bn = k, n
		}
	}
	if total == 0 {
		return "", "", 0
	}
	parts := strings.SplitN(best, "\x00", 2)
	return parts[0], parts[1], bn * 100 / total
}

// appendLine writes an entry the store itself produces, stamped now. It takes no lock:
// it runs inside write, which holds one, and a single O_APPEND write is atomic anyway.
func appendLine(path string, e Entry) {
	e.TS = now().Format(tsLayout)
	redactEntry(&e)
	line := encode(e)
	if line == nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(line)
	_ = f.Close()
}

// markOnce runs fn only for the first process to claim name for day.
func markOnce(dir, day, name string, fn func()) {
	f, err := os.OpenFile(filepath.Join(dir, "."+name+"-"+day), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_ = f.Close()
	fn()
}

// maybeCleanupForNewDay runs the cleanup once per day, in whichever process writes that
// day's first entry.
func maybeCleanupForNewDay(dir, day string) {
	if _, err := os.Stat(DayFile(dir, day)); err == nil {
		return
	}
	markOnce(dir, day, "cleaned", func() { cleanup(dir, LoadLimits(), day) })
}

// A Removal is one file the cleanup deleted.
type Removal struct {
	Path   string
	Bytes  int64
	Reason string // "expired" or "over-cap"
}

// Cleanup applies retention to the store now: days older than the window go, then the
// oldest days while the store is over its cap. Today is never removed. Each removal is
// recorded as an action. serve's slow tick and `gtmux doctor --fix` call this.
func Cleanup() (removed []Removal) {
	defer func() { _ = recover() }()
	writeMu.Lock()
	defer writeMu.Unlock()
	dir := state.LogsDir()
	return cleanup(dir, LoadLimits(), now().Format("2006-01-02"))
}

func cleanup(dir string, lim Limits, today string) []Removal {
	files := Files(dir)
	cutoff := now().AddDate(0, 0, -lim.RetainDays).Format("2006-01-02")
	var removed []Removal
	var keep []StoreFile
	for _, f := range files {
		if f.Day < cutoff && f.Day != today {
			if os.Remove(f.Path) == nil {
				removed = append(removed, Removal{Path: f.Path, Bytes: f.Size, Reason: "expired"})
			}
			continue
		}
		keep = append(keep, f)
	}
	var total int64
	for _, f := range keep {
		total += f.Size
	}
	for i := 0; total > lim.MaxBytes && i < len(keep); i++ {
		if keep[i].Day == today {
			break
		}
		if os.Remove(keep[i].Path) == nil {
			removed = append(removed, Removal{Path: keep[i].Path, Bytes: keep[i].Size, Reason: "over-cap"})
			total -= keep[i].Size
		}
	}
	pruneMarkers(dir, today)
	// Recorded straight into today's file: this runs under write's lock, so it cannot
	// go back through write.
	for _, r := range removed {
		appendLine(currentSegment(dir, today), Entry{Level: Info.String(), Component: "log",
			Kind: KindAct, Event: "act.cleanup", Actor: "system", Target: filepath.Base(r.Path),
			Outcome: OK, Msg: "removed an old log file",
			Attrs: map[string]any{"bytes": r.Bytes, "reason": r.Reason}})
	}
	return removed
}

// currentSegment is the file today's entries are going to.
func currentSegment(dir, day string) string {
	last := 0
	for _, f := range Files(dir) {
		if f.Day == day {
			last = f.Segment
		}
	}
	return segmentPath(dir, day, last)
}

// pruneMarkers removes the once-a-day markers of past days.
func pruneMarkers(dir, today string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		n := e.Name()
		if strings.HasPrefix(n, ".") && !strings.HasSuffix(n, today) {
			_ = os.Remove(filepath.Join(dir, n))
		}
	}
}

// OldestDay is the first day the store holds, or "" when it is empty.
func OldestDay(dir string) string {
	fs := Files(dir)
	if len(fs) == 0 {
		return ""
	}
	return fs[0].Day
}

// Size is the store's total size in bytes.
func Size(dir string) int64 {
	var n int64
	for _, f := range Files(dir) {
		n += f.Size
	}
	return n
}
