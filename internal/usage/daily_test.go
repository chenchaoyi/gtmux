package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func codexCount(ts string, totalIn, totalOut int64) string {
	return fmt.Sprintf(`{"timestamp":"%s","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"output_tokens":%d},"last_token_usage":{"input_tokens":100},"model_context_window":200000}}}`, ts, totalIn, totalOut)
}

// The ledger attributes each message to the local day it happened, a cumulative log
// contributes deltas, and reading twice counts once.
func TestDailyLedgerAttributesByMessageDay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TZ", "UTC")
	cl := filepath.Join(home, ".claude", "projects", "-x")
	cx := filepath.Join(home, ".codex", "sessions", "2026", "09", "14")
	for _, d := range []string{cl, cx} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	claudeLog := filepath.Join(cl, "a.jsonl")
	if err := os.WriteFile(claudeLog, []byte(
		asst("2026-09-13T09:00:00Z", 10, 1000, 0, 0)+"\n"+
			asst("2026-09-14T09:00:00Z", 20, 2000, 0, 0)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	codexLog := filepath.Join(cx, "rollout-2026-09-14-abc.jsonl")
	if err := os.WriteFile(codexLog, []byte(
		codexCount("2026-09-14T10:00:00Z", 50, 500)+"\n"+
			codexCount("2026-09-14T11:00:00Z", 80, 800)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	globs := map[string][]string{
		"claude": {filepath.Join(home, ".claude", "projects", "*", "*.jsonl")},
		"codex":  {filepath.Join(home, ".codex", "sessions", "*", "*", "*", "rollout-*.jsonl")},
	}
	l := dailyLedger{V: 1, Files: map[string]fileMark{}, Days: map[string]map[string]*Count{}}
	updateDaily(&l, now, globs)
	h := dailyHistory(l, now, map[string]string{"claude": "Claude Code", "codex": "Codex"})
	if h.TodayOut != 2800 || h.WeekOut != 3800 {
		t.Fatalf("today %d week %d, want 2800 / 3800", h.TodayOut, h.WeekOut)
	}
	if len(h.Days) != 7 || h.Days[6].Date != "2026-09-14" || h.Days[5].Out != 1000 {
		t.Fatalf("days = %+v", h.Days)
	}
	if len(h.ByAgent) != 2 || h.ByAgent[0].AgentKey != "claude" || h.ByAgent[0].WeekOut != 3000 ||
		h.ByAgent[1].AgentKey != "codex" || h.ByAgent[1].WeekOut != 800 || h.ByAgent[1].TodayOut != 800 ||
		h.ByAgent[0].AgentName != "Claude Code" {
		t.Fatalf("by agent = %+v", h.ByAgent)
	}

	// Nothing appended: the second read changes nothing.
	updateDaily(&l, now, globs)
	if h2 := dailyHistory(l, now, nil); h2.WeekOut != 3800 {
		t.Fatalf("a second read double-counted: %d", h2.WeekOut)
	}

	// An appended message lands on its own day; the codex total moving 800 → 1100 adds 300.
	f, _ := os.OpenFile(claudeLog, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString(asst("2026-09-14T11:30:00Z", 1, 100, 0, 0) + "\n")
	_ = f.Close()
	g, _ := os.OpenFile(codexLog, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = g.WriteString(codexCount("2026-09-14T11:40:00Z", 90, 1100) + "\n")
	_ = g.Close()
	// mtime must move for the ledger to look again; a same-second append is the case
	// the offset check covers.
	later := now.Add(time.Minute)
	_ = os.Chtimes(claudeLog, later, later)
	_ = os.Chtimes(codexLog, later, later)
	updateDaily(&l, later, globs)
	if h3 := dailyHistory(l, later, nil); h3.TodayOut != 3200 || h3.WeekOut != 4200 {
		t.Fatalf("after append: today %d week %d, want 3200 / 4200", h3.TodayOut, h3.WeekOut)
	}

	// A log rewritten shorter is read again from the top, not from a stale offset.
	if err := os.WriteFile(claudeLog, []byte(asst("2026-09-14T11:50:00Z", 1, 50, 0, 0)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later2 := later.Add(time.Minute)
	_ = os.Chtimes(claudeLog, later2, later2)
	updateDaily(&l, later2, globs)
	if mark := l.Files[claudeLog]; mark.Offset == 0 {
		t.Fatal("the rewritten log was not re-read")
	}
}

// A log untouched for longer than the window is forgotten; a message without a time has
// no day; days past the keep are pruned.
func TestDailyLedgerWindowAndPrune(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TZ", "UTC")
	cl := filepath.Join(home, ".claude", "projects", "-x")
	_ = os.MkdirAll(cl, 0o755)
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	old := filepath.Join(cl, "old.jsonl")
	_ = os.WriteFile(old, []byte(asst("2026-07-01T09:00:00Z", 1, 999, 0, 0)+"\n"), 0o644)
	stale := now.Add(-30 * 24 * time.Hour)
	_ = os.Chtimes(old, stale, stale)
	fresh := filepath.Join(cl, "fresh.jsonl")
	_ = os.WriteFile(fresh, []byte(`{"type":"assistant","message":{"role":"assistant","usage":{"output_tokens":5}}}`+"\n"+
		asst("2026-09-14T09:00:00Z", 1, 10, 0, 0)+"\n"), 0o644)
	globs := map[string][]string{"claude": {filepath.Join(home, ".claude", "projects", "*", "*.jsonl")}}
	l := dailyLedger{V: 1, Files: map[string]fileMark{}, Days: map[string]map[string]*Count{
		"2025-06-01": {"claude": &Count{Out: 7}},
	}}
	updateDaily(&l, now, globs)
	if _, seen := l.Files[old]; seen {
		t.Error("a log untouched for a month should not be read or remembered")
	}
	if _, kept := l.Days["2025-06-01"]; kept {
		t.Error("a day past the keep should be pruned")
	}
	h := dailyHistory(l, now, nil)
	if h.TodayOut != 10 {
		t.Fatalf("today = %d, want 10 (the timeless line has no day)", h.TodayOut)
	}
}

func TestActivityReadsTheWholeLedger(t *testing.T) {
	// The heatmap's year: every day with output, the total since the ledger's first day,
	// the peak, and a streak counted across the calendar (an empty day breaks it; today,
	// still being written, does not).
	l := dailyLedger{V: 1, Days: map[string]map[string]*Count{
		"2026-09-01": {"claude": {Out: 100}},
		"2026-09-02": {"claude": {Out: 300}, "codex": {Out: 50}},
		"2026-09-04": {"claude": {Out: 900}},
		"2026-09-05": {"claude": {Out: 200}},
	}}
	now := time.Date(2026, 9, 6, 9, 0, 0, 0, time.Local)
	a := dailyHistory(l, now, nil).Activity
	if a == nil {
		t.Fatal("no activity")
	}
	if a.Since != "2026-09-01" || a.DaysKnown != 6 || a.ActiveDays != 4 || a.AllOut != 1550 {
		t.Errorf("since %s known %d active %d all %d", a.Since, a.DaysKnown, a.ActiveDays, a.AllOut)
	}
	if a.PeakOut != 900 || a.PeakDate != "2026-09-04" {
		t.Errorf("peak %d on %s", a.PeakOut, a.PeakDate)
	}
	// Sep 4 and 5 ran; today (Sep 6) is empty but does not break the run yet.
	if a.Streak != 2 || a.BestStreak != 2 {
		t.Errorf("streak %d best %d", a.Streak, a.BestStreak)
	}
	if got := len(a.Series); got != 4 || a.Series[1].Out != 350 {
		t.Errorf("series = %+v", a.Series)
	}
	// Yesterday empty, the day before not: the current streak is over.
	if a2 := dailyHistory(l, time.Date(2026, 9, 7, 9, 0, 0, 0, time.Local), nil).Activity; a2.Streak != 0 || a2.BestStreak != 2 {
		t.Errorf("streak after a gap = %d best %d", a2.Streak, a2.BestStreak)
	}
	if dailyHistory(dailyLedger{V: 1, Days: map[string]map[string]*Count{}}, now, nil).Activity != nil {
		t.Error("an empty ledger has no activity")
	}
}
