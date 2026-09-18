// `gtmux usage` — the usage-watch fleet view CLI: renders the radar-assembled
// per-session token snapshots + per-agent-type rollup (radar.GatherUsage). The
// producer lives in internal/radar; this file is the command + rendering only.
package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/humanize"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/radar"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// cmdUsage implements `gtmux usage [--json] [--activity]`.
func cmdUsage(args []string) int {
	jsonOut, activity := false, false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--activity":
			activity = true
		case "-h", "--help":
			commandHelp("usage")
			return 0
		default:
			i18n.Sae("gtmux usage: unknown option '"+a+"'", "gtmux usage: 未知选项 '"+a+"'")
			return 2
		}
	}
	if jsonOut {
		b, err := radar.UsageJSONBytes()
		if err != nil {
			i18n.Sae("gtmux: "+err.Error(), "gtmux: "+err.Error())
			return 1
		}
		fmt.Println(string(b))
		return 0
	}
	rep := radar.GatherUsage()
	if activity {
		printActivity(rep.History, time.Now())
		return 0
	}
	if len(rep.Sessions) == 0 {
		i18n.Say("No conversation has usage data yet.", "还没有哪段对话带上用量数据。")
		return 0
	}
	printUsage(rep, time.Now())
	return 0
}

// printUsage draws the screen, in the order the questions get asked.
//
// It used to open on one line per session — fifteen of them on the machine this was
// written for — and end on the plan line, which is the one number local counting cannot
// produce and the only one that says whether you can keep going at all. So the plan leads
// now, the session list keeps its head and folds its tail, and the column words ("out ·
// ctx · /m", repeated on every row) are said once in a header.
//
// The header also carries the PERIOD, which nothing did: a session's own total can be
// larger than the whole week's, because the session is older than the week, and a reader
// meeting those two numbers with nothing to explain them concludes one of them is wrong.
func printUsage(rep radar.UsageReport, now time.Time) {
	const numsEnd = 62 // the three number columns end here; a warning sits to their right
	locW := 8
	for _, r := range rep.Sessions {
		if w := i18n.DispWidth(sessionHead(r)); w > locW {
			locW = w
		}
	}
	if locW > 24 {
		locW = 24
	}

	// PLAN first: the windows, with a bar, and when each one comes back.
	// Every window, not limits.Summary's one-per-agent pick: that reduction exists for
	// the single line this screen used to end on, and here the session window and the
	// weekly one answer different questions.
	wins := rep.Limits.Windows
	if len(wins) > 0 || len(rep.Limits.Unknown) > 0 {
		fmt.Println(i18n.Bold + i18n.Tr("PLAN", "额度") + i18n.Reset +
			i18n.Dim + i18n.Tr("   % used, and when the window comes back", "   已用多少，以及窗口什么时候回来") + i18n.Reset)
		for _, w := range wins {
			fmt.Println("  " + planRow(w, now))
		}
		// An agent with no readable plan is NAMED rather than left out: dropping it is
		// what made Codex look broken, its rows simply gone.
		for _, u := range rep.Limits.Unknown {
			fmt.Printf("  %s%s%s\n", i18n.Dim, i18n.PadRight(u.Agent, 12)+i18n.Tr("plan unreadable", "读不到额度"), i18n.Reset)
		}
		fmt.Println()
	}

	// CONVERSATIONS: the head of the list, then one line for the tail.
	hdr := i18n.PadLeft(i18n.Tr("out", "输出"), 8) + i18n.PadLeft("ctx", 7) + i18n.PadLeft(i18n.Tr("rate", "速率"), 7)
	title := i18n.Tr("CONVERSATIONS", "对话") + "  " + fmt.Sprint(len(rep.Sessions))
	fmt.Println(i18n.Bold + title + i18n.Reset +
		i18n.PadRight("", maxInt(1, numsEnd-i18n.DispWidth(title)-i18n.DispWidth(hdr))) + i18n.Dim + hdr + i18n.Reset)
	fmt.Println("  " + i18n.Dim + i18n.Tr("each conversation since it started", "每段对话自它开始以来") + i18n.Reset)
	shown, restTok := usageHead(rep.Sessions)
	for _, r := range shown {
		glyph, color, _ := statusStyle(r.Status)
		head := i18n.TruncDisp(sessionHead(r), locW)
		left := "  " + glyph + " " + i18n.PadRight(head, locW)
		nums := i18n.PadLeft(compact(r.Tok), 8) + i18n.PadLeft(fmt.Sprintf("%d%%", int(r.Ctx*100)), 7) + i18n.PadLeft(rateOf(r.Rate), 7)
		line := "  " + color + glyph + i18n.Reset + " " + i18n.PadRight(head, locW) +
			i18n.PadRight("", maxInt(1, numsEnd-i18n.DispWidth(left)-i18n.DispWidth(nums))) +
			i18n.PadLeft(compact(r.Tok), 8)
		ctx := i18n.PadLeft(fmt.Sprintf("%d%%", int(r.Ctx*100)), 7)
		if r.UsageWarn != "" {
			ctx = i18n.Amber + ctx + i18n.Reset
		}
		line += ctx + i18n.Dim + i18n.PadLeft(rateOf(r.Rate), 7) + i18n.Reset
		if r.UsageWarn != "" {
			line += i18n.Amber + "   ⚠ " + r.UsageWarn + i18n.Reset
		}
		fmt.Println(line)
	}
	if n := len(rep.Sessions) - len(shown); n > 0 {
		fmt.Printf("    %s%s%s\n", i18n.Dim,
			fmt.Sprintf(i18n.Tr("… %d more idle, %s between them", "… 另外 %d 段空闲对话，合计 %s"), n, compact(restTok)), i18n.Reset)
	}

	// TOTALS: today, this week, and the long view.
	h := rep.History
	if h.WeekOut > 0 || len(rep.Types) > 0 {
		fmt.Println()
		fmt.Println(i18n.Bold + i18n.Tr("TOTALS", "合计") + i18n.Reset +
			i18n.Dim + i18n.Tr("   every agent on this Mac", "   这台 Mac 上的全部 agent") + i18n.Reset)
	}
	if h.WeekOut > 0 {
		fmt.Printf("  %s%s\n", i18n.PadRight(i18n.Tr("today", "今天"), locW+2), i18n.PadLeft(compact(h.TodayOut), 8))
		line := fmt.Sprintf("  %s%s", i18n.PadRight(i18n.Tr("this week", "本周"), locW+2), i18n.PadLeft(compact(h.WeekOut), 8))
		if len(h.ByAgent) > 1 {
			var parts []string
			for _, a := range h.ByAgent {
				parts = append(parts, a.AgentKey+" "+compact(a.WeekOut))
			}
			line += i18n.Dim + "   " + strings.Join(parts, " · ") + i18n.Reset
		}
		fmt.Println(line)
		if a := h.Activity; a != nil && a.AllOut > 0 {
			since := a.Since
			if t, err := time.ParseInLocation("2006-01-02", a.Since, time.Local); err == nil {
				since = i18n.Tr(t.Format("Jan 2"), fmt.Sprintf("%d月%d日", int(t.Month()), t.Day()))
			}
			fmt.Printf("  %s%s%s%s%s\n", i18n.Dim, i18n.PadRight(i18n.Tr("since ", "自 ")+since, locW+2),
				i18n.PadLeft(compact(a.AllOut), 8),
				fmt.Sprintf(i18n.Tr("   busiest day %s · %d days running, best %d", "   最多的一天 %s · 连续 %d 天，最长 %d 天"),
					compact(a.PeakOut), a.Streak, a.BestStreak), i18n.Reset)
		}
	}
	for _, t := range rep.Types {
		if t.UsageWarn == "" {
			continue // the per-type rollup only earns a line when it is warning
		}
		fmt.Printf("  %s%s ⚠ %s%s\n", i18n.Amber, i18n.PadRight(t.AgentKey, locW+2), t.UsageWarn, i18n.Reset)
	}
}

// usageHead is the part of the session list worth printing in full: everything that is
// not idle, plus the heaviest idle sessions, capped. The tail is one line with its total,
// because fifteen rows of "0/m" is a list nobody reads to the end of.
func usageHead(rows []radar.UsageRow) (shown []radar.UsageRow, restTok int64) {
	const cap = 6
	for _, r := range rows {
		if len(shown) < cap && (r.Status != "idle" || r.UsageWarn != "" || len(shown) < 4) {
			shown = append(shown, r)
			continue
		}
		restTok += r.Tok
	}
	return shown, restTok
}

func sessionHead(r radar.UsageRow) string {
	if r.Loc != "" {
		return r.Loc
	}
	return r.Agent
}

// rateOf drops the unit on a session that is not burning anything: "0/m" spends three
// columns saying nothing is happening.
func rateOf(rate int64) string {
	if rate <= 0 {
		return "0"
	}
	return compact(rate) + "/m"
}

// planRow is one window: name, percent used, a bar, and when it comes back. The bar is
// the same twelve characters `gtmux limits` draws, so the two screens agree on sight.
func planRow(w limits.Window, now time.Time) string {
	tone := ""
	if w.Tier == limits.TierWarn || w.Tier == limits.TierFull {
		tone = i18n.Amber
	}
	name := i18n.PadRight(limits.Name(w), 26)
	pct := i18n.PadLeft(fmt.Sprintf("%d%%", w.PctUsed), 4)
	// When it comes back: the relative form first because that is the question, the
	// clock time the agent reported after it, because that is what you set an alarm by.
	back := ""
	if w.ResetUnix > now.Unix() {
		back = fmt.Sprintf(i18n.Tr("   back in %s", "   %s后回来"), humanize.AgeShort(w.ResetUnix-now.Unix()))
	}
	if w.ResetAt != "" {
		back += "  " + w.ResetAt
	}
	return name + tone + pct + i18n.Reset + " " + planBar(w.PctUsed, tone) + i18n.Dim + back + i18n.Reset
}

// planBar is twelve cells of how much of the window is gone.
func planBar(pct int, tone string) string {
	const width = 12
	filled := pct * width / 100
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return tone + strings.Repeat("█", filled) + i18n.Reset +
		i18n.Dim + strings.Repeat("░", width-filled) + i18n.Reset
}

// compact renders token counts like the warn strings do.
func compact(n int64) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// activityShades are the five steps of the heatmap, GitHub's greens in the 256-colour
// cube (the commander asked for that palette so the picture reads the same everywhere);
// without colour the density carries it.
var activityShades = [5]string{"\x1b[38;5;238m", "\x1b[38;5;22m", "\x1b[38;5;28m", "\x1b[38;5;34m", "\x1b[38;5;46m"}
var activityGlyphs = [5]string{"·", "░", "▒", "▓", "█"}

// printActivity draws the ledger's last weeks as a calendar heatmap, Monday to Sunday
// down, one column a week, months labelled above — `gtmux usage --activity`.
func printActivity(h uwatch.History, now time.Time) {
	a := h.Activity
	if a == nil || len(a.Series) == 0 {
		i18n.Say("no days on the ledger yet", "账本里还没有一天的记录")
		return
	}
	weeks := 26
	if w := os.Getenv("COLUMNS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && (n-4)/2 < weeks {
			weeks = (n - 4) / 2
		}
	}
	if weeks < 4 {
		weeks = 4
	}
	byDay := map[string]int64{}
	for _, d := range a.Series {
		byDay[d.Date] = d.Out
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
	dow := (int(today.Weekday()) + 6) % 7 // Monday = 0
	start := today.AddDate(0, 0, -dow-(weeks-1)*7)
	level := func(out int64) int {
		switch {
		case out == 0:
			return 0
		case out*4 <= a.PeakOut:
			return 1
		case out*2 <= a.PeakOut:
			return 2
		case out*4 <= a.PeakOut*3:
			return 3
		}
		return 4
	}
	fmt.Printf("%s   %s\n", i18n.Tr("Token activity", "Token 活动"), i18n.Tr(fmt.Sprintf("last %d weeks", weeks), fmt.Sprintf("最近 %d 周", weeks)))
	fmt.Printf("%s %s · %s %s · %s\n\n", i18n.Tr("all", "累计"), compact(a.AllOut), i18n.Tr("peak", "峰值"), compact(a.PeakOut),
		i18n.Tr(fmt.Sprintf("streak %dd (best %dd)", a.Streak, a.BestStreak), fmt.Sprintf("连续 %d 天（最长 %d 天）", a.Streak, a.BestStreak)))
	// Month labels: one where a week's Monday starts a new month, written into a
	// two-cells-a-week ruler so a wide label ("Jan", "3月") never overlaps the next.
	ruler := []rune(strings.Repeat(" ", weeks*2+2))
	lastM := time.Month(0)
	cursor := 0
	for w := 0; w < weeks; w++ {
		d := start.AddDate(0, 0, w*7)
		if d.Month() == lastM || (w == 0 && d.Day() > 7) {
			lastM = d.Month()
			continue
		}
		lastM = d.Month()
		label := i18n.Tr(d.Format("Jan"), fmt.Sprintf("%d月", int(d.Month())))
		col := w * 2
		if col < cursor {
			continue
		}
		lr := []rune(label)
		copy(ruler[col:], lr)
		cursor = col + i18n.DispWidth(label) + 1
	}
	months := "    " + strings.TrimRight(string(ruler), " ")
	fmt.Println(months)
	labels := [7]string{i18n.Tr("Mo", "一"), "  ", i18n.Tr("We", "三"), "  ", i18n.Tr("Fr", "五"), "  ", i18n.Tr("Su", "日")}
	for r := 0; r < 7; r++ {
		line := labels[r] + "  "
		for w := 0; w < weeks; w++ {
			d := start.AddDate(0, 0, w*7+r)
			if d.After(today) {
				line += "  "
				continue
			}
			lv := level(byDay[d.Format("2006-01-02")])
			if i18n.ColorEnabled() {
				line += activityShades[lv] + "■" + i18n.Reset + " "
			} else {
				line += activityGlyphs[lv] + " "
			}
		}
		fmt.Println(line)
	}
	fmt.Println()
	legend := "  " + i18n.Tr("Less", "少") + " "
	for lv := 0; lv < 5; lv++ {
		if i18n.ColorEnabled() {
			legend += activityShades[lv] + "■" + i18n.Reset + " "
		} else {
			legend += activityGlyphs[lv] + " "
		}
	}
	fmt.Println(legend + i18n.Tr("More", "多"))
}
