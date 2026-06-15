package main

import (
	"regexp"
)

var paneIDRe = regexp.MustCompile(`^%[0-9]+`)

// jumpPane selects a pane's window+pane in tmux and brings its Ghostty tab
// forward (no output). Used by the watch TUI on Enter.
func jumpPane(paneID string) {
	if tmuxBin == "" || display(paneID, "#{pane_id}") == "" {
		return
	}
	sess := display(paneID, "#{session_name}")
	if win := display(paneID, "#{window_id}"); win != "" {
		tmuxOK("select-window", "-t", win)
	}
	tmuxOK("select-pane", "-t", paneID)
	if sess != "" {
		ghosttyFocusTab(sess)
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
		sae("usage: gtmux focus <session|pane-id>   (jump to that tab / exact pane)",
			"用法: gtmux focus <session|pane-id>   (跳到那个 tab / 确切 pane)")
		return 2
	}

	if paneIDRe.MatchString(target) {
		if tmuxBin == "" || display(target, "#{pane_id}") == "" {
			sae("Pane "+target+" no longer exists", "pane "+target+" 已不存在")
			return 1
		}
		sess := display(target, "#{session_name}")
		win := display(target, "#{window_id}")
		if win != "" {
			tmuxOK("select-window", "-t", win)
		}
		tmuxOK("select-pane", "-t", target)
		target = sess // fall through to focus this session's Ghostty tab
	}
	if target == "" {
		sae("could not resolve a session", "无法解析 session")
		return 1
	}

	res, err := ghosttyFocusTab(target)
	switch {
	case res == "ok":
		return 0
	case err != nil || res == "":
		sae("AppleScript failed. Needs Ghostty 1.3+ and Automation permission",
			"AppleScript 执行失败:需要 Ghostty 1.3+ 及自动化权限。")
		sae("(System Settings → Privacy & Security → Automation → allow controlling Ghostty).",
			"(系统设置 → 隐私与安全性 → 自动化 → 允许控制 Ghostty)。")
		return 1
	default:
		sae("No Ghostty tab is showing session '"+target+"' (it may be detached).",
			"没有显示 session '"+target+"' 的 Ghostty tab(可能尚未接回)。")
		sae("Restore it with:  gtmux restore "+target, "接回它:  gtmux restore "+target)
		return 1
	}
}
