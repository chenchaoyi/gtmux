package app

import (
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// agentsSummary renders "N agents · [X waiting ·] Y working · Z idle".
func agentsSummary(panes []radar.Pane) string {
	s := i18n.Pl(len(panes), "agent")
	if len(panes) == 0 {
		return s
	}
	var nWork, nWait int
	for _, p := range panes {
		switch p.Status {
		case "working":
			nWork++
		case "waiting":
			nWait++
		}
	}
	parts := []string{}
	if nWait > 0 {
		parts = append(parts, fmt.Sprintf(i18n.Tr("%d waiting", "%d 等输入"), nWait))
	}
	parts = append(parts, fmt.Sprintf(i18n.Tr("%d working", "%d 运行中"), nWork))
	parts = append(parts, fmt.Sprintf(i18n.Tr("%d idle", "%d 空闲"), len(panes)-nWork-nWait))
	return s + " · " + strings.Join(parts, " · ")
}

// cmdAgents implements `gtmux agents [--watch] [--popup] [--json]`.
func cmdAgents(args []string) int {
	watch, popup, asJSON := false, false, false
	for _, a := range args {
		switch a {
		case "-h", "--help":
			usage()
			return 0
		case "--watch", "-w":
			watch = true
		case "--popup":
			popup = true // close the TUI after a jump (used by the prefix+a popup)
		case "--json":
			asJSON = true
		}
	}
	if !tmux.ServerUp() {
		if asJSON {
			fmt.Println("[]")
			return 0
		}
		i18n.Say("No tmux server running", "没有运行中的 tmux server")
		return 1
	}
	if asJSON {
		return agentsJSON()
	}
	if watch {
		return runWatch(popup)
	}

	panes := radar.GatherAgents()
	fmt.Printf("%sgtmux %s%s — %s\n\n", i18n.Bold, i18n.Tr("agents", "agent"), i18n.Reset, agentsSummary(panes))
	if len(panes) == 0 {
		i18n.Say("No coding-agent panes found.", "没有发现 coding-agent 的 pane。")
		return 0
	}
	for _, p := range panes {
		glyph, color, label := statusStyle(p.Status)
		task := p.Task
		if task == "" {
			task = i18n.Dim + "—" + i18n.Reset
		}
		// errored idle: an amber ⚠ modifier (NOT red — red is waiting), and surface
		// the failure summary as the task so "ended on an error" is visible at a glance.
		if p.Errored {
			glyph, color, label = "⚠", i18n.Amber, i18n.Tr("errored", "报错")
			if p.ErrorText != "" {
				task = i18n.Amber + p.ErrorText + i18n.Reset
			}
		}
		// background-running idle: keep the green ✓ idle glyph (it IS idle), but flag
		// with an amber ⧗ modifier (NOT red) that background work is still in flight.
		if p.Bg && !p.Errored {
			n := ""
			if p.BgCount > 1 {
				n = fmt.Sprintf("%d", p.BgCount)
			}
			mark := i18n.Amber + "⧗" + n + " " + i18n.Tr("background running", "后台运行中") + i18n.Reset
			if p.BgText != "" {
				task = mark + i18n.Dim + " · " + p.BgText + i18n.Reset
			} else {
				task = mark
			}
		}
		dot := ""
		if p.Activity {
			dot = i18n.Yellow + " •" + i18n.Reset
		}
		done := ""
		if p.Latest {
			done = i18n.Yellow + i18n.Tr("  ✓ latest", "  ✓ 最近完成") + i18n.Reset
		}
		fmt.Printf("%s%s%s %s%s%s %s%s%s %s%s%s %s%s%s%s\n",
			color, glyph, i18n.Reset,
			color, i18n.PadRight(label, 8), i18n.Reset,
			i18n.Bold, i18n.PadRight(p.Agent, 12), i18n.Reset,
			i18n.Bold, i18n.PadRight(p.Loc, 22), i18n.Reset,
			task, dot, i18n.Dim+" "+p.PaneID+i18n.Reset, done)
	}
	fmt.Printf("\n%s%s%s\n", i18n.Dim,
		i18n.Tr("jump: gtmux focus <pane>   (e.g. gtmux focus "+panes[0].PaneID+")",
			"跳转：gtmux focus <pane>   （例如 gtmux focus "+panes[0].PaneID+"）"), i18n.Reset)
	return 0
}

// agentsJSON prints the live agents as a JSON array (stable shape; no colors,
// no screen-scraping — for scripts and the menu-bar app).
func agentsJSON() int {
	b, err := radar.AgentsJSONBytes()
	if err != nil {
		i18n.Sae("json error: "+err.Error(), "json 错误："+err.Error())
		return 1
	}
	fmt.Println(string(b))
	return 0
}

func statusStyle(status string) (glyph, color, label string) {
	switch status {
	case "working":
		return "⠿", i18n.Cyan, i18n.Tr("working", "运行中")
	case "waiting":
		return "⏸", i18n.Yellow, i18n.Tr("waiting", "等输入")
	case "idle":
		return "✳", i18n.Green, i18n.Tr("idle", "空闲")
	default:
		return "●", i18n.Yellow, i18n.Tr("running", "运行中")
	}
}
