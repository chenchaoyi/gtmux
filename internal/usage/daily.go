package usage

// The daily token ledger (usage-daily-totals, 2026-09-14).
//
// `gtmux usage` counted output per SESSION since that session began — a session three
// weeks old contributed three weeks, and the phone's "output so far" block had to carry a
// sentence explaining that it was not a billing period. The commander asked for the
// number people actually want: how many tokens all agents burned today, and this week.
//
// This ledger answers it from the same logs, attributed by the MESSAGE's own timestamp
// to the local day it happened, not the day gtmux read it. Every transcript on the
// machine that changed in the last eight days is read from a per-file byte watermark, so
// a call costs a stat per log plus whatever was appended; the first run reads those
// recent files whole (a bounded backfill). Cumulative logs (Codex) contribute the delta
// between consecutive totals. Concurrent writers (serve, a hook, the CLI) are serialised
// by a file lock, and the file is written whole and renamed.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// dailyKeepDays is how far back the ledger keeps days. Two months: long enough for
// "was last month like this", short enough that the file stays a few KB.
const dailyKeepDays = 366

// dailyScanDays is how far back a log's mtime may be for the ledger to keep reading it. A
// log untouched for longer has nothing new to attribute to a day we still report.
const dailyScanDays = 8

// DayTotal is one local day's tokens across every agent, with the per-agent split.
type DayTotal struct {
	Date    string           `json:"date"` // YYYY-MM-DD, local
	Out     int64            `json:"out"`
	In      int64            `json:"in"`
	ByAgent map[string]Count `json:"by_agent,omitempty"`
}

// Count is an output/input pair.
type Count struct {
	Out int64 `json:"out"`
	In  int64 `json:"in"`
}

// AgentHistory is one agent's today and this-week totals.
type AgentHistory struct {
	AgentKey  string `json:"agent_key"`
	AgentName string `json:"agent_name,omitempty"`
	TodayOut  int64  `json:"today_out"`
	WeekOut   int64  `json:"week_out"`
	TodayIn   int64  `json:"today_in"`
	WeekIn    int64  `json:"week_in"`
}

// History is what the surfaces show: the last seven local days oldest first (today
// last), the two headline sums, and the per-agent split of the week.
type History struct {
	Days     []DayTotal     `json:"days"`
	TodayOut int64          `json:"today_out"`
	WeekOut  int64          `json:"week_out"`
	TodayIn  int64          `json:"today_in"`
	WeekIn   int64          `json:"week_in"`
	ByAgent  []AgentHistory `json:"by_agent,omitempty"`
	// Activity is the ledger's whole window, as a heatmap reads it (usage-activity):
	// every day with output plus the figures a reader asks of a year — the total, the
	// peak, the streak. nil when the ledger knows no day yet.
	Activity *Activity `json:"activity,omitempty"`
	// When the ledger last read the logs. A surface can say "as of 20s ago".
	ScannedAt int64 `json:"scanned_at,omitempty"`
}

// DayOut is one day's output, the unit of the activity series. Days with no output are
// omitted from the series; a reader fills the calendar itself.
type DayOut struct {
	Date string `json:"date"` // YYYY-MM-DD, local
	Out  int64  `json:"out"`
}

// Activity is what a year of the ledger says at a glance. `Since` is the first day the
// ledger knows, so "all" is honest about its window rather than claiming a lifetime.
type Activity struct {
	Since      string   `json:"since"`
	Series     []DayOut `json:"series"`
	AllOut     int64    `json:"all_out"`
	PeakOut    int64    `json:"peak_out"`
	PeakDate   string   `json:"peak_date,omitempty"`
	Streak     int      `json:"streak"`      // consecutive days with output ending today, or yesterday while today is still empty
	BestStreak int      `json:"best_streak"` // the longest such run in the window
	ActiveDays int      `json:"active_days"` // days with any output
	DaysKnown  int      `json:"days_known"`  // days from Since through today
}

type fileMark struct {
	Offset  int64 `json:"offset"`
	LastOut int64 `json:"last_out,omitempty"` // cumulative logs: the last total seen
	LastIn  int64 `json:"last_in,omitempty"`
	MTime   int64 `json:"mtime"`
}

type dailyLedger struct {
	V         int                          `json:"v"`
	ScannedAt int64                        `json:"scanned_at"`
	Files     map[string]fileMark          `json:"files"`
	Days      map[string]map[string]*Count `json:"days"` // date → agent → count
}

func dailyPath() string { return filepath.Join(state.Dir(), "usage-daily.json") }

func loadDaily() dailyLedger {
	l := dailyLedger{V: 1, Files: map[string]fileMark{}, Days: map[string]map[string]*Count{}}
	b, err := os.ReadFile(dailyPath())
	if err != nil {
		return l
	}
	var on dailyLedger
	if json.Unmarshal(b, &on) != nil || on.V != 1 {
		return l
	}
	if on.Files == nil {
		on.Files = map[string]fileMark{}
	}
	if on.Days == nil {
		on.Days = map[string]map[string]*Count{}
	}
	return on
}

func saveDaily(l dailyLedger) {
	b, err := json.Marshal(l)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(dailyPath()), 0o755) != nil {
		return
	}
	tmp := dailyPath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, dailyPath())
	}
}

// UpdateDaily reads what the logs appended since the last call and attributes it to
// days. Safe to call from any process; a second caller waits on the lock.
func UpdateDaily(now time.Time) {
	if os.MkdirAll(state.Dir(), 0o755) != nil {
		return
	}
	lock, err := os.OpenFile(dailyPath()+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX) != nil {
		return
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	l := loadDaily()
	updateDaily(&l, now, transcript.LogGlobs())
	saveDaily(l)
}

// updateDaily is UpdateDaily without the lock and the file, for tests.
func updateDaily(l *dailyLedger, now time.Time, globs map[string][]string) {
	since := now.Add(-dailyScanDays * 24 * time.Hour).Unix()
	seen := map[string]bool{}
	for agent, pats := range globs {
		for _, pat := range pats {
			paths, _ := filepath.Glob(pat)
			for _, p := range paths {
				fi, err := os.Stat(p)
				if err != nil || fi.IsDir() || fi.ModTime().Unix() < since {
					continue
				}
				seen[p] = true
				mark := l.Files[p]
				if mark.MTime == fi.ModTime().Unix() && mark.Offset == fi.Size() {
					continue // untouched since the last read
				}
				if mark.Offset > fi.Size() {
					mark = fileMark{} // replaced or truncated: read it again from the top
				}
				grew, msgs := scanFrom(p, mark.Offset, fi.Size())
				for _, m := range msgs {
					if m.at.IsZero() {
						continue // a message with no time has no day
					}
					var out, in int64
					if m.cumulative {
						if m.totalOut < mark.LastOut || m.totalIn < mark.LastIn {
							mark.LastOut, mark.LastIn = 0, 0 // the log restarted its count
						}
						out, in = m.totalOut-mark.LastOut, m.totalIn-mark.LastIn
						mark.LastOut, mark.LastIn = m.totalOut, m.totalIn
					} else {
						out, in = m.out, m.in
					}
					day := m.at.Local().Format("2006-01-02")
					if l.Days[day] == nil {
						l.Days[day] = map[string]*Count{}
					}
					c := l.Days[day][agent]
					if c == nil {
						c = &Count{}
						l.Days[day][agent] = c
					}
					c.Out += out
					c.In += in
				}
				mark.Offset, mark.MTime = grew, fi.ModTime().Unix()
				l.Files[p] = mark
			}
		}
	}
	// Forget marks for logs that fell out of the window, and days past the keep.
	for p := range l.Files {
		if !seen[p] {
			delete(l.Files, p)
		}
	}
	keep := now.Add(-dailyKeepDays * 24 * time.Hour).Local().Format("2006-01-02")
	for d := range l.Days {
		if d < keep {
			delete(l.Days, d)
		}
	}
	l.ScannedAt = now.Unix()
}

// DailyHistory reads the ledger into the shape the surfaces show. `names` maps an agent
// key to its display label (the registry's), or nil.
func DailyHistory(now time.Time, names map[string]string) History {
	return dailyHistory(loadDaily(), now, names)
}

func dailyHistory(l dailyLedger, now time.Time, names map[string]string) History {
	h := History{ScannedAt: l.ScannedAt}
	byAgent := map[string]*AgentHistory{}
	today := now.Local().Format("2006-01-02")
	for i := 6; i >= 0; i-- {
		day := now.Local().AddDate(0, 0, -i).Format("2006-01-02")
		d := DayTotal{Date: day}
		if agents := l.Days[day]; len(agents) > 0 {
			d.ByAgent = map[string]Count{}
			for a, c := range agents {
				d.Out += c.Out
				d.In += c.In
				d.ByAgent[a] = *c
				ah := byAgent[a]
				if ah == nil {
					ah = &AgentHistory{AgentKey: a, AgentName: names[a]}
					byAgent[a] = ah
				}
				ah.WeekOut += c.Out
				ah.WeekIn += c.In
				if day == today {
					ah.TodayOut += c.Out
					ah.TodayIn += c.In
				}
			}
		}
		h.Days = append(h.Days, d)
		h.WeekOut += d.Out
		h.WeekIn += d.In
		if day == today {
			h.TodayOut, h.TodayIn = d.Out, d.In
		}
	}
	h.Activity = activityOf(l, now)
	for _, a := range byAgent {
		h.ByAgent = append(h.ByAgent, *a)
	}
	sort.Slice(h.ByAgent, func(i, j int) bool {
		if h.ByAgent[i].WeekOut != h.ByAgent[j].WeekOut {
			return h.ByAgent[i].WeekOut > h.ByAgent[j].WeekOut
		}
		return h.ByAgent[i].AgentKey < h.ByAgent[j].AgentKey
	})
	return h
}

// activityOf reads the whole ledger into the year-at-a-glance shape. Days are walked
// in calendar order from the first day the ledger knows through today, so a streak is
// counted across the gaps the series omits.
func activityOf(l dailyLedger, now time.Time) *Activity {
	if len(l.Days) == 0 {
		return nil
	}
	dates := make([]string, 0, len(l.Days))
	for d := range l.Days {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	first, err := time.ParseInLocation("2006-01-02", dates[0], time.Local)
	if err != nil {
		return nil
	}
	today := now.Local()
	todayKey := today.Format("2006-01-02")
	a := &Activity{Since: dates[0]}
	run, best := 0, 0
	for d := first; ; d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if key > todayKey {
			break
		}
		var out int64
		for _, c := range l.Days[key] {
			out += c.Out
		}
		a.DaysKnown++
		if out > 0 {
			a.Series = append(a.Series, DayOut{Date: key, Out: out})
			a.AllOut += out
			a.ActiveDays++
			if out > a.PeakOut {
				a.PeakOut, a.PeakDate = out, key
			}
			run++
			if run > best {
				best = run
			}
		} else if key != todayKey {
			// An empty day ends a run, except today, which is still being written.
			run = 0
		}
	}
	a.Streak, a.BestStreak = run, best
	return a
}
