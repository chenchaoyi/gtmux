package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// check status levels.
const (
	stOK   = iota
	stRec  // recommended (a feature degrades without it)
	stMiss // required (a core feature breaks without it)
	stInfo // neutral note
)

type doctorCounters struct{ ok, rec, miss int }

// line prints one check line and tallies it.
func (n *doctorCounters) line(status int, en, zh string) {
	icon := "✓"
	switch status {
	case stRec:
		icon, n.rec = "⚠", n.rec+1
	case stMiss:
		icon, n.miss = "✗", n.miss+1
	case stInfo:
		icon = "·"
	default:
		n.ok++
	}
	fmt.Printf("  %s %s\n", icon, i18n.Tr(en, zh))
}

// cmdDoctor implements `gtmux doctor`: a READ-ONLY health check (Layer 1) mapping
// each gtmux feature to the tmux / terminal / hook prerequisite it needs. With
// `--fix` it then applies the recommended fixes (Layer 2, see doctorFix) after a
// confirmation; `--yes` skips the prompt.
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

	i18n.Say("gtmux doctor — environment check (read-only; nothing is changed)",
		"gtmux doctor —— 环境体检(只读,不改动任何东西)")
	fmt.Println()

	n := &doctorCounters{}
	doctorTmux(n)
	doctorSetTitles(n)
	doctorPlugins(n)
	doctorResurrect(n)
	doctorHistory(n)
	doctorTerminal(n)
	doctorHooks(n)
	doctorApp(n)

	fmt.Println()
	i18n.Say(
		fmt.Sprintf("%d ok · %d recommended · %d required-missing", n.ok, n.rec, n.miss),
		fmt.Sprintf("%d 正常 · %d 建议 · %d 必需缺失", n.ok, n.rec, n.miss))

	if fix {
		fmt.Println()
		return doctorFix(yes)
	}
	if n.rec > 0 || n.miss > 0 {
		i18n.Say("Run `gtmux doctor --fix` to apply the recommended fixes.",
			"跑 `gtmux doctor --fix` 自动修复上面的建议项。")
	}
	if n.miss > 0 {
		return 1
	}
	return 0
}

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

func doctorTmux(n *doctorCounters) {
	if tmux.Bin == "" {
		n.line(stMiss, "tmux: NOT found — gtmux needs tmux.  Install: brew install tmux",
			"tmux: 未找到 —— gtmux 依赖 tmux。安装: brew install tmux")
		return
	}
	ver := ""
	if v := tmux.Lines("-V"); len(v) > 0 {
		ver = v[0]
	}
	n.line(stOK, "tmux: "+ver, "tmux: "+ver)
}

func doctorSetTitles(n *doctorCounters) {
	if tmuxOpt("set-titles") == "on" && tmuxOpt("set-titles-string") == "#S — #W" {
		n.line(stOK, "set-titles: on with '#S — #W' (focus/restore can locate tabs)",
			"set-titles: 已开且为 '#S — #W'(focus/restore 能定位 tab)")
		return
	}
	n.line(stMiss,
		"set-titles: REQUIRED for focus/restore (tabs are matched by this title). In tmux.conf:\n        set -g set-titles on\n        set -g set-titles-string '#S — #W'",
		"set-titles: focus/restore 必需(靠这个标题定位 tab)。tmux.conf 加:\n        set -g set-titles on\n        set -g set-titles-string '#S — #W'")
}

func doctorPlugins(n *doctorCounters) {
	for _, p := range []struct{ dir, en, zh string }{
		{"tpm", "TPM (tmux plugin manager)", "TPM(tmux 插件管理器)"},
		{"tmux-resurrect", "tmux-resurrect (restore layout after reboot)", "tmux-resurrect(重启后恢复布局)"},
		{"tmux-continuum", "tmux-continuum (auto-save + auto-restore)", "tmux-continuum(自动存档+恢复)"},
	} {
		if pluginDir(p.dir) != "" {
			n.line(stOK, p.en+": installed", p.zh+": 已装")
		} else {
			n.line(stRec, p.en+": not installed — restore/snapshot needs it (TPM: github.com/tmux-plugins/tpm)",
				p.zh+": 未装 —— restore/快照依赖它(TPM: github.com/tmux-plugins/tpm)")
		}
	}
}

func doctorResurrect(n *doctorCounters) {
	if tmuxOpt("@resurrect-capture-pane-contents") == "on" {
		n.line(stOK, "@resurrect-capture-pane-contents: on (scrollback snapshot on restore)",
			"@resurrect-capture-pane-contents: 已开(restore 时带回 scrollback 快照)")
	} else {
		n.line(stRec, "@resurrect-capture-pane-contents: off — set 'on' to snapshot each pane's scrollback",
			"@resurrect-capture-pane-contents: 未开 —— 设为 'on' 才能快照每个 pane 的 scrollback")
	}
	if tmuxOpt("@continuum-restore") == "on" {
		n.line(stOK, "@continuum-restore: on (auto-restore on tmux start)",
			"@continuum-restore: 已开(tmux 启动时自动恢复)")
	} else {
		n.line(stRec, "@continuum-restore: off — set 'on' to auto-restore after a reboot",
			"@continuum-restore: 未开 —— 设为 'on' 才能重启后自动恢复")
	}
}

func doctorHistory(n *doctorCounters) {
	hl := tmuxOpt("history-limit")
	if v, _ := strconv.Atoi(hl); v >= 10000 {
		n.line(stOK, "history-limit: "+hl+" (good scrollback depth)", "history-limit: "+hl+"(滚动缓冲够深)")
	} else {
		n.line(stRec, "history-limit: "+hl+" — raise it (e.g. 50000) for deeper scrollback snapshots",
			"history-limit: "+hl+" —— 调大(如 50000)以获得更深的 scrollback 快照")
	}
}

func doctorTerminal(n *doctorCounters) {
	name := terminal.DetectedName()
	if terminal.HasDriver(name) {
		n.line(stOK, "host terminal: "+name+" (supported — focus/restore/new work)",
			"宿主终端: "+name+"(已支持 —— focus/restore/new 可用)")
	} else {
		n.line(stRec, "host terminal: "+name+" (no driver yet — agents/overview still work, but focus/restore/new don't)",
			"宿主终端: "+name+"(暂无驱动 —— agents/overview 照常,但 focus/restore/new 不可用)")
	}
}

func doctorHooks(n *doctorCounters) {
	if claudeHookInstalled() {
		n.line(stOK, "Claude Code hook: installed (⏸ needs-input + notifications)",
			"Claude Code hook: 已装(⏸ 需要输入 + 通知)")
	} else {
		n.line(stRec, "Claude Code hook: not installed — run `gtmux install-hooks` for ⏸ needs-input + notifications",
			"Claude Code hook: 未装 —— 跑 `gtmux install-hooks` 获得 ⏸ 需要输入 + 通知")
	}
	if codexNotifyIsGtmux() {
		n.line(stOK, "Codex hook: wired (turn-done notifications)", "Codex hook: 已接(turn 结束通知)")
	} else {
		n.line(stInfo, "Codex hook: not wired — `gtmux install-hooks --agent codex` prints the snippet (detection works regardless)",
			"Codex hook: 未接 —— `gtmux install-hooks --agent codex` 打印接法(检测不依赖它)")
	}
}

func doctorApp(n *doctorCounters) {
	if _, err := os.Stat(gtmuxAppPath()); err == nil {
		n.line(stOK, "menu-bar app: installed (delivers notifications)", "菜单栏 app: 已装(负责发通知)")
	} else {
		n.line(stRec, "menu-bar app: not installed — needed for desktop notifications (curl installer, or `make app`)",
			"菜单栏 app: 未装 —— 桌面通知需要它(curl 安装脚本,或 `make app`)")
	}
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
