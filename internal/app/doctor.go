package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// check status levels.
const (
	stOK   = iota // ✓ good
	stRec         // ⚠ recommended (a feature degrades without it)
	stMiss        // ✗ blocking (a core feature breaks without it)
	stInfo        // · neutral note
)

// dcheck is one rendered row: a status, a short label, the current value, and a
// dim one-line "why it matters" note.
type dcheck struct {
	status             int
	label, value, note string
}

// dsection groups related checks under a heading.
type dsection struct {
	title string
	rows  []dcheck
}

func statusGlyph(status int) (glyph, color string) {
	switch status {
	case stRec:
		return "⚠", i18n.Yellow
	case stMiss:
		return "✗", i18n.Red
	case stInfo:
		return "·", i18n.Dim
	default:
		return "✓", i18n.Green
	}
}

// cmdDoctor implements `gtmux doctor`: a grouped, READ-ONLY health check (Layer 1)
// mapping each gtmux feature to the tmux / terminal / hook prerequisite it needs.
// With `--fix` it then walks the recommended fixes, explaining and confirming each
// one (Layer 2, see doctorFix); `--yes` applies them all without prompting.
func cmdDoctor(args []string) int {
	fix, yes := false, false
	for _, a := range args {
		switch a {
		case "-h", "--help":
			usage()
			return 0
		case "--fix":
			fix = true
		case "-y", "--yes":
			yes = true
		}
	}

	secs := doctorSections()

	fmt.Printf("%sgtmux doctor%s %s· macOS environment (read-only)%s\n\n",
		i18n.Bold, i18n.Reset, i18n.Dim, i18n.Reset)
	ok, rec, miss := renderSections(secs)

	fmt.Printf("  %s%d %s%s · %s%d %s%s · %s%d %s%s\n",
		i18n.Green, ok, i18n.Tr("ok", "正常"), i18n.Reset,
		i18n.Yellow, rec, i18n.Tr("to improve", "待改进"), i18n.Reset,
		i18n.Red, miss, i18n.Tr("blocking", "阻塞"), i18n.Reset)

	if fix {
		fmt.Println()
		return doctorFix(yes)
	}
	if rec > 0 || miss > 0 {
		// Don't make the user re-run from scratch with --fix: if we're on a TTY,
		// offer to walk the fixes right here (each step still explains + asks).
		// Off a TTY (piped / CI), just print the hint and keep doctor read-only.
		if isTTY() {
			fmt.Println()
			if confirm(i18n.Tr("  Fix these now? [Y/n] ", "  现在就修复这些？[Y/n] ")) {
				fmt.Println()
				return doctorFix(false)
			}
			i18n.Say("  → or run `gtmux doctor --fix` anytime (explains + asks before each change)",
				"  → 也可随时跑 `gtmux doctor --fix`（每步先解释并征求确认）")
		} else {
			i18n.Say("  → run `gtmux doctor --fix` to set up the rest (it explains and asks before each change)",
				"  → 跑 `gtmux doctor --fix` 把其余项配好（每步都会解释并征求确认）")
		}
	}
	if miss > 0 {
		return 1
	}
	return 0
}

// renderSections prints the grouped checks with aligned columns and returns the
// tally (ok / recommended / blocking).
func renderSections(secs []dsection) (ok, rec, miss int) {
	lw, vw := 0, 0
	for _, s := range secs {
		for _, r := range s.rows {
			if w := i18n.DispWidth(r.label); w > lw {
				lw = w
			}
			if w := i18n.DispWidth(r.value); w > vw {
				vw = w
			}
		}
	}
	for _, s := range secs {
		fmt.Printf("  %s%s%s\n", i18n.Bold, s.title, i18n.Reset)
		for _, r := range s.rows {
			switch r.status {
			case stOK:
				ok++
			case stRec:
				rec++
			case stMiss:
				miss++
			}
			glyph, color := statusGlyph(r.status)
			note := ""
			if r.note != "" {
				note = "   " + i18n.Dim + r.note + i18n.Reset
			}
			fmt.Printf("    %s%s%s  %s  %s%s\n",
				color, glyph, i18n.Reset,
				i18n.PadRight(r.label, lw),
				i18n.PadRight(r.value, vw), note)
		}
		fmt.Println()
	}
	return ok, rec, miss
}

// doctorSections runs every check and groups the rows by concern.
func doctorSections() []dsection {
	agents := []dcheck{rowClaudeHook()}
	// Only surface Codex when it's actually present (~/.codex exists), so users
	// who don't run Codex aren't shown an irrelevant row.
	if fileExists(filepath.Join(homeDir(), ".codex")) {
		agents = append(agents, rowCodexHook())
	}
	agents = append(agents, rowApp())
	return []dsection{
		{i18n.Tr("tmux", "tmux"), []dcheck{rowTmux(), rowLocale(), rowSetTitles(), rowHistory()}},
		{i18n.Tr("Restore after reboot", "重启后恢复"),
			append(rowPlugins(), rowCapture(), rowAutoRestore())},
		{i18n.Tr("Terminal", "终端"), []dcheck{rowTerminal()}},
		{i18n.Tr("Agents & notifications", "Agent 与通知"), agents},
		{i18n.Tr("Remote access", "远程访问"), []dcheck{rowCloudflared()}},
	}
}

// --- individual checks (read-only) ---

func rowTmux() dcheck {
	if tmux.Bin == "" {
		return dcheck{stMiss, i18n.Tr("tmux", "tmux"), i18n.Tr("not found", "未找到"),
			i18n.Tr("gtmux needs tmux — brew install tmux", "gtmux 依赖 tmux，brew install tmux")}
	}
	ver := ""
	if v := tmux.Lines("-V"); len(v) > 0 {
		ver = strings.TrimPrefix(v[0], "tmux ")
	}
	return dcheck{stOK, i18n.Tr("tmux", "tmux"), ver, ""}
}

// rowLocale flags when the environment's locale isn't UTF-8. Without a UTF-8
// LC_CTYPE/LANG, tmux substitutes every non-ASCII byte with "_"/"?" — so CJK
// (中文) file names render as ? and the ✳/braille agent glyphs `classifyAgent`
// keys off get mangled. gtmux forces UTF-8 on its OWN tmux calls (internal/tmux),
// but panes and shells you open inherit the ambient env, so a non-UTF-8 locale
// still bites your interactive `ls` and any pane gtmux didn't spawn.
func rowLocale() dcheck {
	label := i18n.Tr("locale", "字符集")
	note := i18n.Tr("UTF-8 so 中文 names + agent glyphs render right",
		"UTF-8 才能正确显示中文名称与 agent 图标")
	cs := localeCharset()
	if isUTF8Locale(cs) {
		return dcheck{stOK, label, cs, note}
	}
	val := cs
	if val == "" {
		val = i18n.Tr("unset", "未设置")
	}
	return dcheck{stRec, label, val,
		i18n.Tr("not UTF-8 — 中文 file names show as ?; set a UTF-8 LANG",
			"非 UTF-8——中文文件名显示为 ?；需设置 UTF-8 的 LANG")}
}

// localeCharset returns the effective locale string in POSIX precedence
// (LC_ALL > LC_CTYPE > LANG), or "" when none is set.
func localeCharset() string {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// isUTF8Locale reports whether a locale string selects the UTF-8 charset.
func isUTF8Locale(v string) bool {
	u := strings.ToUpper(v)
	return strings.Contains(u, "UTF-8") || strings.Contains(u, "UTF8")
}

func rowSetTitles() dcheck {
	label := i18n.Tr("set-titles", "set-titles")
	note := i18n.Tr("focus/restore locate tabs by this title", "focus/restore 靠此标题定位 tab")
	if tmuxOpt("set-titles") == "on" && tmuxOpt("set-titles-string") == "#S — #W" {
		return dcheck{stOK, label, "on · '#S — #W'", note}
	}
	return dcheck{stMiss, label, i18n.Tr("not set", "未设置"), note}
}

func rowHistory() dcheck {
	label := i18n.Tr("history-limit", "history-limit")
	hl := tmuxOpt("history-limit")
	if v, _ := strconv.Atoi(hl); v >= 10000 {
		return dcheck{stOK, label, hl, i18n.Tr("scrollback depth", "滚动缓冲深度")}
	}
	return dcheck{stRec, label, hl, i18n.Tr("raise to ~50000 for deeper snapshots", "调到 ~50000 获得更深快照")}
}

func rowPlugins() []dcheck {
	specs := []struct{ dir, label, en, zh string }{
		{"tpm", i18n.Tr("TPM", "TPM"), "plugin manager", "插件管理器"},
		{"tmux-resurrect", i18n.Tr("tmux-resurrect", "tmux-resurrect"), "restores layout after reboot", "重启后恢复布局"},
		{"tmux-continuum", i18n.Tr("tmux-continuum", "tmux-continuum"), "auto-save + auto-restore", "自动存档 + 恢复"},
	}
	rows := make([]dcheck, 0, len(specs))
	for _, s := range specs {
		if pluginDir(s.dir) != "" {
			rows = append(rows, dcheck{stOK, s.label, i18n.Tr("installed", "已装"), i18n.Tr(s.en, s.zh)})
		} else {
			rows = append(rows, dcheck{stRec, s.label, i18n.Tr("missing", "未装"), i18n.Tr(s.en, s.zh)})
		}
	}
	return rows
}

func rowCapture() dcheck {
	label := i18n.Tr("capture-pane", "capture-pane")
	note := i18n.Tr("snapshot scrollback on restore", "restore 时带回 scrollback")
	if tmuxOpt("@resurrect-capture-pane-contents") == "on" {
		return dcheck{stOK, label, "on", note}
	}
	return dcheck{stRec, label, "off", note}
}

func rowAutoRestore() dcheck {
	label := i18n.Tr("auto-restore", "auto-restore")
	note := i18n.Tr("continuum restores on tmux start", "tmux 启动时 continuum 自动恢复")
	if tmuxOpt("@continuum-restore") == "on" {
		return dcheck{stOK, label, "on", note}
	}
	return dcheck{stRec, label, "off", note}
}

func rowTerminal() dcheck {
	name := terminal.DetectedName()
	label := i18n.Tr("host", "宿主终端")
	if terminal.HasDriver(name) {
		return dcheck{stOK, label, name, i18n.Tr("focus / restore / new supported", "focus / restore / new 可用")}
	}
	return dcheck{stRec, label, name, i18n.Tr("no driver — agents work, focus/restore don't", "暂无驱动，agents 照常，focus/restore 不可用")}
}

func rowClaudeHook() dcheck {
	label := i18n.Tr("Claude Code hook", "Claude Code hook")
	if claudeHookInstalled() {
		return dcheck{stOK, label, i18n.Tr("installed", "已装"), i18n.Tr("⏸ needs-input + notifications", "⏸ 需要输入 + 通知")}
	}
	return dcheck{stRec, label, i18n.Tr("not installed", "未装"), i18n.Tr("⏸ needs-input + notifications", "⏸ 需要输入 + 通知")}
}

func rowCodexHook() dcheck {
	label := i18n.Tr("Codex hook", "Codex hook")
	// Wired via the preferred hooks system (precise state), or the legacy notify.
	if codexHooksWired() {
		return dcheck{stOK, label, i18n.Tr("wired", "已接"), i18n.Tr("precise state + notifications", "状态精准 + 通知")}
	}
	if codexNotifyIsGtmux() {
		return dcheck{stOK, label, i18n.Tr("wired (notify)", "已接（notify）"), i18n.Tr("turn-done notifications", "turn 结束通知")}
	}
	// Only reached when ~/.codex EXISTS (this row isn't added otherwise), so Codex is
	// in use — an un-wired hook is a real improvement (`--fix` offers it), not a
	// neutral note. Detection still works without it, but you miss precise per-event
	// state + notifications.
	return dcheck{stRec, label, i18n.Tr("not wired", "未接"), i18n.Tr("wire for precise state + notifications", "接入以获精准状态 + 通知")}
}

// rowCloudflared surfaces the optional tunnel client. It's only needed for
// `gtmux tunnel` (remote phone access), so a missing one is neutral (·), not a
// problem — `doctor --fix` offers to install it.
func rowCloudflared() dcheck {
	label := i18n.Tr("cloudflared", "cloudflared")
	if _, err := exec.LookPath("cloudflared"); err == nil {
		return dcheck{stOK, label, i18n.Tr("installed", "已装"),
			i18n.Tr("remote access via `gtmux tunnel`", "`gtmux tunnel` 远程访问")}
	}
	return dcheck{stInfo, label, i18n.Tr("not installed", "未装"),
		i18n.Tr("optional — only for `gtmux tunnel`", "可选，仅 `gtmux tunnel` 需要")}
}

func rowApp() dcheck {
	label := i18n.Tr("menu-bar app", "菜单栏 app")
	if _, err := os.Stat(gtmuxAppPath()); err == nil {
		return dcheck{stOK, label, i18n.Tr("installed", "已装"), i18n.Tr("delivers desktop notifications", "负责发桌面通知")}
	}
	return dcheck{stRec, label, i18n.Tr("not installed", "未装"), i18n.Tr("needed for notifications", "通知需要它")}
}

// --- shared probes (also used by doctorFix) ---

// tmuxOpt reads a global tmux option's value ("" if unset/error).
func tmuxOpt(name string) string {
	lines := tmux.Lines("show", "-gv", name)
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

// pluginDir returns the install dir of a TPM plugin if present.
func pluginDir(name string) string {
	for _, base := range []string{
		filepath.Join(homeDir(), ".tmux", "plugins"),
		filepath.Join(homeDir(), ".config", "tmux", "plugins"),
	} {
		p := filepath.Join(base, name)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return ""
}

// claudeHookInstalled reports whether ~/.claude/settings.json has a gtmux hook.
func claudeHookInstalled() bool {
	b, err := os.ReadFile(claudeSettingsPath())
	if err != nil {
		return false
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	hooks, _ := m["hooks"].(map[string]any)
	for _, ev := range hooks {
		arr, _ := ev.([]any)
		for _, item := range arr {
			obj, _ := item.(map[string]any)
			inner, _ := obj["hooks"].([]any)
			for _, h := range inner {
				ho, _ := h.(map[string]any)
				if cmd, _ := ho["command"].(string); isGtmuxHookCommand(cmd) {
					return true
				}
			}
		}
	}
	return false
}

// codexNotifyIsGtmux reports whether Codex's notify points at gtmux.
func codexNotifyIsGtmux() bool {
	b, err := os.ReadFile(filepath.Join(homeDir(), ".codex", "config.toml"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "notify") && strings.Contains(l, "gtmux") && strings.Contains(l, "codex") {
			return true
		}
	}
	return false
}
