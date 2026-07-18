// `gtmux resource` — local machine resource watch (resource-watch): disk/memory/
// CPU snapshot, per-agent RSS/CPU attributed by pane process tree, and actionable
// reclaim candidates (heavy orphan processes no live pane owns). HQ weighs these
// when dispatching and, when severe, advises reclaim or holding new sessions.
package app

import (
	"encoding/json"
	"fmt"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resource"
)

// cmdResource implements `gtmux resource [--json]`.
func cmdResource(args []string) int {
	jsonOut := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			i18n.Say("usage: gtmux resource [--json]", "用法：gtmux resource [--json]")
			i18n.Say("  Local disk/memory/CPU, per-agent RSS/CPU, and reclaim candidates.",
				"  本机磁盘/内存/CPU、按 agent 归因的 RSS/CPU、可回收孤儿进程。")
			return 0
		default:
			i18n.Sae("gtmux resource: unknown option '"+a+"'", "gtmux resource: 未知选项 '"+a+"'")
			return 2
		}
	}
	rep := radar.CurrentResource()
	if jsonOut {
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	m := rep.Machine
	warn := ""
	if m.Warn != "" {
		warn = "   ⚠ " + m.Warn
	}
	fmt.Printf("%s %dGB free · %s %d%% free (%s) · %s %.2f×%d cores%s\n",
		i18n.Tr("disk", "磁盘"), m.DiskFreeGB,
		i18n.Tr("mem", "内存"), m.MemFreePct, memTierLabel(m.MemTier),
		i18n.Tr("load", "负载"), m.LoadRatio, m.NCPU, warn)
	if len(rep.Agents) > 0 {
		fmt.Println(i18n.Tr("per-agent (RSS · CPU):", "按 agent（RSS · CPU）："))
		for pane, u := range rep.Agents {
			fmt.Printf("  %-6s %dMB · %.1f%%\n", pane, u.RSSMB, u.CPU)
		}
	}
	if len(rep.Orphans) > 0 {
		fmt.Println(i18n.Tr("reclaim candidates (orphans no live agent owns):", "可回收（无 agent 归属的孤儿进程）："))
		for _, o := range rep.Orphans {
			tag := ""
			if o.Kind != "" {
				tag = " [" + o.Kind + "]"
			}
			fmt.Printf("  pid %d  %dMB · %.1f%%  %s%s\n    ↳ %s\n", o.PID, o.RSSMB, o.CPU, o.Comm, tag, o.Hint)
		}
	}
	return 0
}

// preflightResource warns (to stderr) when a machine resource is at its RED line
// before adding load (gtmux hq / new). Returns true when it warned. Never blocks.
func preflightResource() bool {
	m := radar.CurrentResource().Machine
	if resource.MachineTier(m) < resource.TierRed {
		return false
	}
	i18n.Sae("⚠ resource red line: "+m.Warn+" — consider reclaiming/holding before adding load.",
		"⚠ 资源红线："+m.Warn+" —— 建议先回收或暂缓,再新增负载。")
	return true
}

func memTierLabel(tier string) string {
	switch tier {
	case "critical":
		return i18n.Tr("critical", "临界")
	case "warn":
		return i18n.Tr("warn", "警戒")
	case "normal":
		return i18n.Tr("normal", "正常")
	default:
		return "-"
	}
}
