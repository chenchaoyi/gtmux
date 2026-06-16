package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// hookEvents are the Claude Code events gtmux registers for. Stop/Notification
// fire notifications; UserPromptSubmit is state-only. (Contract — do not rename.)
var hookEvents = []string{"Stop", "Notification", "UserPromptSubmit"}

const lsregister = "/System/Library/Frameworks/CoreServices.framework/" +
	"Frameworks/LaunchServices.framework/Support/lsregister"

func homeDir() string { return os.Getenv("HOME") }

// legacyFocusAppPath is the retired one-shot click target (GtmuxFocus.app). The
// menu-bar app (com.gtmux.menubar) is the notification target now; uninstall
// still cleans this up for anyone migrating from an older install.
func legacyFocusAppPath() string { return filepath.Join(homeDir(), "Applications", "GtmuxFocus.app") }

func claudeSettingsPath() string { return filepath.Join(homeDir(), ".claude", "settings.json") }

func peonPingConfigPath() string {
	return filepath.Join(homeDir(), ".claude", "hooks", "peon-ping", "config.json")
}

// cmdInstallHooks implements `gtmux install-hooks [--yes]`: cache the Claude
// icon and register `gtmux hook` in ~/.claude/settings.json. Idempotent and
// reversible (see cmdUninstallHooks). Notification clicks activate the menu-bar
// app (com.gtmux.menubar), which jumps to last-finished — so it nudges you to
// install Gtmux.app if it's missing.
func cmdInstallHooks(args []string) int {
	yes := false
	for _, a := range args {
		switch a {
		case "-y", "--yes":
			yes = true
		case "-h", "--help":
			usage()
			return 0
		}
	}
	if runtime.GOOS != "darwin" {
		i18n.Sae("install-hooks is macOS-only", "install-hooks 仅支持 macOS")
		return 1
	}
	bin := selfPath()

	// Retire any legacy one-shot click target from an older install.
	if _, err := os.Stat(legacyFocusAppPath()); err == nil {
		runQuiet(lsregister, "-u", legacyFocusAppPath())
		_ = os.RemoveAll(legacyFocusAppPath())
	}

	// 1. Cache the Claude icon (best-effort).
	if cacheClaudeIcon() {
		i18n.Say("✓ cached Claude icon for notifications", "✓ 已缓存 Claude 通知图标")
	}

	// 2. terminal-notifier makes the notification clickable.
	if _, err := exec.LookPath("terminal-notifier"); err != nil {
		i18n.Say("• terminal-notifier not found — notifications won't be clickable. Install: brew install terminal-notifier",
			"• 未找到 terminal-notifier —— 通知将不可点击。安装: brew install terminal-notifier")
	}

	// 3. Register the hook in ~/.claude/settings.json.
	if err := updateSettings(claudeSettingsPath(), bin, true); err != nil {
		i18n.Sae("failed to update ~/.claude/settings.json: "+err.Error(), "更新 ~/.claude/settings.json 失败: "+err.Error())
		return 1
	}
	i18n.Say("✓ registered 'gtmux hook' in ~/.claude/settings.json (Stop · Notification · UserPromptSubmit)",
		"✓ 已在 ~/.claude/settings.json 注册 'gtmux hook' (Stop · Notification · UserPromptSubmit)")

	// 4. peon-ping coexistence.
	handlePeonPing(yes)

	// Notification clicks need the menu-bar app running to jump.
	if _, err := os.Stat(gtmuxAppPath()); err != nil {
		i18n.Say("• install the menu-bar app for click-to-jump notifications (curl installer, or 'make app')",
			"• 安装菜单栏 app 才能点击通知跳转(用 curl 安装脚本,或 'make app')")
	}

	i18n.Say("Done. Restart your Claude Code sessions to load the hooks.",
		"完成。重启 Claude Code 会话以加载 hook。")
	return 0
}

// cmdUninstallHooks implements `gtmux uninstall-hooks`: de-register from
// settings.json. Leaves cached icon/state and the menu-bar app alone (use
// uninstall-app for that); cleans up any legacy GtmuxFocus.app.
func cmdUninstallHooks(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			usage()
			return 0
		}
	}
	if err := updateSettings(claudeSettingsPath(), "", false); err != nil {
		i18n.Sae("failed to update ~/.claude/settings.json: "+err.Error(), "更新 ~/.claude/settings.json 失败: "+err.Error())
		return 1
	}
	i18n.Say("✓ de-registered 'gtmux hook' from ~/.claude/settings.json", "✓ 已从 ~/.claude/settings.json 注销 'gtmux hook'")

	// Remove the legacy one-shot click target if present (best-effort).
	if _, err := os.Stat(legacyFocusAppPath()); err == nil {
		runQuiet(lsregister, "-u", legacyFocusAppPath())
		_ = os.RemoveAll(legacyFocusAppPath())
	}
	i18n.Say("Restart your Claude Code sessions to drop the hooks.", "重启 Claude Code 会话以移除 hook。")
	return 0
}

// cacheClaudeIcon writes a 256px PNG of the Claude app icon to the state dir.
// Best-effort: returns false (silently) if Claude or sips isn't available.
func cacheClaudeIcon() bool {
	icns := "/Applications/Claude.app/Contents/Resources/electron.icns"
	if _, err := os.Stat(icns); err != nil {
		return false
	}
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		return false
	}
	return runQuiet("sips", "-s", "format", "png", "-Z", "256", icns, "--out", state.IconPath()) == nil
}

// handlePeonPing offers to disable peon-ping's desktop notifications and tab-title
// management so gtmux owns both. terminal_tab_title=false is critical: focus
// relies on tmux set-titles owning the tab title.
func handlePeonPing(yes bool) {
	cfg := peonPingConfigPath()
	if _, err := os.Stat(cfg); err != nil {
		return
	}
	if !yes && !confirm(i18n.Tr(
		"peon-ping detected. Disable its desktop_notifications and terminal_tab_title so gtmux owns notifications + tab titles? [Y/n] ",
		"检测到 peon-ping。关闭它的 desktop_notifications 和 terminal_tab_title,让 gtmux 接管通知与 tab 标题?[Y/n] ")) {
		i18n.Say("• left peon-ping unchanged — for focus to work, set terminal_tab_title=false there yourself.",
			"• 未改动 peon-ping —— 为使 focus 生效,请自行将其 terminal_tab_title 设为 false。")
		return
	}
	m, err := loadJSONObject(cfg)
	if err != nil {
		i18n.Sae("• could not read peon-ping config: "+err.Error(), "• 无法读取 peon-ping 配置: "+err.Error())
		return
	}
	backupFile(cfg)
	m["desktop_notifications"] = false
	m["terminal_tab_title"] = false
	if err := writeJSONObject(cfg, m); err != nil {
		i18n.Sae("• could not update peon-ping config: "+err.Error(), "• 无法更新 peon-ping 配置: "+err.Error())
		return
	}
	i18n.Say("✓ disabled peon-ping desktop_notifications + terminal_tab_title", "✓ 已关闭 peon-ping 的 desktop_notifications 与 terminal_tab_title")
}

// updateSettings rewrites the hooks block of a Claude settings.json. When
// install is true it ensures exactly one gtmux-hook entry per event (using the
// absolute bin path); when false it removes every gtmux-hook entry. Other hooks
// and unknown top-level keys are preserved, and the file is backed up first.
func updateSettings(path, bin string, install bool) error {
	m, err := loadJSONObject(path)
	if err != nil {
		return err
	}
	backupFile(path) // no-op if the file doesn't exist yet

	hooks := asObject(m["hooks"])
	cmd := bin + " hook"
	for _, ev := range hookEvents {
		list := removeGtmuxEntries(asArray(hooks[ev]))
		if install {
			list = append(list, map[string]any{
				"matcher": "",
				"hooks": []any{map[string]any{
					"type":    "command",
					"command": cmd,
					"async":   true,
				}},
			})
		}
		if len(list) == 0 {
			delete(hooks, ev)
		} else {
			hooks[ev] = list
		}
	}
	if len(hooks) == 0 {
		delete(m, "hooks")
	} else {
		m["hooks"] = hooks
	}
	return writeJSONObject(path, m)
}

// removeGtmuxEntries drops gtmux-hook commands from a settings hook list,
// dropping any entry left with no hooks. Preserves all non-gtmux entries.
func removeGtmuxEntries(list []any) []any {
	out := make([]any, 0, len(list))
	for _, raw := range list {
		entry, ok := raw.(map[string]any)
		if !ok {
			out = append(out, raw)
			continue
		}
		inner := asArray(entry["hooks"])
		kept := make([]any, 0, len(inner))
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok || !isGtmuxHookCommand(asString(hm["command"])) {
				kept = append(kept, h)
			}
		}
		if len(kept) == 0 {
			continue // whole entry was gtmux's — drop it
		}
		entry["hooks"] = kept
		out = append(out, entry)
	}
	return out
}

// isGtmuxHookCommand matches our registered command regardless of install path,
// so reinstalling (even from a moved binary) replaces rather than duplicates and
// uninstall reliably removes it. Matches "<anything>/gtmux hook" and bare
// "gtmux hook" by basename — not a brittle string suffix.
func isGtmuxHookCommand(cmd string) bool {
	f := strings.Fields(cmd)
	return len(f) == 2 && f[1] == "hook" && filepath.Base(f[0]) == "gtmux"
}

// --- small generic JSON-object helpers (preserve unknown fields) ---

func loadJSONObject(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

func writeJSONObject(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// backupFile copies path to path.gtmux.bak when it exists (a single rolling backup).
func backupFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = os.WriteFile(path+".gtmux.bak", b, 0o644)
}

func asObject(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asArray(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// confirm prompts on a TTY (default yes); off a TTY it returns false so callers
// fall back to a printed recommendation rather than mutating silently.
func confirm(prompt string) bool {
	if !isTTY() {
		return false
	}
	fmt.Print(prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	s := strings.TrimSpace(strings.ToLower(line))
	return s == "" || s == "y" || s == "yes"
}

// runQuiet runs a command discarding its output; returns its error.
func runQuiet(bin string, args ...string) error {
	return exec.Command(bin, args...).Run()
}
