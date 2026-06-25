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

// claudeHook is one Claude Code hook gtmux registers: the event + (PreToolUse
// only) a tool-name matcher regex.
type claudeHook struct {
	event   string
	matcher string
}

// hookEvents are the Claude Code hooks gtmux registers. Stop/Notification fire
// notifications; UserPromptSubmit is state-only; PreToolUse/PostToolUse are
// SCOPED via a matcher to the always-blocking plan/question tools, so they fire
// only when you're actually asked — not on every (auto-approved) tool call.
// PreToolUse raises the wait; PostToolUse clears it once you've answered, so a
// long-running approved plan doesn't keep showing "waiting" until Stop.
// (Contract — do not rename.)
var hookEvents = []claudeHook{
	{event: "Stop"},
	{event: "Notification"},
	{event: "UserPromptSubmit"},
	{event: "PreToolUse", matcher: "ExitPlanMode|AskUserQuestion"},
	{event: "PostToolUse", matcher: "ExitPlanMode|AskUserQuestion"},
}

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

// installHooksCodex prints how to wire Codex's `notify` at gtmux. We deliberately
// do NOT auto-edit ~/.codex/config.toml: `notify` is a SINGLE program (often
// already in use, e.g. computer-use) and Codex's richer hooks format isn't
// documented, so clobbering either would be worse than guiding. Detection
// (working/idle) already works without any of this — this only adds notifications.
func installHooksCodex() int {
	bin := selfPath()
	i18n.Say("Codex: add this to ~/.codex/config.toml so gtmux sends a notification when a turn finishes:",
		"Codex：把下面这行加到 ~/.codex/config.toml，turn 结束时 gtmux 就会发通知：")
	fmt.Printf("\n  notify = [%q, \"hook\", \"--agent\", \"codex\"]\n\n", bin)
	i18n.Say("Note: Codex's `notify` points at ONE program — if you already use it (e.g. computer-use),\nthis REPLACES it; gtmux can't chain into it. Detection (working/idle) works regardless.",
		"注意：Codex 的 `notify` 只能指向一个程序，如果你已在用它（如 computer-use），这会替换它；\ngtmux 无法链式追加。检测（working/idle）不依赖它，照常工作。")
	return 0
}

// cmdInstallHooks implements `gtmux install-hooks [--yes]`: cache the Claude
// icon and register `gtmux hook` in ~/.claude/settings.json. Idempotent and
// reversible (see cmdUninstallHooks). Notification clicks activate the menu-bar
// app (com.gtmux.menubar), which jumps to last-finished — so it nudges you to
// install Gtmux.app if it's missing.
func cmdInstallHooks(args []string) int {
	yes := false
	agent := "claude"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-y", "--yes":
			yes = true
		case "-h", "--help":
			usage()
			return 0
		case "--agent":
			if i+1 < len(args) {
				agent = args[i+1]
				i++
			}
		}
	}
	if runtime.GOOS != "darwin" {
		i18n.Sae("install-hooks is macOS-only", "install-hooks 仅支持 macOS")
		return 1
	}
	if agent == "codex" {
		return installHooksCodex()
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

	// 2. Register the hook in ~/.claude/settings.json.
	if err := updateSettings(claudeSettingsPath(), bin, true); err != nil {
		i18n.Sae("failed to update ~/.claude/settings.json: "+err.Error(), "更新 ~/.claude/settings.json 失败："+err.Error())
		return 1
	}
	i18n.Say("✓ registered 'gtmux hook' in ~/.claude/settings.json (Stop · Notification · UserPromptSubmit · plan/question)",
		"✓ 已在 ~/.claude/settings.json 注册 'gtmux hook' (Stop · Notification · UserPromptSubmit · 计划/提问)")

	// 3. peon-ping coexistence.
	handlePeonPing(yes)

	// The menu-bar app delivers notifications now (no terminal-notifier). Without
	// it installed, the hook still tracks state but no banners are posted.
	if _, err := os.Stat(gtmuxAppPath()); err != nil {
		i18n.Say("• install the menu-bar app to get desktop notifications (curl installer, or 'make app')",
			"• 安装菜单栏 app 才能收到桌面通知（用 curl 安装脚本，或 'make app'）")
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
		i18n.Sae("failed to update ~/.claude/settings.json: "+err.Error(), "更新 ~/.claude/settings.json 失败："+err.Error())
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
		"检测到 peon-ping。关闭它的 desktop_notifications 和 terminal_tab_title，让 gtmux 接管通知与 tab 标题？[Y/n] ")) {
		i18n.Say("• left peon-ping unchanged — for focus to work, set terminal_tab_title=false there yourself.",
			"• 未改动 peon-ping，为使 focus 生效，请自行将其 terminal_tab_title 设为 false。")
		return
	}
	m, err := loadJSONObject(cfg)
	if err != nil {
		i18n.Sae("• could not read peon-ping config: "+err.Error(), "• 无法读取 peon-ping 配置："+err.Error())
		return
	}
	backupFile(cfg)
	m["desktop_notifications"] = false
	m["terminal_tab_title"] = false
	if err := writeJSONObject(cfg, m); err != nil {
		i18n.Sae("• could not update peon-ping config: "+err.Error(), "• 无法更新 peon-ping 配置："+err.Error())
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
	for _, h := range hookEvents {
		list := removeGtmuxEntries(asArray(hooks[h.event]))
		if install {
			list = append(list, map[string]any{
				"matcher": h.matcher,
				"hooks": []any{map[string]any{
					"type":    "command",
					"command": cmd,
					"async":   true,
				}},
			})
		}
		if len(list) == 0 {
			delete(hooks, h.event)
		} else {
			hooks[h.event] = list
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

// confirm prompts on a TTY (Enter = default yes); off a TTY it returns false so
// callers fall back to a printed recommendation rather than mutating silently.
// EOF with no input (e.g. stdin redirected from /dev/null, which still passes the
// char-device isTTY check) must NOT count as a yes — otherwise a non-interactive
// caller would silently auto-confirm a mutating action.
func confirm(prompt string) bool { return promptConfirm(prompt, true) }

// confirmRisky is confirm with the default flipped to NO — for a destructive
// choice (e.g. replacing a value the user set), where Enter must not assent.
func confirmRisky(prompt string) bool { return promptConfirm(prompt, false) }

func promptConfirm(prompt string, defaultYes bool) bool {
	if !isTTY() {
		return false
	}
	fmt.Print(prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return answer(line, err, defaultYes)
}

// answerYes interprets a default-yes prompt answer (kept for its callers/tests).
func answerYes(line string, readErr error) bool { return answer(line, readErr, true) }

// answer interprets a prompt answer: Enter (empty, no error) = the default;
// "y"/"yes" = yes; anything else = no. A read error with no input (EOF on a
// redirected stdin) is NEVER a yes — the guard against silent auto-confirm.
func answer(line string, readErr error, defaultYes bool) bool {
	s := strings.TrimSpace(strings.ToLower(line))
	if readErr != nil && s == "" {
		return false
	}
	if s == "" {
		return defaultYes
	}
	return s == "y" || s == "yes"
}

// runQuiet runs a command discarding its output; returns its error.
func runQuiet(bin string, args ...string) error {
	return exec.Command(bin, args...).Run()
}
