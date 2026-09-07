// `gtmux limits` — real subscription-window remaining (5h session + weekly),
// via the cached `claude -p /usage` scrape (see openspec limits-watch). Also
// folded into `gtmux usage` / GET /api/usage as the `limits` block.
package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/limits"
)

// limitsReport is the CLI/API shape (thin wrapper over limits.Report).
func currentLimits(force bool) limits.Report {
	r, _ := limits.Get(limits.LoadConfig(), force, time.Now())
	return r
}

// cmdLimits implements `gtmux limits [--json] [--refresh]`.
func cmdLimits(args []string) int {
	jsonOut, force := false, false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--refresh":
			force = true
		case "-h", "--help":
			i18n.Say("usage: gtmux limits [--json] [--refresh]", "用法：gtmux limits [--json] [--refresh]")
			i18n.Say("  Real subscription-window usage (5h session + weekly), from the agent's",
				"  真实订阅窗口用量（5 小时会话 + 周额度），来自 agent 自己的")
			i18n.Say("  own `/usage` (cached; configure via ~/.config/gtmux/usage.json limits* keys).",
				"  `/usage`（有缓存；用 ~/.config/gtmux/usage.json 的 limits* 键配置）。")
			return 0
		default:
			i18n.Sae("gtmux limits: unknown option '"+a+"'", "gtmux limits: 未知选项 '"+a+"'")
			return 2
		}
	}
	r := currentLimits(force)
	if jsonOut {
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	if len(r.Windows) == 0 && len(r.Unknown) == 0 {
		i18n.Say("No subscription-window data (is `claude -p /usage` reachable? see usage.json limitsCommand).",
			"没有订阅窗口数据（`claude -p /usage` 能跑通吗？见 usage.json 的 limitsCommand）。")
		return 0
	}
	// CJK-safe, display-width-aware alignment — same primitives the digest
	// table uses, so every gtmux surface reads as one column-aligned system.
	labelWidth := 8
	for _, w := range r.Windows {
		if lw := i18n.DispWidth(w.Label); lw > labelWidth {
			labelWidth = lw
		}
	}
	for _, w := range r.Windows {
		line := fmt.Sprintf("● %s  %s %s", i18n.PadRight(w.Label, labelWidth),
			i18n.PadLeft(fmt.Sprintf("%d%%", w.PctUsed), 4), i18n.Tr("used", "已用"))
		if w.ResetAt != "" {
			line += "   " + i18n.Tr("resets ", "重置 ") + w.ResetAt
		}
		fmt.Println(line)
	}
	// An agent whose plan could not be read gets a LINE, not silence. A row that
	// disappears and a plan that is fine look the same, and a Codex reading rolls over
	// on its own the moment you stop using Codex for a day.
	for _, u := range r.Unknown {
		fmt.Println(unknownLine(u))
	}
	if r.Warn != "" {
		i18n.Say("⚠ near the weekly cap: "+r.Warn, "⚠ 接近周额度上限："+r.Warn)
	}
	if r.At > 0 {
		age := int(time.Since(time.Unix(r.At, 0)).Minutes())
		i18n.Say(fmt.Sprintf("(updated %dm ago)", age), fmt.Sprintf("（%d 分钟前更新）", age))
	}
	return 0
}

// unknownLine says why an agent's plan is missing, and what brings it back.
//
// The reason arrives as a key ("rolled-over") and is turned into a sentence here, so the
// wording lives in one place per language rather than inside the package that detected
// it. An unrecognised key still prints the agent — a surface that says nothing is the
// failure being fixed, so it must not be the fallback.
func unknownLine(u limits.UnknownPlan) string {
	switch u.Reason {
	case "rolled-over":
		return i18n.Tr(
			"○ "+u.Agent+"  the window it last reported has ended — "+u.Agent+" writes its plan into a session log, so one turn brings the figure back",
			"○ "+u.Agent+"  上次报告的窗口已经过去 —— "+u.Agent+" 把额度写在会话日志里，跑一轮就能重新读到")
	}
	return i18n.Tr("○ "+u.Agent+"  plan not readable right now", "○ "+u.Agent+"  当前读不到额度")
}
