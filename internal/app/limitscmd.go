// `gtmux limits` — real subscription-window remaining (5h session + weekly),
// via the cached `claude -p /usage` scrape (see openspec limits-watch). Also
// folded into `gtmux usage` / GET /api/usage as the `limits` block.
package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chenchaoyi/gtmux/internal/humanize"
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
			commandHelp("limits")
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
		if lw := i18n.DispWidth(limits.Name(w)); lw > labelWidth {
			labelWidth = lw
		}
	}
	// A bar per window, the word "used" said once in the heading rather than on every
	// row, and the same twelve cells `gtmux usage` draws so the two screens agree on
	// sight. Every row used to carry the same grey dot, which told a reader nothing
	// about which window was the tight one.
	fmt.Println(i18n.Dim + i18n.Tr("% used, and when the window comes back", "已用多少，以及窗口什么时候回来") + i18n.Reset)
	for _, w := range r.Windows {
		fmt.Println("  " + planRow(w, time.Now()))
	}
	// An agent whose plan could not be read gets a LINE, not silence. A row that
	// disappears and a plan that is fine look the same, and a Codex reading rolls over
	// on its own the moment you stop using Codex for a day.
	for _, u := range r.Unknown {
		fmt.Println(unknownLine(u))
	}
	// The closing line answers what the rows raise and do not: when the full one comes
	// back, and what still works until then. It used to restate the reading one line
	// under the reading.
	if line := capLine(r, time.Now()); line != "" {
		fmt.Println()
		fmt.Println("  " + line)
	}
	if r.At > 0 {
		age := int(time.Since(time.Unix(r.At, 0)).Minutes())
		if age <= 0 {
			i18n.Say(i18n.Dim+"read just now"+i18n.Reset, i18n.Dim+"刚读的"+i18n.Reset)
		} else {
			i18n.Say(fmt.Sprintf("%sread %dm ago%s", i18n.Dim, age, i18n.Reset),
				fmt.Sprintf("%s%d 分钟前读的%s", i18n.Dim, age, i18n.Reset))
		}
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
			"○ "+u.Agent+"  the window it last reported has ended. "+u.Agent+" writes its plan into its own log, so one turn brings the figure back",
			"○ "+u.Agent+"  上次报告的窗口已经过去，"+u.Agent+" 把额度写在自己的日志里，跑一轮就能重新读到")
	}
	return i18n.Tr("○ "+u.Agent+"  plan not readable right now", "○ "+u.Agent+"  当前读不到额度")
}

// capLine is the sentence under the windows. A window at its cap is the only thing worth
// a sentence, and the sentence a reader needs is not the percentage again: it is when
// that window comes back, and whether anything still works meanwhile.
func capLine(r limits.Report, now time.Time) string {
	var full, widest limits.Window
	for _, w := range r.Windows {
		if w.Tier == limits.TierFull && full.Label == "" {
			full = w
		}
		// The window that still has room, to say what keeps working: the all-models
		// week outranks a session window, which is short and comes back on its own.
		if w.Tier == "" && w.Kind == limits.KindWeekAll {
			widest = w
		}
	}
	if full.Label == "" {
		if r.Warn != "" {
			return i18n.Amber + fmt.Sprintf(i18n.Tr("%s is close to its cap.", "%s 快到上限了。"), r.Warn) + i18n.Reset
		}
		return ""
	}
	back := ""
	if full.ResetAt != "" {
		back = fmt.Sprintf(i18n.Tr(" until %s", "，%s 才回来"), full.ResetAt)
	} else if full.ResetUnix > now.Unix() {
		back = fmt.Sprintf(i18n.Tr(" for another %s", "，还要 %s"), humanize.AgeShort(full.ResetUnix-now.Unix()))
	}
	line := i18n.Amber + fmt.Sprintf(i18n.Tr("%s is spent%s.", "%s 用完了%s。"), limits.Name(full), back) + i18n.Reset
	if widest.Label != "" {
		line += i18n.Dim + fmt.Sprintf(i18n.Tr(" %s keeps answering; that window is at %d%%.", " %s 照常还能用，那个窗口用了 %d%%。"),
			agentDisplay(widest), widest.PctUsed) + i18n.Reset
	}
	return line
}

// agentDisplay is the agent's own name when the plan carried one, else its key.
func agentDisplay(w limits.Window) string {
	if w.AgentName != "" {
		return w.AgentName
	}
	return w.Agent
}
