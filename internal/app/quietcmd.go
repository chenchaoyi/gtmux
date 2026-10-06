// `gtmux quiet` — the surfacing-threshold front door (hq-attention-system §4). It
// tunes how high the bar sits for HQ to PRINT an item to the user: `on` = surface
// CRITICAL only, `off` = clear that flag, `status` = show the resolved threshold. Under
// the hood it sets `quiet` / clears it in config.json; with it clear the resolver
// (hqsurface.ResolveThreshold) falls back to config `surfaceTier`, then the NORMAL
// default, and env overrides any of them for one process. A feed
// DEGRADATION is never quieted — it is CRITICAL and always surfaces.
package app

import (
	"github.com/chenchaoyi/gtmux/internal/hqsurface"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// cmdQuiet implements `gtmux quiet [on|off|status]`.
func cmdQuiet(args []string) int {
	if len(args) == 0 || args[0] == "status" {
		return quietStatus()
	}
	switch args[0] {
	case "-h", "--help":
		return quietUsage()
	case "on":
		if err := setConfigKey("quiet", true); err != nil {
			i18n.Sae("gtmux quiet: "+err.Error(), "gtmux quiet: "+err.Error())
			return 1
		}
		return quietStatus()
	case "off":
		if err := setConfigKey("quiet", false); err != nil {
			i18n.Sae("gtmux quiet: "+err.Error(), "gtmux quiet: "+err.Error())
			return 1
		}
		return quietStatus()
	default:
		i18n.Sae("gtmux quiet: unknown option '"+args[0]+"'", "gtmux quiet: 未知选项 '"+args[0]+"'")
		return quietUsage()
	}
}

// quietStatus prints the resolved surfacing threshold.
func quietStatus() int {
	th := hqsurface.ResolveThreshold()
	on := "off"
	if hqsurface.QuietOn() {
		on = "on"
	}
	i18n.Say("quiet = "+on+"  ·  surface threshold = "+th+" and above",
		"安静模式 = "+on+"  ·  呈现阈值 = "+th+" 及以上")
	i18n.Say("  (CRITICAL always surfaces; a degraded feed is never quieted)",
		"  （CRITICAL 始终呈现；感知层降级不受安静模式压制）")
	return 0
}

func quietUsage() int {
	i18n.Say("usage: gtmux quiet [on|off|status]", "用法：gtmux quiet [on|off|status]")
	i18n.Say("  on      surface CRITICAL only, the quietest bar.",
		"  on      仅呈现 CRITICAL，最安静。")
	i18n.Say("  off     clear quiet: config surfaceTier applies, else NORMAL and above.",
		"  off     清掉 quiet：按配置里的 surfaceTier，没配置就是 NORMAL 及以上。")
	i18n.Say("  status  show the resolved threshold (env GTMUX_SURFACE_TIER/GTMUX_QUIET override).",
		"  status  显示当前生效阈值（环境变量 GTMUX_SURFACE_TIER/GTMUX_QUIET 可覆盖）。")
	return 0
}
