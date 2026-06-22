package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
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
		// A server is already up. The classic post-reboot trap: a reopened
		// terminal tab (or anything) started an EMPTY server before `gtmux
		// restore` ran, so the boot+restore path below is skipped and the saved
		// layout never comes back. Recover it.
		recoverMissingSavedSessions()
		return
	}
	// Fix a poisoned resurrect `last` BEFORE booting: starting the server triggers
	// continuum's auto-restore (@continuum-restore on), which reads `last`. If we
	// don't repair it first, both continuum and our own restore below would faithfully
	// restore an empty save. See sanitizeLast.
	sanitizeLast()
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

// resurrectDir returns the tmux-resurrect data directory ("" if none exists),
// mirroring resurrectLastSave's candidate order.
func resurrectDir() string {
	home := os.Getenv("HOME")
	var cands []string
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		cands = append(cands, xdg+"/tmux/resurrect")
	}
	cands = append(cands,
		home+"/.local/share/tmux/resurrect",
		home+"/.tmux/resurrect")
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

// sanitizeLast guards against tmux-resurrect's "poisoned last" failure. resurrect's
// save_all repoints the `last` symlink to whatever it just dumped, checking only that
// the content CHANGED — never that it is non-empty. A save that races with the server
// losing its sessions (classically: jetsam killing them under memory pressure, since
// continuum runs the save in the background) writes a 0-byte file, and `last` ends up
// pointing at it. From then on every restore — continuum's auto-restore, gtmux's, a
// manual prefix+Ctrl-r — faithfully restores nothing.
//
// We run this BEFORE booting the server so the good pointer is in place for both
// continuum's auto-restore hook and our own restore. If `last` is missing or holds no
// layout, repoint it at the newest timestamped save that actually has one. Timestamped
// names (tmux_resurrect_YYYYMMDDTHHMMSS.txt) sort lexically by time, so a reverse sort
// puts the most recent first.
func sanitizeLast() {
	dir := resurrectDir()
	if dir == "" {
		return
	}
	last := filepath.Join(dir, "last")
	if saveHasLayout(last) { // os.Open follows the symlink; false if missing/empty
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var saves []string
	for _, e := range entries {
		if n := e.Name(); strings.HasPrefix(n, "tmux_resurrect_") && strings.HasSuffix(n, ".txt") {
			saves = append(saves, n)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(saves)))
	for _, n := range saves {
		if saveHasLayout(filepath.Join(dir, n)) {
			_ = os.Remove(last)
			if os.Symlink(n, last) == nil {
				i18n.Say("Repaired tmux-resurrect 'last' (it pointed at an empty save) → "+n,
					"已修复 tmux-resurrect 的 'last'(原先指向空存档) → "+n)
			}
			return
		}
	}
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

// savedSessionNames returns the distinct session names recorded in a resurrect
// save (from its window/pane lines, where field 2 is the session name).
func savedSessionNames(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	seen := map[string]bool{}
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "window\t") && !strings.HasPrefix(line, "pane\t") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		if name := fields[1]; name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// liveSessionNames is the set of session names in the running server.
func liveSessionNames() map[string]bool {
	m := map[string]bool{}
	for _, s := range tmux.Lines("list-sessions", "-F", "#{session_name}") {
		if n := strings.TrimSpace(s); n != "" {
			m[n] = true
		}
	}
	return m
}

// shouldRecover decides whether to drive a resurrect restore into a RUNNING
// server: yes only when the save has sessions AND none of them are live (the
// server is a fresh/empty post-reboot one). If any saved session is already live
// we assume a normal reattach and do nothing, to avoid duplicating sessions.
func shouldRecover(saved []string, live map[string]bool) bool {
	if len(saved) == 0 {
		return false
	}
	for _, s := range saved {
		if live[s] {
			return false
		}
	}
	return true
}

// recoverMissingSavedSessions handles the post-reboot trap: a server is already
// up (so ensureServer's boot+restore is skipped) but the saved layout never came
// back. If a real saved layout exists whose sessions are ALL missing from the
// running server, drive the tmux-resurrect restore into it.
func recoverMissingSavedSessions() {
	save := resurrectLastSave()
	if save == "" || !saveHasLayout(save) {
		return
	}
	saved := savedSessionNames(save)
	if !shouldRecover(saved, liveSessionNames()) {
		return
	}
	script := resurrectRestoreScript()
	if script == "" {
		i18n.Sae("⚠ This tmux server is missing your saved sessions, but tmux-resurrect isn't installed to restore them. Save: "+save,
			"⚠ 当前 tmux server 缺少你的存档 session,但未装 tmux-resurrect 无法恢复。存档: "+save)
		return
	}
	i18n.Say("A tmux server is running but your saved sessions are missing — restoring them from the last save...",
		"检测到 tmux server 在跑但缺少你的存档 session —— 正在用最近一次存档恢复...")
	tmux.OK("run-shell", script)
	if waitForSavedSessions(saved, 120*time.Second) {
		i18n.Say("Restored your saved sessions. (running programs are NOT relaunched — e.g. 'claude --resume')",
			"已恢复你的存档 session。(正在运行的程序不会自动重启 —— 如 'claude --resume')")
	} else {
		i18n.Sae("⚠ Restore did not complete in time — your save is intact at "+save,
			"⚠ 恢复未在限定时间内完成 —— 你的存档完好: "+save)
	}
}

// waitForSavedSessions polls until at least one of the saved sessions appears
// live and the live-session count settles, or timeout.
func waitForSavedSessions(saved []string, timeout time.Duration) bool {
	want := map[string]bool{}
	for _, s := range saved {
		want[s] = true
	}
	deadline := time.Now().Add(timeout)
	prev, stable := -1, 0
	for time.Now().Before(deadline) {
		live := liveSessionNames()
		n := 0
		for s := range live {
			if want[s] {
				n++
			}
		}
		if n > 0 && n == prev {
			if stable++; stable >= 2 {
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
	if len(list) == 0 {
		return 0
	}
	// Open a NEW tab for EVERY unattached session. We deliberately do NOT reuse
	// the current tab for the first session: that old optimization made the
	// (alphabetically) first session depend on a fragile `tmux attach` exec in
	// the current process — if that wasn't a real reusable terminal the first
	// session was silently orphaned (the recurring "session N didn't come back"
	// bug, always hitting the alphabetically-first session). A spare launcher
	// tab is a fine price for never dropping a session.
	term := terminal.Active()
	tn := term.Name()
	script, err := term.SpawnTabs(list, dryRun)
	if dryRun {
		i18n.Say(fmt.Sprintf("[dry-run] would open %d %s tab(s) for: %s", len(list), tn, strings.Join(list, " ")),
			fmt.Sprintf("[dry-run] 将为以下 session 各开一个 %s tab: %s", tn, strings.Join(list, " ")))
		i18n.Say("[dry-run] AppleScript:", "[dry-run] AppleScript:")
		fmt.Println(script)
		return 0
	}
	if err != nil {
		i18n.Sae("AppleScript failed. Needs "+tn+" and Automation permission",
			"AppleScript 执行失败:需要 "+tn+" 及自动化权限")
		i18n.Sae("(System Settings → Privacy & Security → Automation → allow controlling "+tn+").",
			"(系统设置 → 隐私与安全性 → 自动化 → 允许控制 "+tn+")。")
		i18n.Sae("Fallback: run 'gtmux restore --one' in each tab",
			"退路: 在每个 tab 里手动运行 gtmux restore --one")
		return 1
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

	// default: one terminal tab per unattached session, in the recorded tab
	// order (so your arrangement is preserved across restore, not tmux's
	// alphabetical list-sessions order).
	ensureServer()
	sessions := orderByTabOrder(unattached(), state.LoadTabOrder())
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
