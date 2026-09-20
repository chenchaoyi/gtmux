package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// `gtmux logs --stats` answers the question a diagnostics screen asks before it shows
// anything: how much is kept here, for how long, and was any of it a problem. The menu
// bar app reads the --json form for its Diagnostics section, which is why the shape is
// stable rather than a formatted line it would have to parse back.

type logStats struct {
	Bytes       int64  `json:"bytes"`
	Files       int    `json:"files"`
	Oldest      string `json:"oldest,omitempty"`
	RetainDays  int    `json:"retainDays"`
	MaxBytes    int64  `json:"maxBytes"`
	WindowHours int    `json:"windowHours"`
	Entries     int    `json:"entries"`
	Warnings    int    `json:"warnings"`
	Errors      int    `json:"errors"`
	Debug       string `json:"debug,omitempty"`
}

// collectLogStats counts the window the filter already describes, so --stats answers
// about exactly the entries `gtmux logs` would have printed.
func collectLogStats(dir string, f logsFilter, now time.Time) logStats {
	lim := diag.LoadLimits()
	st := logStats{
		Bytes:      diag.Size(dir),
		Files:      len(diag.Files(dir)),
		Oldest:     diag.OldestDay(dir),
		RetainDays: lim.RetainDays,
		MaxBytes:   lim.MaxBytes,
		Debug:      diag.DebugSwitch(),
	}
	if h := int(now.Sub(f.since).Hours() + 0.5); h > 0 {
		st.WindowHours = h
	}
	for _, e := range readLogStore(dir, f) {
		st.Entries++
		switch diag.ParseLevel(e.Level) {
		case diag.Error:
			st.Errors++
		case diag.Warn:
			st.Warnings++
		}
	}
	return st
}

func printLogStats(st logStats, asJSON bool) {
	if asJSON {
		b, _ := json.Marshal(st)
		fmt.Println(string(b))
		return
	}
	if st.Files == 0 {
		i18n.Say("Nothing recorded yet. Every gtmux process writes here as it runs.",
			"还没有记录。gtmux 的每个进程运行时都会写到这里。")
		return
	}
	i18n.Say(
		fmt.Sprintf("%s, kept %d days or %s · oldest %s",
			humanBytes(st.Bytes), st.RetainDays, humanBytes(st.MaxBytes), st.Oldest),
		fmt.Sprintf("%s，保留 %d 天或 %s · 最早 %s",
			humanBytes(st.Bytes), st.RetainDays, humanBytes(st.MaxBytes), st.Oldest))
	i18n.Say(
		fmt.Sprintf("last %dh: %d entries, %d warnings, %d errors",
			st.WindowHours, st.Entries, st.Warnings, st.Errors),
		fmt.Sprintf("最近 %d 小时：%d 条，%d 条警告，%d 条错误",
			st.WindowHours, st.Entries, st.Warnings, st.Errors))
	if st.Debug != "" {
		i18n.Say("Recording extra detail: "+st.Debug+"  (`gtmux config debug off` stops it)",
			"正在多记一些细节："+st.Debug+"（`gtmux config debug off` 停掉）")
	}
}

// statsOnly runs the --stats path, so cmdLogs stays a parser.
func statsOnly(f logsFilter, asJSON bool) int {
	printLogStats(collectLogStats(state.LogsDir(), f, time.Now()), asJSON)
	return 0
}
