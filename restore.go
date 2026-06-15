package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// isTTY reports whether stdin is a terminal.
func isTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// execTmuxAttach replaces this process with `tmux attach -t <name>` (like bash exec).
func execTmuxAttach(name string) int {
	argv := []string{tmuxBin, "attach", "-t", name}
	if err := syscall.Exec(tmuxBin, argv, os.Environ()); err != nil {
		sae("failed to attach", "接回失败")
		return 1
	}
	return 0 // unreachable
}

// ensureServer starts tmux (if down) and waits for tmux-continuum to restore the
// last autosave. A throwaway detached session boots the server; once restored
// sessions appear it is dropped, else renamed to "main".
func ensureServer() {
	if tmuxServerUp() {
		return
	}
	boot := fmt.Sprintf("gtmux-boot-%d", os.Getpid())
	say("tmux server not running — starting it; continuum is restoring the last saved sessions...",
		"tmux server 未运行 —— 正在启动;continuum 正在恢复最近一次保存的 session...")
	if !tmuxOK("new-session", "-d", "-s", boot) {
		sae("failed to start tmux", "启动 tmux 失败")
		os.Exit(1)
	}
	for i := 0; i < 20; i++ {
		for _, n := range tmuxLines("list-sessions", "-F", "#{session_name}") {
			if n != boot {
				tmuxOK("kill-session", "-t", boot)
				say("Restored. Layout/dirs/screen text are back; running programs are NOT (e.g. restart claude with 'claude --resume').",
					"已恢复。布局/目录/屏幕文本都回来了;正在运行的程序不会自动重启(如 claude 用 'claude --resume' 拉起)。")
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	tmuxOK("rename-session", "-t", boot, "main")
	say("No saved sessions found — created a fresh session 'main'.",
		"没有找到存档 —— 已新建 session 'main'。")
}

// unattached returns session names with no client attached.
// Attached flag comes FIRST so the (possibly space-containing) name is the rest.
func unattached() []string {
	var out []string
	for _, line := range tmuxLines("list-sessions", "-F", "#{session_attached} #{session_name}") {
		if att, name, ok := splitAttached(line); ok && att == "0" {
			out = append(out, name)
		}
	}
	return out
}

// splitAttached parses an "<attached> <session name>" line, where the name may
// contain spaces. Returns attached flag, name, ok.
func splitAttached(line string) (att, name string, ok bool) {
	i := strings.IndexByte(line, ' ')
	if i < 0 {
		return "", "", false
	}
	return line[:i], line[i+1:], true
}

// restoreSessions opens one Ghostty tab per session and attaches them all.
// If we are a Ghostty tab ourselves, reuse THIS tab for the first session.
func restoreSessions(list []string, dryRun bool) int {
	keepFirst := os.Getenv("TERM_PROGRAM") == "ghostty" && isTTY()
	spawn := list
	if keepFirst && len(list) > 0 {
		spawn = list[1:]
	}

	if len(spawn) > 0 {
		script, err := ghosttySpawnTabs(spawn, dryRun)
		if dryRun {
			say(fmt.Sprintf("[dry-run] would open %d Ghostty tab(s) for: %s", len(spawn), strings.Join(spawn, " ")),
				fmt.Sprintf("[dry-run] 将为以下 session 各开一个 Ghostty tab: %s", strings.Join(spawn, " ")))
			say("[dry-run] AppleScript:", "[dry-run] AppleScript:")
			fmt.Println(script)
		} else if err != nil {
			sae("AppleScript failed. Needs Ghostty 1.3+ and Automation permission",
				"AppleScript 执行失败:需要 Ghostty 1.3+ 及自动化权限")
			sae("(System Settings → Privacy & Security → Automation → allow controlling Ghostty).",
				"(系统设置 → 隐私与安全性 → 自动化 → 允许控制 Ghostty)。")
			sae("Fallback: run 'gtmux restore --one' in each tab",
				"退路: 在每个 tab 里手动运行 gtmux restore --one")
			return 1
		}
	}

	if keepFirst && len(list) > 0 {
		if dryRun {
			say("[dry-run] would attach THIS tab to: "+list[0], "[dry-run] 当前 tab 将接回: "+list[0])
			return 0
		}
		return execTmuxAttach(list[0])
	}
	return 0
}

// cmdRestore implements `gtmux restore [--one|--pick|<name>|--dry-run]`.
func cmdRestore(args []string) int {
	if tmuxBin == "" {
		sae("tmux not installed (brew install tmux)", "未安装 tmux (brew install tmux)")
		return 1
	}
	mode, target, dryRun := "all", "", false
	for _, a := range args {
		switch a {
		case "--one":
			mode = "one"
		case "--pick", "-p":
			mode = "pick"
		case "--all":
			mode = "all"
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			usage()
			return 0
		default:
			target = a
		}
	}

	if inTmux() {
		sae("Already inside tmux — run this in a fresh tab.", "已在 tmux 内,请在新 tab 里运行。")
		return 1
	}

	if target != "" {
		ensureServer()
		return execTmuxAttach(target)
	}

	if mode == "one" {
		ensureServer()
		if u := unattached(); len(u) > 0 {
			return execTmuxAttach(u[0])
		}
		say("Every session already has a client attached:", "所有 session 都已有人连接:")
		fmt.Print(mustRun("list-sessions"))
		say("To mirror one anyway:  tmux attach -t <name>", "如需镜像查看:  tmux attach -t <名字>")
		return 0
	}

	if mode == "pick" {
		ensureServer()
		return restorePick(dryRun)
	}

	// default: one Ghostty tab per unattached session
	ensureServer()
	sessions := unattached()
	if len(sessions) == 0 {
		say("Every session already has a client attached — nothing to do:",
			"所有 session 都已有人连接,无需操作:")
		fmt.Print(mustRun("list-sessions"))
		say("Tip: pick one to mirror with:  gtmux restore --pick",
			"提示: 如需镜像查看可用:  gtmux restore --pick")
		return 0
	}
	return restoreSessions(sessions, dryRun)
}

// restorePick lists sessions and lets the user choose which to restore.
func restorePick(dryRun bool) int {
	type sess struct {
		name     string
		attached bool
	}
	var all []sess
	for _, line := range tmuxLines("list-sessions", "-F", "#{session_attached} #{session_name}") {
		if att, name, ok := splitAttached(line); ok {
			all = append(all, sess{name, att != "0"})
		}
	}
	if len(all) == 0 {
		sae("No tmux sessions found", "没有任何 tmux session")
		return 1
	}
	say("Restorable tmux sessions:", "可接回的 tmux session:")
	for i, s := range all {
		st := tr("detached", "待接回")
		if s.attached {
			st = tr("attached", "已连接")
		}
		wins, _ := tmuxRun("list-windows", "-t", s.name, "-F", "#W")
		wins = strings.ReplaceAll(wins, "\n", ",")
		fmt.Printf("  %2d) %-24s [%s]  windows: %s\n", i+1, s.name, st, wins)
	}
	fmt.Print(tr("Pick (numbers, space-separated; Enter = all detached; q = cancel) > ",
		"选择(编号,空格分隔;回车=全部待接回;q=取消)> "))

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	sel := strings.TrimSpace(line)

	var chosen []string
	switch sel {
	case "q", "Q":
		say("Cancelled", "已取消")
		return 0
	case "", "a", "all":
		for _, s := range all {
			if !s.attached {
				chosen = append(chosen, s.name)
			}
		}
	default:
		for _, tok := range strings.Fields(strings.ReplaceAll(sel, ",", " ")) {
			n, err := strconv.Atoi(tok)
			if err != nil {
				sae("Invalid choice: "+tok, "无效选择: "+tok)
				return 1
			}
			if n < 1 || n > len(all) {
				sae("Out of range: "+tok, "超出范围: "+tok)
				return 1
			}
			chosen = append(chosen, all[n-1].name)
		}
	}
	if len(chosen) == 0 {
		say("Nothing to restore — every session is attached", "没有待接回的 session(都已连接)")
		return 0
	}
	return restoreSessions(chosen, dryRun)
}

// mustRun returns tmux output (for display lists); empty on error.
func mustRun(args ...string) string {
	out, _ := tmuxRun(args...)
	if out != "" {
		out += "\n"
	}
	return out
}
