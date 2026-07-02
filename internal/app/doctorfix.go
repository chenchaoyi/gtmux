package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// doctor --fix (Layer 2) walks the recommended fixes ONE AT A TIME: each step
// explains what it will change and WHY, then asks before doing it (`--yes` applies
// all without prompting; off a TTY every step is skipped rather than mutating
// silently). It folds in the Claude-hook setup that used to need a separate
// `gtmux install-hooks`. It is conservative and reversible:
//   - tmux.conf edits live in a clearly MARKED managed block (your own lines are
//     never touched), and the file is backed up to .gtmux.bak before the first
//     edit of the run;
//   - it MERGES (never drops) managed lines across runs and only writes an option
//     the running tmux is actually missing, so it won't clobber a value you set;
//   - plugins are cloned (non-destructive); TPM wiring is added only when your
//     config has none, with the `run` line placed last so TPM loads correctly;
//   - things it can't do safely (install tmux, install the app, Codex's
//     single-slot notify) are printed as guidance, not forced.

const (
	fixBlockBegin = "# >>> gtmux managed (gtmux doctor --fix) >>>"
	fixBlockEnd   = "# <<< gtmux managed <<<"
)

type pluginSpec struct{ name, repo string }

var fixPlugins = []pluginSpec{
	{"tpm", "https://github.com/tmux-plugins/tpm"},
	{"tmux-resurrect", "https://github.com/tmux-plugins/tmux-resurrect"},
	{"tmux-continuum", "https://github.com/tmux-plugins/tmux-continuum"},
}

// fixState threads the config path, the consent flag, a one-shot backup latch,
// and the running exit code across the steps.
type fixState struct {
	confPath string
	yes      bool
	backedUp bool
	rc       int
}

// doctorFix runs the step-by-step fixer. yes applies every step without prompting.
func doctorFix(yes bool) int {
	if tmux.Bin == "" {
		i18n.Sae("tmux is not installed — install it first (e.g. brew install tmux), then re-run.",
			"tmux 未安装，请先安装（如 brew install tmux）再运行。")
		return 1
	}
	i18n.Say("gtmux doctor --fix — I'll explain each change and ask before doing it (Ctrl-C to stop).",
		"gtmux doctor --fix，每个改动我都会先解释并征求确认（Ctrl-C 退出）。")

	s := &fixState{confPath: tmuxConfPath(), yes: yes}
	applied := 0
	applied += s.stepLocale()
	applied += s.stepSetTitles()
	applied += s.stepRestoreSettings()
	applied += s.stepPlugins()
	applied += s.stepClaudeHook()
	applied += s.stepCodexHook()
	applied += s.stepCloudflared()
	applied += s.stepAppInstall()

	fmt.Println()
	if applied == 0 {
		i18n.Say("Nothing was changed.", "未做任何改动。")
	} else {
		i18n.Say("Done — re-run `gtmux doctor` to confirm.", "完成，重新跑 `gtmux doctor` 确认。")
	}
	return s.rc
}

// ask prints a step heading + explanation and returns whether to apply it.
func (s *fixState) ask(title, detail string) bool {
	fmt.Printf("\n%s%s%s\n", i18n.Bold, title, i18n.Reset)
	if detail != "" {
		fmt.Printf("%s%s%s\n", i18n.Dim, detail, i18n.Reset)
	}
	if s.yes {
		return true
	}
	if !confirm(i18n.Tr("  apply? [Y/n] ", "  应用？[Y/n] ")) {
		i18n.Say("  skipped.", "  已跳过。")
		return false
	}
	return true
}

// applyConf backs up once, merges `lines` into the managed block, writes it, and
// live-applies `live` (so the change takes effect now). Returns 1 on success.
func (s *fixState) applyConf(lines []string, live [][]string) int {
	conf := ""
	if b, err := os.ReadFile(s.confPath); err == nil {
		conf = string(b)
	}
	if err := os.MkdirAll(filepath.Dir(s.confPath), 0o755); err != nil {
		i18n.Sae("  ✗ "+err.Error(), "  ✗ "+err.Error())
		s.rc = 1
		return 0
	}
	if !s.backedUp {
		backupFile(s.confPath) // .gtmux.bak (no-op if the file doesn't exist yet)
		s.backedUp = true
	}
	merged := mergeManagedLines(conf, lines)
	if err := os.WriteFile(s.confPath, []byte(upsertManagedBlock(conf, merged)), 0o644); err != nil {
		i18n.Sae("  ✗ write "+tildeify(s.confPath)+": "+err.Error(), "  ✗ 写入 "+tildeify(s.confPath)+"："+err.Error())
		s.rc = 1
		return 0
	}
	for _, c := range live {
		_, _ = tmux.Run(c...)
	}
	i18n.Say("  ✓ updated "+tildeify(s.confPath), "  ✓ 已更新 "+tildeify(s.confPath))
	return 1
}

// stepLocale injects a UTF-8 LANG into the tmux SERVER environment (managed
// block + live), so shells started in NEW panes — and the serve/tunnel daemons —
// inherit UTF-8 and stop rendering 中文 file names as ? (and the agent glyphs the
// radar reads stop getting mangled). It does NOT touch your shell rc; the CURRENT
// pane keeps its old env (can't be changed retroactively), so we print the manual
// one-liner for it. Only offered when the ambient locale isn't already UTF-8.
func (s *fixState) stepLocale() int {
	if isUTF8Locale(localeCharset()) {
		return 0
	}
	const val = "en_US.UTF-8"
	detail := i18n.Tr(
		"  Add to "+tildeify(s.confPath)+" (+ apply live):\n      set-environment -g LANG "+val+
			"\n  Why: your locale isn't UTF-8, so 中文 file names show as ? and the agent\n  glyphs the radar reads get mangled. New tmux panes (and the serve/tunnel\n  daemons) will inherit UTF-8.\n  Note: your CURRENT pane won't change — run:  export LANG="+val,
		"  写入 "+tildeify(s.confPath)+"（并立即生效）：\n      set-environment -g LANG "+val+
			"\n  原因：你的 locale 不是 UTF-8，中文文件名显示为 ?，雷达读取的 agent 图标也会乱。\n  新建的 tmux pane（以及 serve/tunnel 守护进程）将继承 UTF-8。\n  注意：当前 pane 不会变——执行：export LANG="+val)
	if !s.ask(i18n.Tr("locale  (UTF-8 for 中文 / agent glyphs)", "字符集（中文 / agent 图标需 UTF-8）"), detail) {
		return 0
	}
	return s.applyConf(
		[]string{"set-environment -g LANG " + val},
		[][]string{{"set-environment", "-g", "LANG", val}})
}

func (s *fixState) stepSetTitles() int {
	if tmuxOpt("set-titles") == "on" && tmuxOpt("set-titles-string") == "#S — #W" {
		return 0
	}
	detail := i18n.Tr(
		"  Add to "+tildeify(s.confPath)+" (+ apply live):\n      set -g set-titles on\n      set -g set-titles-string '#S — #W'\n  Why: focus/restore find a session's tab by this exact title.",
		"  写入 "+tildeify(s.confPath)+"（并立即生效）：\n      set -g set-titles on\n      set -g set-titles-string '#S — #W'\n  原因：focus/restore 靠这个确切标题找到会话的 tab。")
	if !s.ask(i18n.Tr("set-titles  (required for focus/restore)", "set-titles（focus/restore 必需）"), detail) {
		return 0
	}
	return s.applyConf(
		[]string{"set -g set-titles on", "set -g set-titles-string '#S — #W'"},
		[][]string{{"set", "-g", "set-titles", "on"}, {"set", "-g", "set-titles-string", "#S — #W"}})
}

func (s *fixState) stepRestoreSettings() int {
	var lines []string
	var live [][]string
	var bullets []string
	if tmuxOpt("@resurrect-capture-pane-contents") != "on" {
		lines = append(lines, "set -g @resurrect-capture-pane-contents 'on'")
		live = append(live, []string{"set", "-g", "@resurrect-capture-pane-contents", "on"})
		bullets = append(bullets, i18n.Tr("@resurrect-capture-pane-contents on — snapshot each pane's scrollback",
			"@resurrect-capture-pane-contents on，快照每个 pane 的 scrollback"))
	}
	if tmuxOpt("@continuum-restore") != "on" {
		lines = append(lines, "set -g @continuum-restore 'on'")
		live = append(live, []string{"set", "-g", "@continuum-restore", "on"})
		bullets = append(bullets, i18n.Tr("@continuum-restore on — auto-restore after a reboot",
			"@continuum-restore on，重启后自动恢复"))
	}
	if v, _ := strconv.Atoi(tmuxOpt("history-limit")); v < 10000 {
		lines = append(lines, "set -g history-limit 50000")
		live = append(live, []string{"set", "-g", "history-limit", "50000"})
		bullets = append(bullets, i18n.Tr("history-limit 50000 — deeper scrollback to snapshot",
			"history-limit 50000，更深的 scrollback 可快照"))
	}
	if len(lines) == 0 {
		return 0
	}
	detail := "  " + strings.Join(bullets, "\n  ") + "\n  " +
		i18n.Tr("Written to "+tildeify(s.confPath)+" (+ applied live).", "写入 "+tildeify(s.confPath)+"（并立即生效）。")
	if !s.ask(i18n.Tr("restore & snapshot settings", "恢复与快照设置"), detail) {
		return 0
	}
	return s.applyConf(lines, live)
}

func (s *fixState) stepPlugins() int {
	var clones []pluginSpec
	for _, p := range fixPlugins {
		if pluginDir(p.name) == "" {
			clones = append(clones, p)
		}
	}
	if len(clones) == 0 {
		return 0
	}
	conf := ""
	if b, err := os.ReadFile(s.confPath); err == nil {
		conf = string(b)
	}
	wire := !hasTPMWiring(conf)
	var names []string
	for _, c := range clones {
		names = append(names, c.name)
	}
	detail := "  " + i18n.Tr("git clone "+strings.Join(names, ", ")+" → "+tildeify(pluginBaseDir()),
		"git clone "+strings.Join(names, ", ")+" → "+tildeify(pluginBaseDir()))
	if wire {
		detail += "\n  " + i18n.Tr("and add TPM loader lines to "+tildeify(s.confPath)+" (run line last)",
			"并在 "+tildeify(s.confPath)+" 写入 TPM 加载行（run 行置末）")
	}
	detail += "\n  " + i18n.Tr("Why: restore-after-reboot & scrollback snapshots need these plugins.",
		"原因：重启恢复与 scrollback 快照依赖这些插件。")
	if !s.ask(i18n.Tr("tmux plugins (TPM + resurrect + continuum)", "tmux 插件（TPM + resurrect + continuum）"), detail) {
		return 0
	}
	base := pluginBaseDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		i18n.Sae("  ✗ "+err.Error(), "  ✗ "+err.Error())
		s.rc = 1
		return 0
	}
	applied := 0
	for _, p := range clones {
		if err := runQuiet("git", "clone", "--depth", "1", p.repo, filepath.Join(base, p.name)); err != nil {
			i18n.Sae("  ✗ clone "+p.name+" failed: "+err.Error(), "  ✗ 克隆 "+p.name+" 失败："+err.Error())
			s.rc = 1
		} else {
			i18n.Say("  ✓ cloned "+p.name, "  ✓ 已克隆 "+p.name)
			applied = 1
		}
	}
	if wire {
		if s.applyConf(tpmWiringLines(), nil) == 1 {
			applied = 1
		}
	}
	i18n.Say("  • reload tmux to activate: tmux source "+tildeify(s.confPath)+"  (or restart tmux)",
		"  • 重载 tmux 生效：tmux source "+tildeify(s.confPath)+"  （或重启 tmux）")
	return applied
}

func (s *fixState) stepClaudeHook() int {
	if claudeHookInstalled() {
		return 0
	}
	detail := i18n.Tr(
		"  Register `gtmux hook` in "+tildeify(claudeSettingsPath())+" (Stop · Notification · UserPromptSubmit).\n  Gives ⏸ needs-input + desktop notifications. (backed up first)",
		"  在 "+tildeify(claudeSettingsPath())+" 注册 `gtmux hook`（Stop · Notification · UserPromptSubmit）。\n  提供 ⏸ 需要输入 + 桌面通知。（会先备份）")
	if !s.ask(i18n.Tr("Claude Code hook", "Claude Code hook"), detail) {
		return 0
	}
	cacheClaudeIcon()
	if err := updateSettings(claudeSettingsPath(), selfPath(), true); err != nil {
		i18n.Sae("  ✗ failed: "+err.Error(), "  ✗ 失败："+err.Error())
		s.rc = 1
		return 0
	}
	i18n.Say("  ✓ installed — restart Claude Code sessions to load it", "  ✓ 已安装，重启 Claude Code 会话以加载")
	return 1
}

// stepCodexHook wires gtmux into Codex's `notify` (with confirmation), instead of
// printing a copy-paste. Codex allows ONE notify program, so: if none is set we
// add ours at top level (before any [table], where Codex reads it); if a
// non-gtmux one is set we warn and ask before replacing it (default NO, and never
// under --yes — that would silently drop the user's program). Only offered when
// Codex is actually present (~/.codex exists).
func (s *fixState) stepCodexHook() int {
	codexDir := filepath.Join(homeDir(), ".codex")
	if !fileExists(codexDir) || codexNotifyIsGtmux() {
		return 0
	}
	cfgPath := filepath.Join(codexDir, "config.toml")
	content := ""
	if b, err := os.ReadFile(cfgPath); err == nil {
		content = string(b)
	}
	line := fmt.Sprintf("notify = [%q, \"hook\", \"--agent\", \"codex\"]", selfPath())
	existing := findTomlNotify(content)

	if existing != "" {
		// A notify is already set — Codex runs only one, so this is a replacement.
		fmt.Printf("\n%s%s%s\n", i18n.Bold, i18n.Tr("Codex hook  (replaces existing notify)", "Codex hook（替换现有 notify）"), i18n.Reset)
		fmt.Printf("%s%s%s\n", i18n.Dim, i18n.Tr(
			"  "+tildeify(cfgPath)+" already sets a notify (Codex allows only one):\n      "+strings.TrimSpace(existing)+"\n  Replacing it means that program stops running. New value:\n      "+line,
			"  "+tildeify(cfgPath)+" 已设置 notify（Codex 只允许一个）：\n      "+strings.TrimSpace(existing)+"\n  替换后原程序将不再运行。新值：\n      "+line), i18n.Reset)
		if s.yes {
			i18n.Say("  • skipped — re-run interactively to replace your existing notify.",
				"  • 已跳过，如需替换现有 notify，请交互式重跑。")
			return 0
		}
		if !confirmRisky(i18n.Tr("  replace it? [y/N] ", "  替换它？[y/N] ")) {
			i18n.Say("  skipped.", "  已跳过。")
			return 0
		}
		backupFile(cfgPath)
		if s.writeCodex(cfgPath, strings.Replace(content, existing, line, 1)) {
			i18n.Say("  ✓ replaced Codex notify in "+tildeify(cfgPath), "  ✓ 已替换 "+tildeify(cfgPath)+" 的 notify")
			return 1
		}
		return 0
	}

	// No notify yet → add ours.
	detail := i18n.Tr(
		"  Add to "+tildeify(cfgPath)+":\n      "+line+"\n  Why: gtmux notifies you when a Codex turn finishes (detection works without it too).",
		"  写入 "+tildeify(cfgPath)+"：\n      "+line+"\n  原因：Codex turn 结束时 gtmux 发通知（检测本就不依赖它）。")
	if !s.ask(i18n.Tr("Codex hook", "Codex hook"), detail) {
		return 0
	}
	backupFile(cfgPath)
	if s.writeCodex(cfgPath, insertTomlTopLevel(content, line)) {
		i18n.Say("  ✓ wired Codex notify in "+tildeify(cfgPath), "  ✓ 已接入 "+tildeify(cfgPath)+" 的 notify")
		return 1
	}
	return 0
}

// writeCodex writes Codex's config.toml, reporting (and flagging rc on) failure.
func (s *fixState) writeCodex(path, content string) bool {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		i18n.Sae("  ✗ "+err.Error(), "  ✗ "+err.Error())
		s.rc = 1
		return false
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		i18n.Sae("  ✗ write "+tildeify(path)+": "+err.Error(), "  ✗ 写入 "+tildeify(path)+"："+err.Error())
		s.rc = 1
		return false
	}
	return true
}

// findTomlNotify returns the first TOP-LEVEL `notify = …` line (before any
// [table] header — that's where Codex reads it), or "" if none.
func findTomlNotify(content string) string {
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			break // reached a table header → past the top-level keys
		}
		if strings.HasPrefix(t, "notify") && strings.Contains(t, "=") {
			return line
		}
	}
	return ""
}

// insertTomlTopLevel inserts a top-level key line before the first [table] header
// (so it stays top-level), or appends it when there are no tables.
func insertTomlTopLevel(content, line string) string {
	if strings.TrimSpace(content) == "" {
		return line + "\n"
	}
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "[") {
			out := append([]string{}, lines[:i]...)
			out = append(out, line)
			out = append(out, lines[i:]...)
			return strings.Join(out, "\n")
		}
	}
	return strings.TrimRight(content, "\n") + "\n" + line + "\n"
}

// stepCloudflared offers to install the optional tunnel client for remote phone
// access. It's optional, so it defaults to NO interactively (and is skipped, not
// forced, under --yes — installing a 40MB dependency nobody asked for would be
// presumptuous). `gtmux tunnel` also offers this on first run.
func (s *fixState) stepCloudflared() int {
	if _, err := exec.LookPath("cloudflared"); err == nil {
		return 0
	}
	fmt.Printf("\n%s%s%s\n", i18n.Bold,
		i18n.Tr("cloudflared  (optional — remote phone access)", "cloudflared（可选，手机远程访问）"), i18n.Reset)
	fmt.Printf("%s%s%s\n", i18n.Dim, i18n.Tr(
		"  Install it so `gtmux tunnel` can reach your Mac from anywhere. Only needed\n  for remote access — skip if you don't use the phone app away from home.",
		"  装上它，`gtmux tunnel` 就能从任何地方连回你的 Mac。仅远程访问需要，\n  不在外面用手机 app 可跳过。"), i18n.Reset)
	if _, err := exec.LookPath("brew"); err != nil {
		i18n.Say("  • brew not found — install from https://github.com/cloudflare/cloudflared/releases",
			"  • 未找到 brew，从 https://github.com/cloudflare/cloudflared/releases 安装")
		return 0
	}
	if s.yes {
		i18n.Say("  • skipped (optional) — run `gtmux tunnel` or `brew install cloudflared` when you want remote access.",
			"  • 已跳过（可选）。想远程时跑 `gtmux tunnel` 或 `brew install cloudflared`。")
		return 0
	}
	if !confirmRisky(i18n.Tr("  install it now? [y/N] ", "  现在安装？[y/N] ")) {
		i18n.Say("  skipped.", "  已跳过。")
		return 0
	}
	i18n.Say("  • brew install cloudflared …", "  • brew install cloudflared …")
	c := exec.Command("brew", "install", "cloudflared")
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		i18n.Sae("  ✗ brew install failed: "+err.Error(), "  ✗ brew 安装失败："+err.Error())
		s.rc = 1
		return 0
	}
	i18n.Say("  ✓ installed cloudflared — run `gtmux tunnel` to go live", "  ✓ 已安装 cloudflared，跑 `gtmux tunnel` 即可上线")
	return 1
}

// stepAppInstall offers to install the menu-bar app (needed for desktop
// notifications) by running the official installer. Unlike the tmux/hook steps it
// fetches + runs install.sh (CLI + app) — the app is a signed bundle we can't
// assemble locally. No-op when it's already present.
func (s *fixState) stepAppInstall() int {
	if _, err := os.Stat(gtmuxAppPath()); err == nil {
		return 0
	}
	detail := i18n.Tr(
		"  Fetch + run the official installer (install.sh) to add the menu-bar app.\n  Why: desktop notifications (the \"waiting on you\" alert) are delivered BY the app — nothing else posts them.",
		"  拉取并运行官方安装脚本（install.sh）装上菜单栏 app。\n  原因：桌面通知（\"等你输入\"提醒）由这个 app 负责发出，没有它就没有通知。")
	if !s.ask(i18n.Tr("menu-bar app  (for desktop notifications)", "菜单栏 app（桌面通知需要）"), detail) {
		return 0
	}
	if runInstaller(false) != 0 {
		s.rc = 1
		return 0
	}
	return 1
}

// --- config-path + managed-block helpers (pure; unit-tested) ---

func usesXDG() bool {
	return fileExists(filepath.Join(homeDir(), ".config", "tmux", "tmux.conf"))
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// tmuxConfPath is the config file --fix edits: the XDG file if present, else
// the classic ~/.tmux.conf (created if neither exists).
func tmuxConfPath() string {
	if usesXDG() {
		return filepath.Join(homeDir(), ".config", "tmux", "tmux.conf")
	}
	return filepath.Join(homeDir(), ".tmux.conf")
}

// pluginBaseDir is where TPM plugins are cloned, matching the config location.
func pluginBaseDir() string {
	if usesXDG() {
		return filepath.Join(homeDir(), ".config", "tmux", "plugins")
	}
	return filepath.Join(homeDir(), ".tmux", "plugins")
}

func tpmRunPath() string {
	if usesXDG() {
		return "~/.config/tmux/plugins/tpm/tpm"
	}
	return "~/.tmux/plugins/tpm/tpm"
}

func tpmWiringLines() []string {
	return []string{
		"set -g @plugin 'tmux-plugins/tpm'",
		"set -g @plugin 'tmux-plugins/tmux-resurrect'",
		"set -g @plugin 'tmux-plugins/tmux-continuum'",
		"run '" + tpmRunPath() + "'",
	}
}

// hasTPMWiring reports whether the config already loads TPM (so --fix must NOT
// add a second, conflicting wiring).
func hasTPMWiring(conf string) bool {
	return strings.Contains(conf, "tpm/tpm") || strings.Contains(conf, "@plugin")
}

// tildeify shortens $HOME to ~ for display.
func tildeify(p string) string {
	if h := homeDir(); h != "" && strings.HasPrefix(p, h) {
		return "~" + p[len(h):]
	}
	return p
}

// managedKey is the option/command a managed line sets, used to de-dupe across
// runs ("set -g X …" → "X"; "run …" → "run").
func managedKey(line string) string {
	f := strings.Fields(line)
	if len(f) == 0 {
		return line
	}
	if f[0] == "run" {
		return "run"
	}
	if f[0] == "set" && len(f) >= 3 {
		return f[2]
	}
	return line
}

// extractManagedLines returns the body lines of the existing managed block.
func extractManagedLines(conf string) []string {
	bi := strings.Index(conf, fixBlockBegin)
	ei := strings.Index(conf, fixBlockEnd)
	if bi < 0 || ei <= bi {
		return nil
	}
	var out []string
	for _, l := range strings.Split(conf[bi+len(fixBlockBegin):ei], "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// mergeManagedLines unions the existing managed block with newLines (existing
// wins on key collision so we never duplicate), keeping the TPM run line last.
func mergeManagedLines(conf string, newLines []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(lines []string) {
		for _, l := range lines {
			k := managedKey(l)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, l)
		}
	}
	add(extractManagedLines(conf))
	add(newLines)

	var run string
	var rest []string
	for _, l := range out {
		if managedKey(l) == "run" {
			run = l
		} else {
			rest = append(rest, l)
		}
	}
	if run != "" {
		rest = append(rest, run)
	}
	return rest
}

// upsertManagedBlock replaces the managed block in conf with one holding exactly
// `lines`, or appends it (after a blank separator) when absent.
func upsertManagedBlock(conf string, lines []string) string {
	block := fixBlockBegin + "\n" + strings.Join(lines, "\n") + "\n" + fixBlockEnd
	bi := strings.Index(conf, fixBlockBegin)
	ei := strings.Index(conf, fixBlockEnd)
	if bi >= 0 && ei > bi {
		return conf[:bi] + block + conf[ei+len(fixBlockEnd):]
	}
	if conf == "" {
		return block + "\n"
	}
	return strings.TrimRight(conf, "\n") + "\n\n" + block + "\n"
}
