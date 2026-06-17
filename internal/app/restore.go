package app

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// isTTY reports whether stdin is a terminal.
func isTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// execTmuxAttach replaces this process with `tmux attach -t <name>` (like bash exec).
func execTmuxAttach(name string) int {
	// Set the tab title NOW, before attaching, to match tmux's set-titles-string
	// ('#S — #W'). Why: when we reuse a tab that Ghostty RESTORED from a previous
	// run (macOS "reopen windows"), that tab carries the OLD session's title.
	// tmux does emit a fresh title on attach, but Ghostty re-applies the restored
	// title slightly later, clobbering it — and tmux caches what it "sent" so it
	// never re-emits (a plain `refresh-client` is a no-op). The tab ends up
	// labelled with a DIFFERENT session than the one running in it. Emitting the
	// OSC ourselves here — after restoration has settled, just before exec — wins
	// the race; tmux then keeps the title current as the active window changes.
	if isTTY() {
		title := name
		if t := tmux.Display(name, "#S — #W"); t != "" {
			title = t
		}
		fmt.Printf("\033]0;%s\007", title)
	}
	argv := []string{tmux.Bin, "attach", "-t", name}
	if err := syscall.Exec(tmux.Bin, argv, os.Environ()); err != nil {
		i18n.Sae("failed to attach", "接回失败")
		return 1
	}
	return 0 // unreachable
}

// ensureServer starts tmux (if down) and restores the last tmux-resurrect
// autosave. A throwaway detached session boots the server; once restored sessions
// appear it is dropped, else renamed to "main".
//
// We DRIVE tmux-resurrect explicitly (run-shell, in-server context so its socket
// resolves) rather than passively waiting for continuum's auto-restore hook —
// which is flaky and, for a large layout, slower than any fixed wait. The old
// code waited a fixed 10s; a real restore can take 30s+, so it gave up early,
// created a bare "main", and then continuum's next autosave overwrote the good
// save — losing every session. The guard below makes that unrecoverable case
// impossible: if a saved layout exists but didn't restore, we warn loudly and
// point at the save instead of pretending success.
func ensureServer() {
	if tmux.ServerUp() {
		return
	}
	boot := fmt.Sprintf("gtmux-boot-%d", os.Getpid())
	if !tmux.OK("new-session", "-d", "-s", boot) {
		i18n.Sae("failed to start tmux", "启动 tmux 失败")
		os.Exit(1)
	}

	save := resurrectLastSave()
	hadLayout := save != "" && saveHasLayout(save)

	if script := resurrectRestoreScript(); script != "" {
		i18n.Say("tmux server not running — restoring your saved sessions via tmux-resurrect (may take a moment)...",
			"tmux server 未运行 —— 正在用 tmux-resurrect 恢复你的存档 session(可能要等一会)...")
		tmux.OK("run-shell", script) // runs inside the server; deterministic, not a hook wait
		if waitForRestoredSessions(boot, 120*time.Second) {
			tmux.OK("kill-session", "-t", boot)
			i18n.Say("Restored. Layout/dirs/screen text are back; running programs are NOT (e.g. restart claude with 'claude --resume').",
				"已恢复。布局/目录/屏幕文本都回来了;正在运行的程序不会自动重启(如 claude 用 'claude --resume' 拉起)。")
			return
		}
	} else if hadLayout {
		// No tmux-resurrect script found, but a save exists — fall back to waiting
		// for continuum's auto-restore hook (best effort, longer than the old 10s).
		i18n.Say("tmux server not running — waiting for continuum to restore the last save...",
			"tmux server 未运行 —— 正在等待 continuum 恢复最近一次存档...")
		if waitForRestoredSessions(boot, 60*time.Second) {
			tmux.OK("kill-session", "-t", boot)
			i18n.Say("Restored saved sessions.", "已恢复存档 session。")
			return
		}
	}

	// Nothing restored. If a real layout was saved, do NOT silently keep a bare
	// 'main' as if all is well — continuum would autosave it over the good save.
	// resurrect keeps timestamped saves, so the data is still there; tell the user.
	tmux.OK("rename-session", "-t", boot, "main")
	if hadLayout {
		i18n.Sae("⚠ A saved layout exists but could NOT be restored — it was NOT overwritten. Save: "+save,
			"⚠ 存在存档但未能恢复 —— 没有覆盖它。存档: "+save)
		i18n.Sae("  Recover it before continuum autosaves: point .../resurrect/last at it and re-run, or restore manually.",
			"  请在 continuum 自动存档前恢复: 把 .../resurrect/last 指向它后重试,或手动恢复。")
		return
	}
	i18n.Say("No saved sessions found — created a fresh session 'main'.",
		"没有找到存档 —— 已新建 session 'main'。")
}

// resurrectRestoreScript returns the tmux-resurrect restore.sh path ("" if not
// installed). Must be run inside tmux (run-shell) so its socket resolves.
func resurrectRestoreScript() string {
	home := os.Getenv("HOME")
	cands := []string{
		home + "/.tmux/plugins/tmux-resurrect/scripts/restore.sh",
		home + "/.config/tmux/plugins/tmux-resurrect/scripts/restore.sh",
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		cands = append(cands, xdg+"/tmux/plugins/tmux-resurrect/scripts/restore.sh")
	}
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// resurrectLastSave resolves the resurrect "last" save pointer ("" if none).
func resurrectLastSave() string {
	home := os.Getenv("HOME")
	cands := []string{}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		cands = append(cands, xdg+"/tmux/resurrect/last")
	}
	cands = append(cands,
		home+"/.local/share/tmux/resurrect/last",
		home+"/.tmux/resurrect/last")
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil { // follows the symlink
			return c
		}
	}
	return ""
}

// saveHasLayout reports whether a resurrect save actually holds sessions (any
// window/pane lines) — i.e. losing it would lose real work.
func saveHasLayout(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "window\t") || strings.HasPrefix(line, "pane\t") {
			return true
		}
	}
	return false
}

// waitForRestoredSessions polls until at least one non-boot session exists AND
// the count has settled (resurrect creates sessions incrementally over many
// seconds), or timeout. Returns whether any real session came back.
func waitForRestoredSessions(boot string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	prev, stable := -1, 0
	for time.Now().Before(deadline) {
		n := 0
		for _, s := range tmux.Lines("list-sessions", "-F", "#{session_name}") {
			if s != boot {
				n++
			}
		}
		if n > 0 && n == prev {
			if stable++; stable >= 2 { // unchanged across 2 polls → restore finished
				return true
			}
		} else {
			stable = 0
		}
		prev = n
		time.Sleep(700 * time.Millisecond)
	}
	return prev > 0
}

// unattached returns session names with no client attached.
// Attached flag comes FIRST so the (possibly space-containing) name is the rest.
func unattached() []string {
	var out []string
	for _, line := range tmux.Lines("list-sessions", "-F", "#{session_attached} #{session_name}") {
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
		script, err := ghostty.SpawnTabs(spawn, dryRun)
		if dryRun {
			i18n.Say(fmt.Sprintf("[dry-run] would open %d Ghostty tab(s) for: %s", len(spawn), strings.Join(spawn, " ")),
				fmt.Sprintf("[dry-run] 将为以下 session 各开一个 Ghostty tab: %s", strings.Join(spawn, " ")))
			i18n.Say("[dry-run] AppleScript:", "[dry-run] AppleScript:")
			fmt.Println(script)
		} else if err != nil {
			i18n.Sae("AppleScript failed. Needs Ghostty 1.3+ and Automation permission",
				"AppleScript 执行失败:需要 Ghostty 1.3+ 及自动化权限")
			i18n.Sae("(System Settings → Privacy & Security → Automation → allow controlling Ghostty).",
				"(系统设置 → 隐私与安全性 → 自动化 → 允许控制 Ghostty)。")
			i18n.Sae("Fallback: run 'gtmux restore --one' in each tab",
				"退路: 在每个 tab 里手动运行 gtmux restore --one")
			return 1
		}
	}

	if keepFirst && len(list) > 0 {
		if dryRun {
			i18n.Say("[dry-run] would attach THIS tab to: "+list[0], "[dry-run] 当前 tab 将接回: "+list[0])
			return 0
		}
		return execTmuxAttach(list[0])
	}
	return 0
}

// cmdRestore implements `gtmux restore [--one|--pick|<name>|--dry-run]`.
func cmdRestore(args []string) int {
	if tmux.Bin == "" {
		i18n.Sae("tmux not installed (brew install tmux)", "未安装 tmux (brew install tmux)")
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

	if tmux.InTmux() {
		i18n.Sae("Already inside tmux — run this in a fresh tab.", "已在 tmux 内,请在新 tab 里运行。")
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
		i18n.Say("Every session already has a client attached:", "所有 session 都已有人连接:")
		fmt.Print(mustRun("list-sessions"))
		i18n.Say("To mirror one anyway:  tmux attach -t <name>", "如需镜像查看:  tmux attach -t <名字>")
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
		i18n.Say("Every session already has a client attached — nothing to do:",
			"所有 session 都已有人连接,无需操作:")
		fmt.Print(mustRun("list-sessions"))
		i18n.Say("Tip: pick one to mirror with:  gtmux restore --pick",
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
	for _, line := range tmux.Lines("list-sessions", "-F", "#{session_attached} #{session_name}") {
		if att, name, ok := splitAttached(line); ok {
			all = append(all, sess{name, att != "0"})
		}
	}
	if len(all) == 0 {
		i18n.Sae("No tmux sessions found", "没有任何 tmux session")
		return 1
	}
	i18n.Say("Restorable tmux sessions:", "可接回的 tmux session:")
	for i, s := range all {
		st := i18n.Tr("detached", "待接回")
		if s.attached {
			st = i18n.Tr("attached", "已连接")
		}
		wins, _ := tmux.Run("list-windows", "-t", s.name, "-F", "#W")
		wins = strings.ReplaceAll(wins, "\n", ",")
		fmt.Printf("  %2d) %-24s [%s]  windows: %s\n", i+1, s.name, st, wins)
	}
	fmt.Print(i18n.Tr("Pick (numbers, space-separated; Enter = all detached; q = cancel) > ",
		"选择(编号,空格分隔;回车=全部待接回;q=取消)> "))

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	sel := strings.TrimSpace(line)

	var chosen []string
	switch sel {
	case "q", "Q":
		i18n.Say("Cancelled", "已取消")
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
				i18n.Sae("Invalid choice: "+tok, "无效选择: "+tok)
				return 1
			}
			if n < 1 || n > len(all) {
				i18n.Sae("Out of range: "+tok, "超出范围: "+tok)
				return 1
			}
			chosen = append(chosen, all[n-1].name)
		}
	}
	if len(chosen) == 0 {
		i18n.Say("Nothing to restore — every session is attached", "没有待接回的 session(都已连接)")
		return 0
	}
	return restoreSessions(chosen, dryRun)
}

// mustRun returns tmux output (for display lists); empty on error.
func mustRun(args ...string) string {
	out, _ := tmux.Run(args...)
	if out != "" {
		out += "\n"
	}
	return out
}
