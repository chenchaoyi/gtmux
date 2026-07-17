package app

import (
	"os"
	"path/filepath"
	"regexp"
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
