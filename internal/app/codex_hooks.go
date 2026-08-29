package app

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Codex has TWO ways to signal gtmux, and this file drives the better one:
//
//   - the legacy `notify` in config.toml — a SINGLE program Codex allows, so wiring
//     gtmux there REPLACES whatever was set (e.g. a computer-use notifier). It only
//     fires on turn-done.
//   - the hooks system (~/.codex/hooks.json, gated by `features.hooks = true`) —
//     precise per-event state (UserPromptSubmit/Stop/PermissionRequest/Session*), and
//     crucially ADDITIVE: it COEXISTS with an existing `notify`, so we never clobber
//     the user's program.
//
// We prefer the hooks system. The on-disk shape is verified against Codex's HooksFile
// (config crate): events sit under a top-level "hooks" object — root-level event keys
// are rejected — and each handler's `timeout` is in SECONDS. That's exactly what
// formatCodex + updateAgentSettings write.

// codexHome resolves Codex's config dir ($CODEX_HOME, else ~/.codex).
func codexHome() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	return filepath.Join(homeDir(), ".codex")
}

func codexConfigPath() string { return filepath.Join(codexHome(), "config.toml") }

// installCodexHooks wires gtmux into Codex's hooks system: it writes the gtmux
// entries into ~/.codex/hooks.json (additive — foreign hooks are preserved) and
// enables `features.hooks` in config.toml. Any existing `notify` is left untouched.
// Idempotent and reversible (uninstall via the generic agentInstallers path).
func installCodexHooks(yes bool) int {
	inst := agentInstallers["codex"]
	path := inst.configPath()
	if err := updateAgentSettings(inst, path, selfPath(), true); err != nil {
		i18n.Sae("failed to update "+tildeify(path)+": "+err.Error(),
			"更新 "+tildeify(path)+" 失败："+err.Error())
		return 1
	}
	i18n.Say("✓ registered 'gtmux hook' for Codex in "+tildeify(path)+" (UserPromptSubmit · PermissionRequest · Stop · Session start/end)",
		"✓ 已为 Codex 在 "+tildeify(path)+" 注册 'gtmux hook'（UserPromptSubmit · PermissionRequest · Stop · Session 开始/结束）")
	ensureCodexFeaturesHooks(codexConfigPath())
	i18n.Say("• coexists with any existing Codex `notify` (e.g. computer-use) — it is left untouched.",
		"• 与你现有的 Codex `notify`（如 computer-use）并存，保持不动。")
	if _, err := os.Stat(gtmuxAppPath()); err != nil {
		i18n.Say("• install the menu-bar app to get desktop notifications (curl installer, or 'make app')",
			"• 安装菜单栏 app 才能收到桌面通知（用 curl 安装脚本，或 'make app'）")
	}
	// Codex ≥ ~0.146 gates each newly-registered hook behind a one-time trust prompt
	// ("New hook - review required — press t to trust"). Until you trust them, the
	// hooks never fire and gtmux sees nothing (no waiting/done, no digest) — so call
	// this out explicitly, it's the difference between "installed" and "working".
	i18n.Say("• first launch: recent Codex asks you to review each hook — press 't' to trust the gtmux hooks (else they never fire).",
		"• 首次启动：较新版 Codex 会让你逐个确认 hook —— 按 't' 信任 gtmux 的 hook（否则不会触发）。")
	// Rewriting this file under a RUNNING Codex does not leave that session alone: it
	// reads hooks.json again at some point, finds it changed, and stops firing until the
	// new entries are trusted — with nothing on screen to say so. Measured 2026-08-29:
	// a reinstall at 23:57 left every Codex session firing normally for over two hours,
	// then silence from 02:13 across ALL panes, and an approval that sat unanswered for
	// six hours without gtmux ever knowing. "Restart Codex to load the hooks" did not
	// prepare anyone for that, because it reads as "the new ones arrive later", not as
	// "the ones you have stop".
	i18n.Say("Sessions already running keep the hooks they started with, and may stop firing them once Codex re-reads this file — restart them (and press 't') to be sure gtmux still sees them.",
		"已经在跑的会话仍用它启动时的那套 hook，而且 Codex 再次读到这个文件后可能就不再触发了 —— 重启它们（并按 't'）才能确保 gtmux 还看得见。")
	i18n.Say("Done. Restart Codex to load the hooks.", "完成。重启 Codex 以加载 hook。")
	return 0
}

// ensureCodexFeaturesHooks makes sure `features.hooks = true` is set — Codex won't
// load hooks.json without it (Feature::CodexHooks, config key "hooks"). Conservative:
// if it's already on, do nothing; if a [features] table exists, GUIDE the user rather
// than risk rewriting the table; otherwise append the dotted top-level key (backed up).
func ensureCodexFeaturesHooks(cfgPath string) {
	content := ""
	if b, err := os.ReadFile(cfgPath); err == nil {
		content = string(b)
	}
	if codexHooksFeatureEnabled(content) {
		i18n.Say("✓ features.hooks already enabled in "+tildeify(cfgPath),
			"✓ "+tildeify(cfgPath)+" 已启用 features.hooks")
		return
	}
	backupFile(cfgPath)
	// A [features] table already exists → insert (or flip) `hooks = true` UNDER it —
	// adding one key line beneath the header keeps the rest of the table intact.
	// Otherwise write the dotted top-level `features.hooks = true`. (The old code
	// only PRINTED guidance when a [features] table existed, so the doctor kept
	// reporting "not wired" even after --fix claimed success.)
	var updated string
	if tomlHasTable(content, "features") {
		updated = enableHooksUnderFeatures(content)
	} else {
		updated = insertTomlTopLevel(content, "features.hooks = true")
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		i18n.Sae("• could not enable features.hooks: "+err.Error(), "• 启用 features.hooks 失败："+err.Error())
		return
	}
	if err := os.WriteFile(cfgPath, []byte(updated), 0o644); err != nil {
		i18n.Sae("• could not write "+tildeify(cfgPath)+": "+err.Error(),
			"• 写入 "+tildeify(cfgPath)+" 失败："+err.Error())
		return
	}
	i18n.Say("✓ enabled features.hooks in "+tildeify(cfgPath), "✓ 已在 "+tildeify(cfgPath)+" 启用 features.hooks")
}

var reFeaturesHooksDotted = regexp.MustCompile(`(?m)^\s*features\.hooks\s*=\s*true\b`)
var reHooksTrue = regexp.MustCompile(`^hooks\s*=\s*true\b`)
var reHooksAssign = regexp.MustCompile(`^\s*hooks\s*=`)

// enableHooksUnderFeatures sets `hooks = true` inside an EXISTING [features] table:
// it flips an existing `hooks = …` line in that table, or inserts the key right
// after the `[features]` header. Only the [features] table is touched — foreign
// keys and other tables are preserved byte-for-byte.
func enableHooksUnderFeatures(content string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "[features]" {
			start = i
			break
		}
	}
	if start == -1 { // no table after all — fall back to the dotted top-level key
		return insertTomlTopLevel(content, "features.hooks = true")
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			end = i
			break
		}
	}
	for i := start + 1; i < end; i++ {
		if reHooksAssign.MatchString(lines[i]) {
			lines[i] = "hooks = true"
			return strings.Join(lines, "\n")
		}
	}
	out := append([]string{}, lines[:start+1]...)
	out = append(out, "hooks = true")
	out = append(out, lines[start+1:]...)
	return strings.Join(out, "\n")
}

// codexHooksFeatureEnabled reports whether `features.hooks = true` is set, in either
// the dotted top-level form (`features.hooks = true`) or under a `[features]` table.
func codexHooksFeatureEnabled(content string) bool {
	if reFeaturesHooksDotted.MatchString(content) {
		return true
	}
	inFeatures := false
	for _, ln := range strings.Split(content, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[") {
			inFeatures = t == "[features]"
			continue
		}
		if inFeatures && reHooksTrue.MatchString(t) {
			return true
		}
	}
	return false
}

// tomlHasTable reports whether content has a `[name]` table header.
func tomlHasTable(content, name string) bool {
	header := "[" + name + "]"
	for _, ln := range strings.Split(content, "\n") {
		if strings.TrimSpace(ln) == header {
			return true
		}
	}
	return false
}

// codexHooksHasGtmux reports whether Codex's hooks.json already has a gtmux hook
// entry (in any event), so wiring is idempotent and the doctor can detect it.
func codexHooksHasGtmux(path string) bool {
	m, err := loadJSONObject(path)
	if err != nil {
		return false
	}
	for _, v := range asObject(m["hooks"]) {
		for _, raw := range asArray(v) {
			grp, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			for _, h := range asArray(grp["hooks"]) {
				if hm, ok := h.(map[string]any); ok && isGtmuxHookCommand(asString(hm["command"])) {
					return true
				}
			}
		}
	}
	return false
}

// codexHooksWired reports whether the hooks system is fully wired to gtmux: the
// gtmux entries are present AND features.hooks is enabled (both are required to fire).
func codexHooksWired() bool {
	content := ""
	if b, err := os.ReadFile(codexConfigPath()); err == nil {
		content = string(b)
	}
	return codexHooksHasGtmux(agentInstallers["codex"].configPath()) && codexHooksFeatureEnabled(content)
}

// codexHooksStale reports whether gtmux's Codex hook entries are present but marked
// `async: true` — the shape older gtmux versions wrote. Codex 0.146.0 does NOT support
// async hooks and SKIPS them ("async hooks are not supported yet"), so such entries never
// fire and the receipt is dead even though codexHooksWired() reports "installed". gtmux now
// installs them SYNC, so a stale async entry must be reinstalled (doctor flags it, --fix
// rewrites it).
func codexHooksStale() bool {
	return codexHooksHasAsync(agentInstallers["codex"].configPath())
}

func codexHooksHasAsync(path string) bool {
	m, err := loadJSONObject(path)
	if err != nil {
		return false
	}
	for _, v := range asObject(m["hooks"]) {
		for _, raw := range asArray(v) {
			grp, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			for _, h := range asArray(grp["hooks"]) {
				hm, ok := h.(map[string]any)
				if !ok || !isGtmuxHookCommand(asString(hm["command"])) {
					continue
				}
				if a, ok := hm["async"].(bool); ok && a {
					return true
				}
			}
		}
	}
	return false
}

// missingAgentHookEvents lists the events gtmux WOULD register for an agent that its
// installed hooks file does not carry.
//
// "Installed" and "installed completely" are different facts, and doctor only ever
// checked the first. A hooks file written by an older gtmux keeps working and keeps
// reporting ✓ while missing every event added since — measured on this machine: Codex
// had 5 of the 9 it should have, so its tool and compaction events had been shipped for
// days and were reaching nobody. Nothing rewrites an agent's hooks file on update; that
// is `install hooks` alone, and a user has no reason to re-run it unprompted.
func missingAgentHookEvents(key string) []string {
	inst, ok := agentInstallers[key]
	if !ok {
		return nil
	}
	m, err := loadJSONObject(inst.configPath())
	if err != nil {
		return nil // no file at all is "not installed", which the caller reports already
	}
	have := map[string]bool{}
	for event, v := range asObject(m["hooks"]) {
		for _, raw := range asArray(v) {
			grp, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			for _, h := range asArray(grp["hooks"]) {
				if hm, ok := h.(map[string]any); ok && isGtmuxHookCommand(asString(hm["command"])) {
					have[event] = true
				}
			}
		}
	}
	// Flat-format agents (cursor) keep the command at the top level of each entry.
	for event, v := range asObject(m["hooks"]) {
		for _, raw := range asArray(v) {
			if e, ok := raw.(map[string]any); ok && isGtmuxHookCommand(asString(e["command"])) {
				have[event] = true
			}
		}
	}
	// No gtmux entry anywhere is NOT an incomplete install — it is no install, which the
	// caller already reports in its own words. Saying "9 events missing" there would be
	// a second, noisier claim about the same fact.
	if len(have) == 0 {
		return nil
	}
	var missing []string
	seen := map[string]bool{}
	for _, b := range inst.bindings {
		if have[b.key] || seen[b.key] {
			continue
		}
		seen[b.key] = true
		missing = append(missing, b.key)
	}
	sort.Strings(missing)
	return missing
}
