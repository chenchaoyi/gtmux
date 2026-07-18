// `gtmux usage` — the usage-watch fleet view CLI: renders the radar-assembled
// per-session token snapshots + per-agent-type rollup (radar.GatherUsage). The
// producer lives in internal/radar; this file is the command + rendering only.
package app

import (
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

// cmdUsage implements `gtmux usage [--json]`.
func cmdUsage(args []string) int {
	jsonOut := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			i18n.Say("usage: gtmux usage [--json]", "用法：gtmux usage [--json]")
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
	// Subscription windows (real remaining) — the headline "how much room is left".
	if len(rep.Limits.Windows) > 0 {
		parts := make([]string, 0, len(rep.Limits.Windows))
		for _, w := range rep.Limits.Windows {
			parts = append(parts, fmt.Sprintf("%s %d%%", w.Label, w.PctUsed))
		}
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
