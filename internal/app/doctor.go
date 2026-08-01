package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/servermode"
	"github.com/chenchaoyi/gtmux/internal/state"
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
	secs := []dsection{
		{i18n.Tr("tmux", "tmux"), []dcheck{rowTmux(), rowLocale(), rowSetTitles(), rowHistory()}},
		{i18n.Tr("Restore after reboot", "重启后恢复"), restoreRebootChecks()},
		{i18n.Tr("Terminal", "终端"), []dcheck{rowTerminal()}},
		{i18n.Tr("Agents & notifications", "Agent 与通知"), agents},
		{i18n.Tr("Remote access", "远程访问"), append([]dcheck{rowCloudflared()}, sleepSettingChecks()...)},
		{i18n.Tr("Storage", "存储"), []dcheck{rowDiskUsage()}},
	}
	// Only for a machine that actually runs a supervisor — on any other install these
	// rows would report a cadence for a thing that does not exist.
	if fileExists(state.HQHome()) {
		secs = append(secs, dsection{i18n.Tr("HQ maintenance", "中控维护"),
			hqMaintenanceChecks(time.Now().Unix())})
	}
	return secs
}

// hqMaintenanceChecks reports whether HQ's two periodic rituals are actually running.
// They are raised by the resident serve process, so a stalled cadence means something
// systemic (serve down, the HQ pane unresolvable, a wedged sensor) — and it is otherwise
// invisible: both passes are silent by design, so "it has not distilled in three weeks"
// looks exactly like "nothing needed distilling". This row is the difference.
func hqMaintenanceChecks(now int64) []dcheck {
	distill, selfCheck := hq.MaintenanceStatus(now)
	return []dcheck{
		maintenanceRow(distill,
			i18n.Tr("knowledge distill", "知识蒸馏"),
			i18n.Tr("weekly pass folds the fleet's lessons into the knowledge base",
				"每周把舰队的教训沉淀进知识库"),
			i18n.Tr("no distill for over a week+grace — is `gtmux serve` running with a live HQ?",
				"超过一周+宽限没有蒸馏 —— `gtmux serve` 还在跑、中控还活着吗？")),
		maintenanceRow(selfCheck,
			i18n.Tr("HQ self-check", "中控自检"),
			i18n.Tr("daily pass checks ledger / feed / memory health", "每日自检账本、感知与记忆健康"),
			i18n.Tr("no self-check for over a day+grace — is `gtmux serve` running with a live HQ?",
				"超过一天+宽限没有自检 —— `gtmux serve` 还在跑、中控还活着吗？")),
	}
}

// maintenanceRow renders one cadence verdict. A pass inside its floor is OK; one in the
// grace window is a neutral note (the zero-change gate legitimately skips quiet periods);
// past that it is a ⚠, because the cadence itself has stopped.
func maintenanceRow(r hq.MaintenanceRow, label, okNote, slipNote string) dcheck {
	switch r.State {
	case hq.MaintenanceNever:
		return dcheck{stInfo, label, i18n.Tr("never run", "从未运行"),
			i18n.Tr("no pass raised yet — expected on a fresh HQ", "尚未触发过 —— 新装中控属正常")}
	case hq.MaintenanceSlipped:
		return dcheck{stRec, label, hq.HumanAgeShort(r.AgeSec) + i18n.Tr(" ago", "前"), slipNote}
	default:
		return dcheck{stOK, label, hq.HumanAgeShort(r.AgeSec) + i18n.Tr(" ago", "前"), okNote}
	}
}

// diskAmberBytes / diskRedBytes are the state-dir footprint thresholds the storage
// sentinel warns at. Disk hygiene (diskHygieneSweep) keeps a healthy install well under
// 100 MB; amber flags one trending large, red flags one an unrotated log or a stuck
// hygiene pass has let balloon (the multi-GB incident this guards against).
const (
	diskAmberBytes int64 = 500 << 20 // 500 MB
	diskRedBytes   int64 = 2 << 30   // 2 GB
)

// rowDiskUsage reports the total gtmux state-dir footprint and flags a runaway — the
// visible half of disk hygiene: the sweep keeps it bounded, this makes a breach legible
// before the disk does. An unrotated daemon log (serve/tunnel.log) is the usual culprit.
func rowDiskUsage() dcheck {
	label := i18n.Tr("gtmux disk usage", "gtmux 磁盘占用")
	total := treeSize(state.Dir())
	switch {
	case total >= diskRedBytes:
		return dcheck{stMiss, label, humanBytes(total),
			i18n.Tr("very large — check for a runaway log (serve/tunnel.log)",
				"过大 —— 检查是否有失控日志（serve/tunnel.log）")}
	case total >= diskAmberBytes:
		return dcheck{stRec, label, humanBytes(total),
			i18n.Tr("trending large — hygiene trims logs/uploads automatically",
				"偏大 —— hygiene 会自动裁剪日志/上传")}
	default:
		return dcheck{stOK, label, humanBytes(total),
			i18n.Tr("state dir under control", "状态目录占用正常")}
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

// restoreRebootChecks assembles the "Restore after reboot" rows. The autosave-armed
// check is appended only when continuum is installed AND a server is up (its trigger
// lives in the running status-right) — otherwise there's nothing meaningful to read.
func restoreRebootChecks() []dcheck {
	rows := append(rowPlugins(), rowCapture(), rowAutoRestore())
	if pluginDir("tmux-continuum") != "" && tmux.ServerUp() {
		rows = append(rows, rowAutoSave())
	}
	return rows
}

// rowAutoSave verifies continuum's autosave is actually armed: its save trigger must be
// in the running status-right, else continuum never saves and a reboot restores an
// ancient snapshot (the exact silent failure the restore-save-reliability change guards).
func rowAutoSave() dcheck {
	label := i18n.Tr("resurrect autosave", "resurrect 自动保存")
	note := i18n.Tr("continuum must save periodically for a reboot to restore",
		"continuum 需周期保存,重启才能恢复")
	switch n := continuumTriggerCount(tmuxOpt("status-right")); {
	case n == 1:
		return dcheck{stOK, label, i18n.Tr("armed", "已武装"), note}
	case n > 1:
		// Two triggers = every interval runs the save twice, forever, silently. It comes
		// from a path-FORM mismatch: continuum looks for its own ABSOLUTE path, so a
		// hand-written `~/…` trigger doesn't match and it appends a second copy.
		return dcheck{stRec, label,
			i18n.Tr(fmt.Sprintf("%d triggers", n), fmt.Sprintf("%d 个触发器", n)),
			i18n.Tr("status-right carries continuum's save trigger more than once — every save runs that many times. Usually a `~/…` trigger you wrote by hand plus the absolute-path one continuum added because `~` didn't match its check; keep ONE (the absolute form)",
				"status-right 里有多份 continuum 保存触发器 —— 每次保存会跑这么多遍。通常是你手写的 `~/…` 那份 + continuum 因为 `~` 没匹配上它的检查而又追加的绝对路径那份;只保留一份(用绝对路径那份)")}
	}
	return dcheck{stRec, label, i18n.Tr("trigger missing", "触发器缺失"),
		i18n.Tr("status-right lacks continuum's save trigger — autosave is OFF; append #(~/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh) to status-right",
			"status-right 缺少 continuum 保存触发器 —— 自动保存已关;在 status-right 末尾追加 #(~/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh)")}
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
	if lookTool("cloudflared") != "" {
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

// treeSize / humanBytes are small local copies of the disk-usage helpers the hq
// disk-hygiene sweep also uses — a leaf-level primitive duplicated (not shared)
// so the doctor command need not import the supervisor package.
func treeSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			total += fi.Size()
		}
		return nil
	})
	return total
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return strconv.FormatFloat(float64(n)/(1<<30), 'f', 1, 64) + " GB"
	case n >= 1<<20:
		return strconv.FormatInt(n>>20, 10) + " MB"
	case n >= 1<<10:
		return strconv.FormatInt(n>>10, 10) + " KB"
	default:
		return strconv.FormatInt(n, 10) + " B"
	}
}

// --- server mode (openspec change server-mode) ---

// sleepSettingChecks reports on the system sleep-disable setting. It is SILENT on a
// machine that has never had it touched, so users who don't use server mode see no
// extra rows (the same courtesy rowCodexHook extends to non-Codex users).
//
// It exists because this state is otherwise invisible: the setting survives reboot
// and `pmset`'s reporting commands never mention it, so a Mac left unable to sleep
// stays that way silently — for months, if nobody thinks to look. Making it legible
// is the whole point.
//
// gtmux reverts only what gtmux owns. An unstamped setting is somebody else's
// deliberate choice and is reported with the manual command, never changed.
func sleepSettingChecks() []dcheck {
	stale := true
	if rec, ok := servermode.LoadState(); ok {
		stale = rec.Stale(time.Now())
	}
	return sleepChecksFor(servermode.Current(), stale)
}

// sleepChecksFor is the pure decision half, so every branch is testable without
// touching the machine.
func sleepChecksFor(st servermode.Status, stale bool) []dcheck {
	if !st.SystemDisableSleep && !st.PersistedDisableSleep && !st.OwnedByGtmux {
		return nil // never used here — say nothing
	}
	label := i18n.Tr("sleep setting", "睡眠设置")
	manual := i18n.Tr("undo it with: sudo pmset -a disablesleep 0",
		"手动恢复：sudo pmset -a disablesleep 0")

	switch {
	case st.State == servermode.StateLapsed:
		return []dcheck{{stRec, label, i18n.Tr("lapsed", "已失效"),
			i18n.Tr("gtmux thinks server mode is on but the kernel disabled it — treat the closed-lid session as over",
				"gtmux 以为服务器模式开着，但内核已关闭 —— 请认为合盖会话已结束")}}

	case st.SystemDisableSleep && !st.OwnedByGtmux:
		return []dcheck{{stRec, label, i18n.Tr("this Mac will not sleep", "这台 Mac 不会睡眠"),
			i18n.Tr("not set by gtmux — reported only, never changed for you. ", "不是 gtmux 设的 —— 只报告、不代改。") + manual}}

	case st.SystemDisableSleep:
		// Ours. Fresh heartbeat = server mode in use; stale = left behind.
		if !stale {
			return []dcheck{{stOK, label, i18n.Tr("server mode on", "服务器模式开启中"),
				i18n.Tr("deliberate — the lid may close", "有意为之 —— 合盖不会睡")}}
		}
		return []dcheck{{stMiss, label, i18n.Tr("left disabled by gtmux", "被 gtmux 留在关闭睡眠状态"),
			i18n.Tr("no live server mode — this Mac cannot sleep. ", "没有在运行的服务器模式 —— 这台 Mac 无法睡眠。") + manual}}

	default:
		// Live sleep is fine but the persisted value would disable it again.
		return []dcheck{{stRec, label, i18n.Tr("persisted as disabled", "落盘为禁止睡眠"),
			i18n.Tr("sleeps now, but the stored setting would disable it after a reboot. ",
				"现在能睡，但落盘的设置会在重启后再次禁止。") + manual}}
	}
}
