package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// This file installs gtmux's `hook` into coding agents OTHER than Claude
// (gemini / cursor / copilot / kiro). The hook runtime already classifies these
// agents (internal/hook/classify.go); this is just the per-agent config WRITER.
//
// The shapes are ported from cmux's AgentHookDefinitions: each agent reads a
// JSON hooks file in one of three layouts (flat / nested / kiro-agent). We map
// each agent's NATIVE event key to the canonical gtmux hook token that the
// classifier expects, so a single `gtmux hook --agent <key> <token>` invocation
// per event drives the radar. Lifecycle tokens (UserPromptSubmit/Stop) and the
// always-blocking approval token (PermissionRequest) are resolved by the generic
// classifier table; tool-discriminated feed events (PreToolUse/PostToolUse,
// kiro's preToolUse/postToolUse) pass the agent's native name + payload through.

// agentHookFormat is the on-disk layout of an agent's hooks file.
type agentHookFormat int

const (
	// formatFlat: {"hooks":{"<evt>":[{"command":"…"}]},"version":1} — Cursor.
	formatFlat agentHookFormat = iota
	// formatNested: {"hooks":{"<evt>":[{"hooks":[{"type":"command","command":"…","timeout":N}]}]}}
	// — Gemini/Copilot (Claude-settings-shaped).
	formatNested
	// formatKiro: flat command entries carrying timeout_ms, plus an agent-def
	// wrapper (name/description/tools) — Kiro's ~/.kiro/agents/<name>.json.
	formatKiro
	// formatCopilot: {"version":1,"hooks":{"<Event>":[{"type":"command","bash":"…",
	// "timeoutSec":N}]}} in ~/.copilot/hooks/<file>.json (one file per tool) — Copilot
	// CLI. Distinct from formatNested: flat entries keyed on `bash`/`timeoutSec`.
	formatCopilot
	// formatCodex: Codex's hooks.json — SAME outer/inner nesting as formatNested
	// ({"hooks":{"<Event>":[{"hooks":[{type,command}]}]}}, verified against Codex's
	// HooksFile: events MUST sit under a top-level "hooks" object, root-level event
	// keys are rejected). Two differences from formatNested: Codex's `timeout` is in
	// SECONDS (not ms), and we set `async:true` so `gtmux hook` never blocks the turn
	// boundary. Enabled by `features.hooks = true` in ~/.codex/config.toml.
	formatCodex
)

// feedTimeoutMs is the long timeout for tool feed events (they can block on you).
const feedTimeoutMs = 120000

// agentHookBinding maps one native config event key to the gtmux hook token.
type agentHookBinding struct {
	key   string // the agent-native event key written in the config file
	event string // the token passed to `gtmux hook --agent <agent> <event>`
	feed  bool   // a tool feed event (long timeout) vs a lifecycle event
}

// agentInstaller describes how to write one agent's hooks file.
type agentInstaller struct {
	key        string // gtmux --agent key (also the cmux agent name)
	display    string
	relDir     string // path under ~ , e.g. ".cursor"
	file       string // file name, e.g. "hooks.json"
	envDir     string // env var that overrides ~/relDir (optional)
	envSubpath string // appended to the env-override dir (optional)
	format     agentHookFormat
	timeoutMs  int                // lifecycle hook timeout (formatNested/formatKiro)
	dedicated  bool               // the file is wholly gtmux's → remove it on uninstall
	bindings   []agentHookBinding // native event key → gtmux token
	note       func() string      // optional post-install note
}

// agentInstallers are the non-Claude agents `install-hooks --agent <key>` can wire.
var agentInstallers = map[string]agentInstaller{
	"codex": {
		// Codex's hooks system (~/.codex/hooks.json + features.hooks=true) — the
		// additive path that COEXISTS with an existing `notify` (e.g. computer-use),
		// unlike the single-slot legacy notify. hooks.json is the nested shape:
		//   "<Event>": [ { "hooks": [ { "type":"command", "command":… } ] } ].
		// The features.hooks toml flag is handled separately (installCodexHooks).
		key: "codex", display: "Codex", relDir: ".codex", file: "hooks.json",
		envDir: "CODEX_HOME", format: formatCodex, timeoutMs: 10000,
		bindings: []agentHookBinding{
			{key: "UserPromptSubmit", event: "UserPromptSubmit"},
			{key: "PermissionRequest", event: "PermissionRequest"}, // needs-you approval
			{key: "Stop", event: "Stop"},
			{key: "SessionStart", event: "SessionStart"},
			{key: "SessionEnd", event: "SessionEnd"},
		},
	},
	"cursor": {
		key: "cursor", display: "Cursor", relDir: ".cursor", file: "hooks.json",
		envDir: "CURSOR_HOME", format: formatFlat,
		bindings: []agentHookBinding{
			{key: "beforeSubmitPrompt", event: "UserPromptSubmit"},
			{key: "afterAgentResponse", event: "Stop"},
			{key: "beforeShellExecution", event: "PermissionRequest"}, // a shell is always blocking
			{key: "afterShellExecution", event: "PostToolUse"},        // resolved → clear waiting
			{key: "beforeMCPExecution", event: "PermissionRequest"},   // an MCP tool also blocks on you
			{key: "afterMCPExecution", event: "PostToolUse"},          // resolved → clear waiting
			{key: "sessionEnd", event: "SessionEnd"},                  // clear markers when the conversation ends
		},
	},
	"gemini": {
		key: "gemini", display: "Gemini", relDir: ".gemini", file: "settings.json",
		format: formatNested, timeoutMs: 10000,
		bindings: []agentHookBinding{
			{key: "BeforeAgent", event: "UserPromptSubmit"},
			{key: "AfterAgent", event: "Stop"},
			// Gemini names its tool hooks Before/After Tool, NOT Pre/Post — the old
			// Pre/PostToolUse keys silently never fired. The gtmux TOKEN stays
			// canonical (Pre/PostToolUse) so the classifier still handles it.
			{key: "BeforeTool", event: "PreToolUse", feed: true},
			{key: "AfterTool", event: "PostToolUse", feed: true},
			{key: "SessionStart", event: "SessionStart"}, // clear markers on (re)start
			{key: "SessionEnd", event: "SessionEnd"},     // clear markers on end
		},
	},
	"copilot": {
		// Copilot CLI reads ~/.copilot/hooks/<file>.json (one file per source),
		// NOT ~/.copilot/config.json — the old path/format silently never fired.
		// Shape is {"version":1,"hooks":{"<Event>":[{"type":"command","bash":…,
		// "timeoutSec":N}]}} with PascalCase event keys. A dedicated gtmux file.
		key: "copilot", display: "Copilot", relDir: ".copilot/hooks", file: "gtmux.json",
		envDir: "COPILOT_HOME", envSubpath: "hooks", format: formatCopilot, timeoutMs: 5000,
		dedicated: true,
		bindings: []agentHookBinding{
			{key: "UserPromptSubmit", event: "UserPromptSubmit"},
			{key: "Stop", event: "Stop"},
			{key: "PermissionRequest", event: "PermissionRequest"}, // Copilot's own approval event
			{key: "SessionStart", event: "SessionStart"},
			{key: "SessionEnd", event: "SessionEnd"},
		},
	},
	"kiro": {
		key: "kiro", display: "Kiro", relDir: ".kiro/agents", file: "gtmux.json",
		envDir: "KIRO_HOME", envSubpath: "agents", format: formatKiro, timeoutMs: 5000,
		dedicated: true,
		bindings: []agentHookBinding{
			{key: "userPromptSubmit", event: "UserPromptSubmit"},
			{key: "stop", event: "Stop"},
			{key: "preToolUse", event: "preToolUse", feed: true},   // kiro classifier table
			{key: "postToolUse", event: "postToolUse", feed: true}, // resolved → clear waiting
		},
		note: func() string {
			return i18n.Tr(
				"Kiro applies these only when run as the gtmux agent. Start it with `kiro-cli chat --agent gtmux`, or set it default: `kiro-cli settings chat.defaultAgent gtmux`.",
				"Kiro 仅在以 gtmux agent 运行时应用这些 hook。用 `kiro-cli chat --agent gtmux` 启动，或设为默认：`kiro-cli settings chat.defaultAgent gtmux`。")
		},
	},
}

// configPath resolves where this agent's hooks file lives, honoring its env override.
func (inst agentInstaller) configPath() string {
	if inst.envDir != "" {
		if v := os.Getenv(inst.envDir); v != "" {
			base := v
			if inst.envSubpath != "" {
				base = filepath.Join(base, inst.envSubpath)
			}
			return filepath.Join(base, inst.file)
		}
	}
	return filepath.Join(homeDir(), inst.relDir, inst.file)
}

// hookCommand is the command string written for one binding.
func (inst agentInstaller) hookCommand(bin string, b agentHookBinding) string {
	return fmt.Sprintf("%s hook --agent %s %s", bin, inst.key, b.event)
}

// timeoutFor returns the timeout (ms) for a binding: long for feed events.
func (inst agentInstaller) timeoutFor(b agentHookBinding) int {
	if b.feed {
		return feedTimeoutMs
	}
	return inst.timeoutMs
}

// installAgentHooks writes (or removes) one non-Claude agent's hooks file.
// Idempotent and reversible, preserving any foreign hooks already present.
func installAgentHooks(inst agentInstaller, yes bool) int {
	bin := selfPath()
	path := inst.configPath()
	if err := updateAgentSettings(inst, path, bin, true); err != nil {
		i18n.Sae(fmt.Sprintf("failed to update %s: %v", path, err),
			fmt.Sprintf("更新 %s 失败：%v", path, err))
		return 1
	}
	i18n.Say(fmt.Sprintf("✓ registered 'gtmux hook' for %s in %s", inst.display, path),
		fmt.Sprintf("✓ 已为 %s 在 %s 注册 'gtmux hook'", inst.display, path))
	if inst.note != nil {
		i18n.Say("• "+inst.note(), "• "+inst.note())
	}
	if _, err := os.Stat(gtmuxAppPath()); err != nil {
		i18n.Say("• install the menu-bar app to get desktop notifications (curl installer, or 'make app')",
			"• 安装菜单栏 app 才能收到桌面通知（用 curl 安装脚本，或 'make app'）")
	}
	i18n.Say(fmt.Sprintf("Done. Restart %s to load the hooks.", inst.display),
		fmt.Sprintf("完成。重启 %s 以加载 hook。", inst.display))
	return 0
}

// uninstallAgentHooks reverses installAgentHooks for one agent.
func uninstallAgentHooks(inst agentInstaller) int {
	path := inst.configPath()
	if err := updateAgentSettings(inst, path, "", false); err != nil {
		i18n.Sae(fmt.Sprintf("failed to update %s: %v", path, err),
			fmt.Sprintf("更新 %s 失败：%v", path, err))
		return 1
	}
	i18n.Say(fmt.Sprintf("✓ de-registered 'gtmux hook' for %s from %s", inst.display, path),
		fmt.Sprintf("✓ 已为 %s 从 %s 注销 'gtmux hook'", inst.display, path))
	return 0
}

// updateAgentSettings merges (install) or strips (uninstall) gtmux's entries in
// the agent's hooks file, preserving foreign entries and the file's other keys.
func updateAgentSettings(inst agentInstaller, path, bin string, install bool) error {
	m, err := loadJSONObject(path)
	if err != nil {
		return err
	}
	backupFile(path) // no-op if absent

	hooks := asObject(m["hooks"])
	for _, b := range inst.bindings {
		list := removeAgentEntries(asArray(hooks[b.key]), inst.format)
		if install {
			list = append(list, inst.entry(bin, b))
		}
		if len(list) == 0 {
			delete(hooks, b.key)
		} else {
			hooks[b.key] = list
		}
	}

	// A dedicated, gtmux-only file with nothing left → remove it outright.
	if inst.dedicated && len(hooks) == 0 {
		_ = os.Remove(path)
		return nil
	}

	if len(hooks) == 0 {
		delete(m, "hooks")
	} else {
		m["hooks"] = hooks
	}
	applyFormatWrapper(inst, m, install, len(hooks) > 0)
	if len(m) == 0 {
		_ = os.Remove(path)
		return nil
	}
	return writeJSONObject(path, m)
}

// entry builds the install entry for a binding in the installer's format.
func (inst agentInstaller) entry(bin string, b agentHookBinding) map[string]any {
	cmd := inst.hookCommand(bin, b)
	switch inst.format {
	case formatNested:
		return map[string]any{"hooks": []any{map[string]any{
			"type": "command", "command": cmd, "timeout": inst.timeoutFor(b),
		}}}
	case formatCodex:
		// Same nesting as formatNested, but Codex's `timeout` is in SECONDS, and
		// async:true keeps the fire-and-forget `gtmux hook` off the turn's critical path.
		return map[string]any{"hooks": []any{map[string]any{
			"type": "command", "command": cmd, "timeout": inst.timeoutFor(b) / 1000, "async": true,
		}}}
	case formatKiro:
		return map[string]any{"command": cmd, "timeout_ms": inst.timeoutFor(b)}
	case formatCopilot:
		// Copilot's handler is a flat {type, bash, timeoutSec} — timeout in SECONDS.
		return map[string]any{"type": "command", "bash": cmd, "timeoutSec": inst.timeoutFor(b) / 1000}
	default: // formatFlat
		return map[string]any{"command": cmd}
	}
}

// removeAgentEntries strips gtmux commands from one event's entry list, dropping
// emptied entries and keeping everything foreign — in the given format's shape.
func removeAgentEntries(list []any, format agentHookFormat) []any {
	if format == formatNested || format == formatCodex {
		return removeGtmuxEntries(list) // {"hooks":[{"command":…}]} shape
	}
	// flat / kiro carry the command in "command"; copilot carries it in "bash".
	cmdKey := "command"
	if format == formatCopilot {
		cmdKey = "bash"
	}
	out := make([]any, 0, len(list))
	for _, raw := range list {
		entry, ok := raw.(map[string]any)
		if ok && isGtmuxHookCommand(asString(entry[cmdKey])) {
			continue
		}
		out = append(out, raw)
	}
	return out
}

// applyFormatWrapper sets/clears the file-root keys a format requires beside the
// hooks map: flat's version, kiro's agent-def fields (only when absent).
func applyFormatWrapper(inst agentInstaller, m map[string]any, install, haveHooks bool) {
	switch inst.format {
	case formatFlat, formatCopilot:
		if install && haveHooks {
			m["version"] = 1
		} else if !haveHooks {
			delete(m, "version")
		}
	case formatKiro:
		if install {
			if _, ok := m["name"]; !ok {
				m["name"] = "gtmux"
			}
			if _, ok := m["description"]; !ok {
				m["description"] = "gtmux notification + radar bridge hooks for Kiro CLI."
			}
			if _, ok := m["tools"]; !ok {
				m["tools"] = []any{"*"}
			}
		}
	}
}
