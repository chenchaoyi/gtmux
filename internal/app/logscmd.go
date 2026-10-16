package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// gtmux logs reads the local log store, as `log show` and `log stream` read the system's
// (openspec change `diagnostics`). Without it, reading the store means knowing the day
// files and in-day segments and reading raw JSON; with it, "what did the phone do today"
// and "why did that pairing fail" are one command, and --json gives an agent one stable
// interface instead of file paths.

type logsFilter struct {
	since, until time.Time
	components   map[string]bool
	minLevel     diag.Level
	actsOnly     bool
	actor        string
	event        string
}

func (f logsFilter) keep(e diag.Entry) bool {
	t := e.Time()
	if t.Before(f.since) || (!f.until.IsZero() && t.After(f.until)) {
		return false
	}
	if len(f.components) > 0 && !f.components[e.Component] {
		return false
	}
	if diag.ParseLevel(e.Level) < f.minLevel {
		return false
	}
	if f.actsOnly && e.Kind != diag.KindAct {
		return false
	}
	if f.actor != "" && e.Actor != f.actor && !strings.HasPrefix(e.Actor, f.actor+":") {
		return false
	}
	if f.event != "" {
		if ok, _ := path.Match(f.event, e.Event); !ok {
			return false
		}
	}
	return true
}

func cmdLogs(args []string) int {
	f := logsFilter{since: time.Now().Add(-time.Hour), minLevel: diag.Debug}
	follow, asJSON, stats := false, false, false
	access, before, limit := false, "", 100
	accessPageFlag := false
	next := func(i *int, name string) (string, bool) {
		if *i+1 >= len(args) {
			i18n.Sae("gtmux logs: "+name+" needs a value", "gtmux logs: "+name+" 需要一个值")
			return "", false
		}
		*i++
		return args[*i], true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			commandHelp("logs")
			return 0
		case a == "--since" || a == "--until":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			t, err := parseWhen(v, time.Now())
			if err != nil {
				i18n.Sae("gtmux logs: "+a+" takes a duration (30m, 2h, 3d) or a date (2026-09-19): "+v,
					"gtmux logs: "+a+" 要填时长（30m、2h、3d）或日期（2026-09-19）："+v)
				return 2
			}
			if a == "--since" {
				f.since = t
			} else {
				f.until = t
			}
		case a == "--component" || a == "-c":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			if f.components == nil {
				f.components = map[string]bool{}
			}
			for _, c := range strings.Split(v, ",") {
				f.components[strings.TrimSpace(c)] = true
			}
		case a == "--level":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			f.minLevel = diag.ParseLevel(v)
		case a == "--acts":
			f.actsOnly = true
		case a == "--actor":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			f.actor = v
		case a == "--event":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			f.event = v
		case a == "--access":
			access = true
		case a == "--before":
			accessPageFlag = true
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			before = v
		case a == "--limit":
			accessPageFlag = true
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 200 {
				i18n.Sae("gtmux logs: --limit must be 1..200", "gtmux logs: --limit 必须为 1..200")
				return 2
			}
			limit = n
		case a == "--follow" || a == "-f":
			follow = true
		case a == "--json":
			asJSON = true
		case a == "--stats":
			stats = true
		default:
			i18n.Sae("gtmux logs: unknown option '"+a+"'", "gtmux logs: 未知选项 '"+a+"'")
			return 2
		}
	}

	if access {
		if follow || stats || f.event != "" || len(f.components) > 0 || f.actsOnly || f.minLevel != diag.Debug || !f.until.IsZero() {
			i18n.Sae("gtmux logs --access: use --json, --since, --actor, --before and --limit only", "gtmux logs --access 仅支持 --json、--since、--actor、--before 和 --limit")
			return 2
		}
		since := time.Time{}
		for _, a := range args {
			if a == "--since" {
				since = f.since
				break
			}
		}
		page, err := diag.ReadAccess(diag.AccessQuery{Since: since, Before: before, Actor: f.actor, Limit: limit})
		if err != nil {
			i18n.Sae("Access history unavailable: "+err.Error(), "无法读取访问历史："+err.Error())
			return 1
		}
		if asJSON {
			if json.NewEncoder(os.Stdout).Encode(page) != nil {
				return 1
			}
			return 0
		}
		for _, e := range page.Entries {
			fmt.Printf("%s  %s · %s · %s · HTTP %d\n", e.At, e.Name, i18n.Tr(e.LabelEN, e.LabelZH)+e.Target, e.Outcome, e.Status)
			fmt.Printf("  %s · %s · %s\n", e.Actor, e.Platform, e.SourceIP)
		}
		if len(page.Entries) == 0 {
			i18n.Say("No access records in this page.", "本页没有访问记录。")
		}
		if page.Next != "" {
			i18n.Say("Earlier records: gtmux logs --access --before "+page.Next, "更早记录：gtmux logs --access --before "+page.Next)
		}
		return 0
	}
	if accessPageFlag {
		i18n.Sae("gtmux logs: --before and --limit require --access", "gtmux logs: --before 和 --limit 需与 --access 一起使用")
		return 2
	}
	if stats {
		return statsOnly(f, asJSON)
	}

	dir := state.LogsDir()
	entries := readLogStore(dir, f)
	multiDay := f.since.Format("2006-01-02") != time.Now().Format("2006-01-02")
	for _, e := range entries {
		printLogEntry(os.Stdout, e, asJSON, multiDay)
	}
	if !follow {
		if len(entries) == 0 && !asJSON {
			i18n.Say("No entries in that window. Widen it with --since 1d, or drop a filter.",
				"这段时间里没有记录。可以用 --since 1d 放宽范围，或者去掉筛选条件。")
		}
		return 0
	}
	return followLogStore(dir, f, asJSON)
}

// parseWhen reads "30m", "2h", "3d", "1w" as that long before now, or a date / RFC 3339
// time as that moment.
func parseWhen(v string, now time.Time) (time.Time, error) {
	v = strings.TrimSpace(v)
	if n := len(v); n > 1 {
		if num, err := strconv.Atoi(v[:n-1]); err == nil && num >= 0 {
			switch v[n-1] {
			case 'm':
				return now.Add(-time.Duration(num) * time.Minute), nil
			case 'h':
				return now.Add(-time.Duration(num) * time.Hour), nil
			case 'd':
				return now.AddDate(0, 0, -num), nil
			case 'w':
				return now.AddDate(0, 0, -7*num), nil
			}
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, v)
}

// readLogStore returns the entries the filter keeps, in time order, reading only the
// day files the window can touch.
func readLogStore(dir string, f logsFilter) []diag.Entry {
	from := f.since.Format("2006-01-02")
	to := "9999-12-31"
	if !f.until.IsZero() {
		to = f.until.Format("2006-01-02")
	}
	var out []diag.Entry
	for _, sf := range diag.Files(dir) {
		if sf.Day < from || sf.Day > to {
			continue
		}
		out = append(out, readEntries(sf.Path, 0, f)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time().Before(out[j].Time()) })
	return out
}

func readEntries(path string, offset int64, f logsFilter) []diag.Entry {
	fh, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer fh.Close()
	if offset > 0 {
		_, _ = fh.Seek(offset, io.SeekStart)
	}
	var out []diag.Entry
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, diag.MaxEntry*2), diag.MaxEntry*2)
	for sc.Scan() {
		var e diag.Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if f.keep(e) {
			out = append(out, e)
		}
	}
	return out
}

// followLogStore prints new entries as they are appended, moving to the next segment
// or the next day's file as the store does.
func followLogStore(dir string, f logsFilter, asJSON bool) int {
	f.until = time.Time{}
	offsets := map[string]int64{}
	for _, sf := range diag.Files(dir) {
		offsets[sf.Path] = sf.Size
	}
	for {
		time.Sleep(500 * time.Millisecond)
		today := time.Now().Format("2006-01-02")
		for _, sf := range diag.Files(dir) {
			if sf.Day < today && offsets[sf.Path] >= sf.Size {
				continue
			}
			if sf.Size <= offsets[sf.Path] {
				continue
			}
			for _, e := range readEntries(sf.Path, offsets[sf.Path], f) {
				printLogEntry(os.Stdout, e, asJSON, false)
			}
			offsets[sf.Path] = sf.Size
		}
	}
}

// printLogEntry renders one entry: raw JSON, or the line diag.Format builds.
func printLogEntry(w io.Writer, e diag.Entry, asJSON, withDate bool) {
	if asJSON {
		b, _ := json.Marshal(e)
		fmt.Fprintln(w, string(b))
		return
	}
	fmt.Fprintln(w, diag.Format(e, i18n.ColorEnabled(), withDate))
}
