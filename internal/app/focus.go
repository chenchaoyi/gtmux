package app

import (
	"regexp"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

var paneIDRe = regexp.MustCompile(`^%[0-9]+`)

// jumpPane selects a pane's window+pane in tmux and brings its Ghostty tab
// forward (no output). Used by the watch TUI on Enter.
func jumpPane(paneID string) {
	if tmux.Bin == "" || tmux.Display(paneID, "#{pane_id}") == "" {
		return
	}
	sess := tmux.Display(paneID, "#{session_name}")
	if win := tmux.Display(paneID, "#{window_id}"); win != "" {
		tmux.OK("select-window", "-t", win)
	}
	tmux.OK("select-pane", "-t", paneID)
	if sess != "" {
		ghostty.FocusTab(sess)
	}
}

// cmdFocus implements `gtmux focus <session|pane-id>`.
// A tmux pane id (%N) first selects that window+pane inside its session (so the
// session displays that exact pane), then its Ghostty tab is brought forward.
func cmdFocus(args []string) int {
	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	switch target {
	case "-h", "--help":
		usage()
		return 0
	case "":
		i18n.Sae("usage: gtmux focus <session|pane-id|--last>   (jump to that tab / exact pane)",
			"用法: gtmux focus <session|pane-id|--last>   (跳到那个 tab / 确切 pane)")
		return 2
	case "--last", "-l":
		// Jump to the pane recorded in last-finished (GtmuxFocus.app's click target).
		last := state.ReadLastFinished()
		if last == "" {
			i18n.Sae("No recently-finished pane recorded yet", "还没有记录最近完成的 pane")
			return 1
		}
		target = last
	}

	if paneIDRe.MatchString(target) {
		if tmux.Bin == "" || tmux.Display(target, "#{pane_id}") == "" {
			i18n.Sae("Pane "+target+" no longer exists", "pane "+target+" 已不存在")
			return 1
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
		return 1
	}

	res, err := ghostty.FocusTab(target)
	switch {
	case res == "ok":
		return 0
	case err != nil || res == "":
		i18n.Sae("AppleScript failed. Needs Ghostty 1.3+ and Automation permission",
			"AppleScript 执行失败:需要 Ghostty 1.3+ 及自动化权限。")
		i18n.Sae("(System Settings → Privacy & Security → Automation → allow controlling Ghostty).",
			"(系统设置 → 隐私与安全性 → 自动化 → 允许控制 Ghostty)。")
		return 1
	default:
		i18n.Sae("No Ghostty tab is showing session '"+target+"' (it may be detached).",
			"没有显示 session '"+target+"' 的 Ghostty tab(可能尚未接回)。")
		i18n.Sae("Restore it with:  gtmux restore "+target, "接回它:  gtmux restore "+target)
		return 1
	}
}
