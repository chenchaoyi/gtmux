package app

import (
	"fmt"
	"regexp"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
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
		terminal.Active().FocusTab(sess)
	}
}

// focusPaneByID selects an exact tmux pane (%N) — window then pane — and brings
// its terminal tab forward, the same local "jump" the watch TUI does on Enter.
// It injects no input (read-only/no RCE); the remote server calls it for
// POST /api/focus ("when you're back at your desk, you're already on this pane").
// Returns an error if id isn't a pane id or the pane no longer exists.
func focusPaneByID(id string) error {
	if !paneIDRe.MatchString(id) {
		return fmt.Errorf("not a pane id: %q", id)
	}
	if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
		return fmt.Errorf("pane %s no longer exists", id)
	}
	jumpPane(id)
	return nil
}

// cmdFocus implements `gtmux focus <session|pane-id>`.
// A tmux pane id (%N) first selects that window+pane inside its session (so the
// session displays that exact pane), then its Ghostty tab is brought forward.
func cmdFocus(args []string) int {
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
		switch {
		case res == "ok" || (err == nil && res == ""):
			return 0
		case res == "notfound":
			i18n.Sae("No tab titled '"+tabTitle+"' in "+termApp, "在 "+termApp+" 里没有标题为 '"+tabTitle+"' 的 tab")
			return 1
		default:
			i18n.Sae("AppleScript failed (needs Automation permission)", "AppleScript 执行失败（需要自动化权限）")
			return 1
		}
	}
	args = rest

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
			"用法：gtmux focus <session|pane-id|--last>   （跳到那个 tab / 确切 pane）")
		return 2
	case "--last", "-l":
		// Jump to the pane recorded in last-finished (the notification click target).
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

	term := terminal.Active()
	tn := term.Name()
	res, err := term.FocusTab(target)
	switch {
	case res == "ok":
		return 0
	case err != nil || res == "":
		i18n.Sae("AppleScript failed. Needs "+tn+" and Automation permission",
			"AppleScript 执行失败：需要 "+tn+" 及自动化权限。")
		i18n.Sae("(System Settings → Privacy & Security → Automation → allow controlling "+tn+").",
			"（系统设置 → 隐私与安全性 → 自动化 → 允许控制 "+tn+"）。")
		return 1
	default:
		i18n.Sae("No "+tn+" tab is showing session '"+target+"' (it may be detached).",
			"没有显示 session '"+target+"' 的 "+tn+" tab（可能尚未接回）。")
		i18n.Sae("Restore it with:  gtmux restore "+target, "接回它：  gtmux restore "+target)
		return 1
	}
}
