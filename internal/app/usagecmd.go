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
			i18n.Say("usage: gtmux usage [--json] [--activity]", "用法：gtmux usage [--json] [--activity]")
			i18n.Say("  Token usage per agent session + per-type rollup, with threshold warnings.",
				"  每个 agent 会话的 token 用量 + 按类型汇总,含阈值预警。")
			i18n.Say("  Thresholds: ~/.config/gtmux/usage.json (per agent type; see docs/cli.md).",
				"  阈值:~/.config/gtmux/usage.json(按 agent 类型;见 docs/cli.md)。")
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
		i18n.Say("No sessions with usage data.", "没有带用量数据的会话。")
		return 0
	}
	// CJK-safe, display-width-aware column alignment (i18n.PadRight/PadLeft) —
	// same alignment primitives the digest table uses, so every gtmux surface
	// reads as one column-aligned system rather than ad hoc printf columns.
	nameWidth := 8
	for _, r := range rep.Sessions {
		head := r.Loc
		if head == "" {
			head = r.Agent
		}
		if w := i18n.DispWidth(head); w > nameWidth {
			nameWidth = w
		}
	}
	if nameWidth > 24 {
		nameWidth = 24
	}
	for _, r := range rep.Sessions {
		head := r.Loc
		if head == "" {
			head = r.Agent
		}
		glyph, color, _ := statusStyle(r.Status)
		line := fmt.Sprintf("%s%s%s %s  %s out · ctx %s · %s/m",
			color, glyph, i18n.Reset, i18n.PadRight(i18n.TruncDisp(head, nameWidth), nameWidth),
			i18n.PadLeft(compact(r.Tok), 7), i18n.PadLeft(fmt.Sprintf("%d%%", int(r.Ctx*100)), 4),
			i18n.PadLeft(compact(r.Rate), 6))
		if r.UsageWarn != "" {
			line += "   ⚠ " + r.UsageWarn
		}
		fmt.Println(line)
	}
	for _, t := range rep.Types {
		line := fmt.Sprintf("Σ %s  %s out · %s/m · %s", i18n.PadRight(i18n.TruncDisp(t.AgentKey, nameWidth), nameWidth),
			i18n.PadLeft(compact(t.Tok), 7), i18n.PadLeft(compact(t.Rate), 6),
			i18n.Pl(t.Sessions, i18n.Tr("session", "个会话")))
		if t.UsageWarn != "" {
			line += "   ⚠ " + t.UsageWarn
		}
		fmt.Println(line)
	}
	// Tokens by day (usage-daily-totals): the sum people ask for — today, this week —
	// across every agent, with the week's split by agent.
	if h := rep.History; h.WeekOut > 0 {
		line := fmt.Sprintf("Σ %s  %s out · %s %s out", i18n.PadRight(i18n.Tr("today", "今天"), nameWidth),
			i18n.PadLeft(compact(h.TodayOut), 7), i18n.Tr("this week", "本周"), compact(h.WeekOut))
		if len(h.ByAgent) > 1 {
			var parts []string
			for _, a := range h.ByAgent {
				parts = append(parts, a.AgentKey+" "+compact(a.WeekOut))
			}
			line += " (" + strings.Join(parts, " · ") + ")"
		}
		fmt.Println(line)
		// The year at a glance (usage-activity): the same figures the phone's and the
		// menu bar's heatmap carry, so the three surfaces agree on the numbers.
		if a := h.Activity; a != nil && a.AllOut > 0 {
			since := a.Since
			if t, err := time.ParseInLocation("2006-01-02", a.Since, time.Local); err == nil {
				since = i18n.Tr(t.Format("Jan 2"), fmt.Sprintf("%d月%d日", int(t.Month()), t.Day()))
			}
			fmt.Printf("Σ %s  %s %s · %s %s · %s\n", i18n.PadRight(i18n.Tr("all", "累计"), nameWidth),
				i18n.PadLeft(compact(a.AllOut), 7), i18n.Tr("since "+since, "自 "+since),
				i18n.Tr("peak", "峰值"), compact(a.PeakOut),
				i18n.Tr(fmt.Sprintf("streak %dd (best %dd)", a.Streak, a.BestStreak), fmt.Sprintf("连续 %d 天（最长 %d 天）", a.Streak, a.BestStreak)))
		}
	}
	// Subscription windows (real remaining) — the headline "how much room is left".
	// One window per plan: the full list is `gtmux limits`, and joining all of
	// them here ran past 100 characters once Codex added its own.
	sum := limits.Summary(rep.Limits.Windows)
	parts := make([]string, 0, len(sum)+len(rep.Limits.Unknown))
	for _, w := range sum {
		parts = append(parts, fmt.Sprintf("%s %d%%", limits.Name(w), w.PctUsed))
	}
	// An agent with no readable plan is NAMED here rather than left out. Dropping it is
	// what made Codex look broken: its rows simply stopped appearing, which is
	// indistinguishable from gtmux failing to read them. This line has a width budget,
	// so it only flags the gap — `gtmux limits` is where the reason is spelled out.
	for _, u := range rep.Limits.Unknown {
		parts = append(parts, u.Agent+" "+i18n.Tr("unknown", "未知"))
	}
	if len(parts) > 0 {
		fmt.Println(i18n.Tr("Plan  ", "额度  ") + strings.Join(parts, " · "))
	}
	return 0
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
