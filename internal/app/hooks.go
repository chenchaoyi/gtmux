package app

import (
	"bufio"
	_ "embed"
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

//go:embed templates/Info.plist
var infoPlist string

//go:embed templates/run.tmpl
var runTmpl string

// hookEvents are the Claude Code events gtmux registers for. Stop/Notification
// fire notifications; UserPromptSubmit is state-only. (Contract — do not rename.)
var hookEvents = []string{"Stop", "Notification", "UserPromptSubmit"}

const focusBundleID = "com.gtmux.focus" // stable id; agents read last-finished

const lsregister = "/System/Library/Frameworks/CoreServices.framework/" +
	"Frameworks/LaunchServices.framework/Support/lsregister"

func homeDir() string { return os.Getenv("HOME") }

func focusAppPath() string { return filepath.Join(homeDir(), "Applications", "GtmuxFocus.app") }

func claudeSettingsPath() string { return filepath.Join(homeDir(), ".claude", "settings.json") }

func peonPingConfigPath() string {
	return filepath.Join(homeDir(), ".claude", "hooks", "peon-ping", "config.json")
}

// cmdInstallHooks implements `gtmux install-hooks [--yes]`: generate
// GtmuxFocus.app, cache the Claude icon, and register `gtmux hook` in
// ~/.claude/settings.json. Idempotent and reversible (see cmdUninstallHooks).
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

	// 1. GtmuxFocus.app — the clickable notification target.
	if err := writeFocusApp(bin); err != nil {
		i18n.Sae("failed to create GtmuxFocus.app: "+err.Error(), "创建 GtmuxFocus.app 失败: "+err.Error())
		return 1
	}
	runQuiet(lsregister, "-f", focusAppPath()) // best-effort: resolve -activate by id
	i18n.Say("✓ GtmuxFocus.app installed ("+focusBundleID+")", "✓ 已安装 GtmuxFocus.app ("+focusBundleID+")")

	// 2. Cache the Claude icon (best-effort).
	if cacheClaudeIcon() {
		i18n.Say("✓ cached Claude icon for notifications", "✓ 已缓存 Claude 通知图标")
	}

	// 3. terminal-notifier makes the notification clickable.
	if _, err := exec.LookPath("terminal-notifier"); err != nil {
		i18n.Say("• terminal-notifier not found — notifications won't be clickable. Install: brew install terminal-notifier",
			"• 未找到 terminal-notifier —— 通知将不可点击。安装: brew install terminal-notifier")
	}

	// 4. Register the hook in ~/.claude/settings.json.
	if err := updateSettings(claudeSettingsPath(), bin, true); err != nil {
		i18n.Sae("failed to update ~/.claude/settings.json: "+err.Error(), "更新 ~/.claude/settings.json 失败: "+err.Error())
		return 1
	}
	i18n.Say("✓ registered 'gtmux hook' in ~/.claude/settings.json (Stop · Notification · UserPromptSubmit)",
		"✓ 已在 ~/.claude/settings.json 注册 'gtmux hook' (Stop · Notification · UserPromptSubmit)")

	// 5. peon-ping coexistence.
	handlePeonPing(yes)

	i18n.Say("Done. Restart your Claude Code sessions to load the hooks.",
		"完成。重启 Claude Code 会话以加载 hook。")
	return 0
}

// cmdUninstallHooks implements `gtmux uninstall-hooks`: de-register from
// settings.json and remove GtmuxFocus.app. Leaves cached icon/state alone.
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

	app := focusAppPath()
	runQuiet(lsregister, "-u", app) // best-effort
	if err := os.RemoveAll(app); err != nil {
		i18n.Sae("failed to remove GtmuxFocus.app: "+err.Error(), "删除 GtmuxFocus.app 失败: "+err.Error())
		return 1
	}
	i18n.Say("✓ removed GtmuxFocus.app", "✓ 已删除 GtmuxFocus.app")
	i18n.Say("Restart your Claude Code sessions to drop the hooks.", "重启 Claude Code 会话以移除 hook。")
	return 0
}

// writeFocusApp materializes ~/Applications/GtmuxFocus.app from the embedded
// templates, injecting the absolute gtmux path into the run script.
func writeFocusApp(bin string) error {
	macOS := filepath.Join(focusAppPath(), "Contents", "MacOS")
	if err := os.MkdirAll(macOS, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(focusAppPath(), "Contents", "Info.plist"), []byte(infoPlist), 0o644); err != nil {
		return err
	}
	run := strings.ReplaceAll(runTmpl, "__GTMUX_BIN__", bin)
	return os.WriteFile(filepath.Join(macOS, "run"), []byte(run), 0o755)
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
