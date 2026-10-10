package app

import (
	"regexp"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/panefocus"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

var paneIDRe = regexp.MustCompile(`^%[0-9]+`)

// paneRunsAgent reports whether a pane id is currently a live agent on the radar —
// the same classification the radar uses (title glyph + process subtree), so a pane
// whose agent has exited (now a plain shell) answers false even if a stale tmux
// title lingers. Overridable for tests.
var paneRunsAgent = func(paneID string) bool {
	for _, p := range radar.GatherAgents() {
		if p.PaneID == paneID {
			return true
		}
	}
	return false
}

// cmdFocus implements `gtmux focus <session|pane-id>`.
// A tmux pane id (%N) first selects that window+pane inside its session (so the
// session displays that exact pane), then its Ghostty tab is brought forward.
func cmdFocus(args []string) int {
	rc, attrs := focus(args)
	if target := focusTarget(args); target != "" {
		diag.DidRC("act.focus", target, rc, "brought a pane to the front", attrs...)
	}
	return rc
}

// focusTarget is what a focus call names: the pane or session, "--last", or the terminal
// app of a native jump. Help names nothing.
func focusTarget(args []string) string {
	for i, a := range args {
		switch a {
		case "-h", "--help":
			return ""
		case "--terminal":
			if i+1 < len(args) {
				return args[i+1]
			}
		case "--tab":
		default:
			if i == 0 || (args[i-1] != "--terminal" && args[i-1] != "--tab") {
				return a
			}
		}
	}
	return ""
}

func focus(args []string) (int, []any) {
	// Native-agent jump (DESIGN §7): gtmux focus --terminal <app> --tab <title>.
	var termApp, tabTitle string
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--terminal":
			if i+1 < len(args) {
				termApp = args[i+1]
				i++
			}
		case "--tab":
			if i+1 < len(args) {
				tabTitle = args[i+1]
				i++
			}
		default:
			rest = append(rest, args[i])
		}
	}
	if termApp != "" && tabTitle != "" {
		res, err := ghostty.FocusTerminalTab(termApp, tabTitle)
		attrs := []any{"driver", termApp, "stage", "native-tab", "result", res}
		switch {
		case res == "ok" || (err == nil && res == ""):
			return 0, attrs
		case res == "notfound":
			i18n.Sae("No tab titled '"+tabTitle+"' in "+termApp, "在 "+termApp+" 里没有标题为 '"+tabTitle+"' 的 tab")
			return 1, attrs
		default:
			i18n.Sae("AppleScript failed (needs Automation permission)", "AppleScript 执行失败（需要自动化权限）")
			if err != nil {
				attrs = append(attrs, "error", err.Error())
			}
			return 1, attrs
		}
	}
	args = rest

	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	switch target {
	case "-h", "--help":
		commandHelp("focus")
		return 0, nil
	case "":
		i18n.Sae("usage: gtmux focus <session|pane-id|--last>   (jump to that tab / exact pane)",
			"用法：gtmux focus <session|pane-id|--last>   （跳到那个 tab / 确切 pane）")
		return 2, nil
	case "--last", "-l":
		// Jump to the pane recorded in last-finished (the notification click target).
		last := state.ReadLastFinished()
		if last == "" {
			i18n.Sae("No recently-finished pane recorded yet", "还没有记录最近完成的 pane")
			return 1, []any{"stage", "last-finished"}
		}
		target = last
	}

	if paneIDRe.MatchString(target) {
		if tmux.Bin == "" || tmux.Display(target, "#{pane_id}") == "" {
			i18n.Sae("Pane "+target+" no longer exists", "pane "+target+" 已不存在")
			return 1, []any{"stage", "pane-missing"}
		}
		// The pane exists, but no coding agent may be running in it: one that exited
		// since the notification, or a pane that never ran one. Still jump (its screen
		// is often what you want to see), but say so. Say only what is known: nothing
		// here tells an exited agent from a pane that never had one, and the line used
		// to claim an exit for both (%12, 2026-10-06).
		if !paneRunsAgent(target) {
			i18n.Say("↪ "+target+": no coding agent is running here right now.",
				"↪ "+target+"：这里目前没有在运行的 coding agent。")
		}
		sess := tmux.Display(target, "#{session_name}")
		win := tmux.Display(target, "#{window_id}")
		if win != "" {
			tmux.OK("select-window", "-t", win)
		}
		tmux.OK("select-pane", "-t", target)
		target = sess // fall through to focus this session's Ghostty tab
	}
	if target == "" {
		i18n.Sae("could not resolve a session", "无法解析 session")
		return 1, []any{"stage", "session-resolve"}
	}

	// Resolve the terminal hosting THIS session, so focus lands on the right app
	// when sessions span multiple terminals (Ghostty + iTerm2). Active() would pick
	// whichever terminal hosts the first tmux client — wrong for a session elsewhere.
	term := terminal.ForSession(target)
	tn := term.Name()
	attrs := []any{"driver", tn, "session", target}
	// Nothing attached means there is no tab to focus — so OPEN one, rather than telling
	// the user to go run another command. This is the same call `gtmux new` and `restore`
	// make. (panefocus does this for the TUI/serve jump; the CLI has its own path and was
	// fixed second, after `gtmux focus %30` on a real detached session still printed the
	// old advice — the change had landed in one of the two places that jump.)
	if !panefocus.Attached(target) {
		if _, err := panefocus.BringForward(target); err != nil {
			i18n.Sae("could not open a "+tn+" tab for '"+target+"': "+err.Error(),
				"无法为 '"+target+"' 打开 "+tn+" 标签页："+err.Error())
			return 1, append(attrs, "stage", "open-tab", "error", err.Error())
		}
		i18n.Say("Nothing was showing '"+target+"', so gtmux opened a "+tn+" tab for it.",
			"之前没有窗口在显示 '"+target+"'，已为它打开一个 "+tn+" 标签页。")
		return 0, append(attrs, "stage", "open-tab")
	}
	res, err := term.FocusTab(target)
	attrs = append(attrs, "stage", "focus-tab", "result", res)
	switch {
	case res == "ok":
		return 0, attrs
	case err != nil || res == "":
		i18n.Sae("AppleScript failed. Needs "+tn+" and Automation permission",
			"AppleScript 执行失败：需要 "+tn+" 及自动化权限。")
		i18n.Sae("(System Settings → Privacy & Security → Automation → allow controlling "+tn+").",
			"（系统设置 → 隐私与安全性 → 自动化 → 允许控制 "+tn+"）。")
		if err != nil {
			attrs = append(attrs, "error", err.Error())
		}
		return 1, attrs
	default:
		i18n.Sae("No "+tn+" tab is showing session '"+target+"' (it may be detached).",
			"没有显示 session '"+target+"' 的 "+tn+" tab（可能尚未接回）。")
		i18n.Sae("Restore it with:  gtmux restore "+target, "接回它：  gtmux restore "+target)
		return 1, attrs
	}
}
