package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// doctor --fix (Layer 2) applies the recommended fixes that Layer 1 reports.
// It is conservative and reversible by design:
//   - tmux.conf edits live inside a clearly MARKED managed block (your own lines
//     are never touched), and the file is backed up to .gtmux.bak first;
//   - it MERGES (never drops) managed lines across runs and only adds an option
//     the running tmux is actually missing, so it won't clobber a value you set;
//   - plugins are cloned (non-destructive); TPM wiring is added only when your
//     config has none, and is placed last so TPM loads correctly;
//   - everything it can't do safely (install tmux, install the app, Codex's
//     single-slot notify) is printed as guidance, not forced.

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

// optFix is one tmux-option fix: the config line(s) for the managed block, the
// live `tmux set -g` command(s) to apply it now, and its bilingual plan text.
type optFix struct {
	confLines []string
	setCmds   [][]string
	en, zh    string
}

// usesXDG reports whether the user keeps tmux config under ~/.config/tmux.
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

// neededOptFixes returns the tmux-option fixes the RUNNING tmux is missing, so
// --fix never overrides a value you already set acceptably.
func neededOptFixes() []optFix {
	var fixes []optFix
	if tmuxOpt("set-titles") != "on" || tmuxOpt("set-titles-string") != "#S — #W" {
		fixes = append(fixes, optFix{
			confLines: []string{"set -g set-titles on", "set -g set-titles-string '#S — #W'"},
			setCmds:   [][]string{{"set", "-g", "set-titles", "on"}, {"set", "-g", "set-titles-string", "#S — #W"}},
			en:        "set-titles on + '#S — #W' (focus/restore tab matching)",
			zh:        "set-titles on + '#S — #W'(focus/restore 定位 tab)",
		})
	}
	if tmuxOpt("@resurrect-capture-pane-contents") != "on" {
		fixes = append(fixes, optFix{
			confLines: []string{"set -g @resurrect-capture-pane-contents 'on'"},
			setCmds:   [][]string{{"set", "-g", "@resurrect-capture-pane-contents", "on"}},
			en:        "@resurrect-capture-pane-contents on (scrollback snapshot on restore)",
			zh:        "@resurrect-capture-pane-contents on(restore 时带回 scrollback 快照)",
		})
	}
	if tmuxOpt("@continuum-restore") != "on" {
		fixes = append(fixes, optFix{
			confLines: []string{"set -g @continuum-restore 'on'"},
			setCmds:   [][]string{{"set", "-g", "@continuum-restore", "on"}},
			en:        "@continuum-restore on (auto-restore after reboot)",
			zh:        "@continuum-restore on(重启后自动恢复)",
		})
	}
	if v, _ := strconv.Atoi(tmuxOpt("history-limit")); v < 10000 {
		fixes = append(fixes, optFix{
			confLines: []string{"set -g history-limit 50000"},
			setCmds:   [][]string{{"set", "-g", "history-limit", "50000"}},
			en:        "history-limit 50000 (deeper scrollback)",
			zh:        "history-limit 50000(更深的 scrollback)",
		})
	}
	return fixes
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

// extractManagedLines returns the body lines of the existing managed block ("").
func extractManagedLines(conf string) []string {
	bi := strings.Index(conf, fixBlockBegin)
	ei := strings.Index(conf, fixBlockEnd)
	if bi < 0 || ei <= bi {
		return nil
	}
	body := conf[bi+len(fixBlockBegin) : ei]
	var out []string
	for _, l := range strings.Split(body, "\n") {
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

	// Float the single `run` line to the end (TPM must be initialized last).
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
	sep := "\n\n"
	if !strings.HasSuffix(conf, "\n") {
		sep = "\n\n"
	}
	return strings.TrimRight(conf, "\n") + sep + block + "\n"
}

// doctorFix runs the Layer-2 fixer. yes skips the confirmation prompt.
func doctorFix(yes bool) int {
	if tmux.Bin == "" {
		i18n.Sae("tmux is not installed — install it first (e.g. brew install tmux), then re-run.",
			"tmux 未安装 —— 请先安装(如 brew install tmux)再运行。")
		return 1
	}
	confPath := tmuxConfPath()
	confBytes, _ := os.ReadFile(confPath)
	conf := string(confBytes)

	opts := neededOptFixes()
	var clones []pluginSpec
	for _, p := range fixPlugins {
		if pluginDir(p.name) == "" {
			clones = append(clones, p)
		}
	}
	wireTPM := len(clones) > 0 && !hasTPMWiring(conf)
	installClaude := !claudeHookInstalled()

	if len(opts) == 0 && len(clones) == 0 && !installClaude {
		i18n.Say("Nothing to fix — everything checks out. ✓", "无需修复 —— 一切正常。✓")
		return 0
	}

	// Preview.
	i18n.Say("gtmux doctor --fix will:", "gtmux doctor --fix 将:")
	for _, o := range opts {
		fmt.Printf("  • %s\n", i18n.Tr(o.en, o.zh))
	}
	if len(clones) > 0 {
		var names []string
		for _, c := range clones {
			names = append(names, c.name)
		}
		fmt.Printf("  • %s\n", i18n.Tr("clone tmux plugins: "+strings.Join(names, ", "),
			"克隆 tmux 插件: "+strings.Join(names, ", ")))
	}
	if wireTPM {
		fmt.Printf("  • %s\n", i18n.Tr("wire TPM into "+tildeify(confPath), "在 "+tildeify(confPath)+" 写入 TPM 加载配置"))
	}
	if installClaude {
		fmt.Printf("  • %s\n", i18n.Tr("install the Claude Code hook (needs-input + notifications)",
			"安装 Claude Code hook(需要输入 + 通知)"))
	}
	if len(opts) > 0 || wireTPM {
		i18n.Say("It edits a marked block in "+tildeify(confPath)+" (backup: .gtmux.bak); your own lines are untouched.",
			"它会改写 "+tildeify(confPath)+" 里一段带标记的配置(备份: .gtmux.bak);不动你自己的行。")
	}
	if !yes && !confirm(i18n.Tr("Apply these fixes? [Y/n] ", "应用这些修复?[Y/n] ")) {
		i18n.Say("Aborted — nothing changed.", "已取消 —— 未做改动。")
		return 1
	}

	rc := 0

	// 1. tmux.conf managed block (options + maybe TPM wiring), then live-apply.
	if len(opts) > 0 || wireTPM {
		var newLines []string
		for _, o := range opts {
			newLines = append(newLines, o.confLines...)
		}
		if wireTPM {
			newLines = append(newLines, tpmWiringLines()...)
		}
		if err := os.MkdirAll(filepath.Dir(confPath), 0o755); err != nil {
			i18n.Sae("could not create "+tildeify(filepath.Dir(confPath))+": "+err.Error(),
				"无法创建 "+tildeify(filepath.Dir(confPath))+": "+err.Error())
			return 1
		}
		backupFile(confPath) // writes .gtmux.bak if the file exists
		merged := mergeManagedLines(conf, newLines)
		if err := os.WriteFile(confPath, []byte(upsertManagedBlock(conf, merged)), 0o644); err != nil {
			i18n.Sae("failed to write "+tildeify(confPath)+": "+err.Error(),
				"写入 "+tildeify(confPath)+" 失败: "+err.Error())
			return 1
		}
		i18n.Say("✓ updated "+tildeify(confPath)+" (managed block)", "✓ 已更新 "+tildeify(confPath)+"(托管配置块)")
		for _, o := range opts {
			for _, cmd := range o.setCmds {
				_, _ = tmux.Run(cmd...) // live-apply so focus/restore work now
			}
		}
	}

	// 2. Clone missing plugins.
	if len(clones) > 0 {
		base := pluginBaseDir()
		if err := os.MkdirAll(base, 0o755); err != nil {
			i18n.Sae("could not create "+tildeify(base)+": "+err.Error(), "无法创建 "+tildeify(base)+": "+err.Error())
			rc = 1
		} else {
			for _, p := range clones {
				dir := filepath.Join(base, p.name)
				if err := runQuiet("git", "clone", "--depth", "1", p.repo, dir); err != nil {
					i18n.Sae("✗ clone "+p.name+" failed: "+err.Error(), "✗ 克隆 "+p.name+" 失败: "+err.Error())
					rc = 1
				} else {
					i18n.Say("✓ cloned "+p.name, "✓ 已克隆 "+p.name)
				}
			}
			i18n.Say("• reload tmux to activate plugins:  tmux source "+tildeify(confPath)+"  (or restart tmux)",
				"• 重载 tmux 以启用插件:  tmux source "+tildeify(confPath)+"  (或重启 tmux)")
		}
	}

	// 3. Claude hook (reuse the install-hooks machinery).
	if installClaude {
		cacheClaudeIcon()
		if err := updateSettings(claudeSettingsPath(), selfPath(), true); err != nil {
			i18n.Sae("✗ Claude hook install failed: "+err.Error(), "✗ 安装 Claude hook 失败: "+err.Error())
			rc = 1
		} else {
			i18n.Say("✓ installed the Claude Code hook (restart Claude sessions to load it)",
				"✓ 已安装 Claude Code hook(重启 Claude 会话以加载)")
		}
	}

	// Things --fix won't force.
	if _, err := os.Stat(gtmuxAppPath()); err != nil {
		i18n.Say("• menu-bar app not installed (needed for notifications): curl installer, or `make app`",
			"• 菜单栏 app 未装(通知需要它):用 curl 安装脚本,或 `make app`")
	}

	i18n.Say("Done. Re-run `gtmux doctor` to confirm.", "完成。重新跑 `gtmux doctor` 确认。")
	return rc
}
