package app

import (
	"encoding/json"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"os"
	"path/filepath"
	"strings"
)

var additionalHookAgents = func() []string {
	var keys []string
	for _, key := range agents.HookEquippedKeys() {
		manifest, _ := agents.For(key)
		if key != manifest.Key {
			continue
		}
		if key != "claude" && key != "codex" && key != "kimi" {
			keys = append(keys, key)
		}
	}
	return keys
}()

// Inspect existing homes and binaries without creating configuration for absent agents.
func agentHookPresent(key string) bool {
	inst, ok := selectedAgentInstaller(key)
	if !ok {
		return false
	}
	if inst.envDir != "" && os.Getenv(inst.envDir) != "" {
		return fileExists(os.Getenv(inst.envDir))
	}
	root := filepath.Join(homeDir(), strings.Split(inst.relDir, "/")[0])
	if key == "opencode" {
		root = filepath.Join(homeDir(), ".config", "opencode")
	}
	if fileExists(root) {
		return true
	}
	if key == "kiro" {
		return lookTool("kiro-cli") != ""
	}
	if key == "copilot" {
		return lookTool("copilot") != ""
	}
	if manifest, ok := agents.For(key); ok {
		for _, bin := range manifest.Detect {
			if lookTool(bin) != "" {
				return true
			}
		}
	}
	return false
}

func agentHooksInstalled(inst agentInstaller) bool {
	if inst.plugin != nil {
		b, err := os.ReadFile(inst.configPath())
		return (err == nil && strings.Contains(string(b), "export const GtmuxHook")) || ownedOpencodePlugin(legacyOpencodePluginPath())
	}
	m, err := loadJSONObject(inst.configPath())
	if err != nil {
		return false
	}
	if inst.format == formatKiroV3 {
		for _, raw := range asArray(m["hooks"]) {
			h := asObject(raw)
			if isGtmuxHookCommand(asString(asObject(h["action"])["command"])) {
				return true
			}
		}
		return false
	}
	return len(gtmuxHookEventsIn(inst.configPath())) > 0
}

func claudeHookCommandMatches(command, event string) bool {
	if !isGtmuxHookCommand(command) {
		return false
	}
	fields := strings.Fields(command)
	// Claude can supply hook_event_name on stdin, so its installed bare hook
	// command is valid; explicit event arguments must agree with the registration.
	return len(fields) == 2 || (len(fields) == 3 && fields[2] == event) ||
		(len(fields) == 5 && fields[2] == "--agent" && fields[3] == "claude" && fields[4] == event)
}

func hookCommandMatches(command, agent, event string) bool {
	return isGtmuxHookCommand(command) && strings.Contains(command, " hook --agent "+agent+" "+event) &&
		strings.HasSuffix(strings.TrimSpace(command), " "+event)
}

// Check the executable handler and expected token, not just an event name or a
// foreign hook. Disabled and scoped handlers do not satisfy an unscoped binding.
func installedAgentBindings(inst agentInstaller) map[string]bool {
	have := map[string]bool{}
	m, err := loadJSONObject(inst.configPath())
	if err != nil {
		return have
	}
	if (inst.format == formatFlat || inst.format == formatCopilot) && m["version"] != float64(1) {
		return have
	}
	if inst.format == formatKiroV3 && asString(m["version"]) != "v1" {
		return have
	}
	if hookPolicyDisabled(inst.key, inst.configPath()) {
		return have
	}
	if inst.format == formatKiroV3 {
		for _, raw := range asArray(m["hooks"]) {
			h := asObject(raw)
			if h["enabled"] == false || asString(h["matcher"]) != "" {
				continue
			}
			for _, b := range inst.bindings {
				a := asObject(h["action"])
				if asString(h["trigger"]) == b.key && asString(a["type"]) == "command" && hookCommandMatches(asString(a["command"]), inst.key, b.event) {
					have[b.key] = true
				}
			}
		}
		return have
	}
	for _, b := range inst.bindings {
		for _, raw := range asArray(asObject(m["hooks"])[b.key]) {
			h := asObject(raw)
			if h["enabled"] == false || asString(h["matcher"]) != "" {
				continue
			}
			if inst.format == formatNested || inst.format == formatCodex {
				for _, rawHandler := range asArray(h["hooks"]) {
					handler := asObject(rawHandler)
					if handler["enabled"] != false && asString(handler["type"]) == "command" && hookCommandMatches(asString(handler["command"]), inst.key, b.event) {
						have[b.key] = true
					}
				}
			} else {
				command := asString(h["command"])
				if inst.format == formatCopilot {
					if asString(h["type"]) != "command" {
						continue
					}
					command = asString(h["bash"])
				}
				if hookCommandMatches(command, inst.key, b.event) {
					have[b.key] = true
				}
			}
		}
	}
	return have
}

func agentPluginCurrent(inst agentInstaller) bool {
	b, err := os.ReadFile(inst.configPath())
	if err != nil {
		return false
	}
	// Preserve the installed executable location during template comparison.
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "const BIN = ") {
			var bin string
			quoted := strings.TrimSuffix(strings.TrimPrefix(line, "const BIN = "), ";")
			if json.Unmarshal([]byte(quoted), &bin) == nil {
				return string(b) == inst.plugin(bin)
			}
		}
	}
	return false
}

// A user's global off switch is a policy choice, not a missing binding.
func hookPolicyDisabled(key, path string) bool {
	m, err := loadJSONObject(path)
	if err != nil {
		return false
	}
	if m["disableAllHooks"] == true {
		return true
	}
	if key == "gemini" {
		config := asObject(m["hooksConfig"])
		if config["enabled"] == false {
			return true
		}
		for _, disabled := range asArray(config["disabled"]) {
			for _, groups := range asObject(m["hooks"]) {
				for _, raw := range asArray(groups) {
					for _, handler := range asArray(asObject(raw)["hooks"]) {
						h := asObject(handler)
						if isGtmuxHookCommand(asString(h["command"])) && (asString(disabled) == asString(h["name"]) || asString(disabled) == asString(h["command"])) {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func rowAgentHook(key string) dcheck {
	inst, _ := selectedAgentInstaller(key)
	label := inst.display + " hook"
	if hookPolicyDisabled(key, inst.configPath()) {
		return dcheck{stRec, label, i18n.Tr("disabled by settings", "配置已禁用"), i18n.Tr("enable hooks in the agent settings; doctor preserves this choice", "在 agent 设置中启用 hook；doctor 会保留这一选择")}
	}
	if _, err := loadJSONObject(inst.configPath()); err != nil && inst.plugin == nil {
		return dcheck{stRec, label, i18n.Tr("invalid configuration", "配置无法读取"), i18n.Tr("repair the configuration file before reinstalling hooks", "先修复配置文件，再重新安装 hook")}
	}
	if !agentHooksInstalled(inst) {
		return dcheck{stRec, label, i18n.Tr("not installed", "未装"), i18n.Tr("gtmux doctor --fix installs the state hooks", "gtmux doctor --fix 可安装状态 hook")}
	}
	if inst.plugin != nil {
		if !agentPluginCurrent(inst) {
			return dcheck{stRec, label, i18n.Tr("outdated bridge", "桥接插件需要更新"), i18n.Tr("gtmux doctor --fix updates the gtmux plugin", "gtmux doctor --fix 可更新 gtmux 插件")}
		}
	} else if missing := missingAgentHookEvents(key); len(missing) > 0 {
		return dcheck{stRec, label, fmt.Sprintf(i18n.Tr("%d events need repair", "%d 个事件需要修复"), len(missing)), strings.Join(missing, ", ") + "; gtmux doctor --fix"}
	}
	note := i18n.Tr("configured; running sessions must load it", "配置齐全；运行中的会话需加载")
	if key == "kiro" && inst.format == formatKiro {
		note = i18n.Tr("CLI 2.x profile: run with --agent gtmux", "CLI 2.x 配置：用 --agent gtmux 启动")
	}
	return dcheck{stOK, label, i18n.Tr("installed", "已装"), note}
}

func (s *fixState) stepAgentHook(key string) int {
	if !agentHookPresent(key) {
		return 0
	}
	inst, _ := selectedAgentInstaller(key)
	if hookPolicyDisabled(key, inst.configPath()) {
		i18n.Sae("  ⚠ "+inst.display+" hooks are disabled by settings; enable them in the agent first", "  ⚠ "+inst.display+" 的配置已禁用 hook；请先在 agent 中启用")
		s.rc = 1
		return 0
	}
	if rowAgentHook(key).status == stOK {
		return 0
	}
	detail := i18n.Tr("  Update gtmux state hooks in "+tildeify(inst.configPath())+". Foreign hooks are preserved; the file is backed up first. Restart running sessions to load it.", "  更新 "+tildeify(inst.configPath())+" 中 gtmux 的状态 hook；保留其它 hook，写入前先备份。运行中的会话需要重启才能加载。")
	if !s.ask(inst.display+" hook", detail) {
		return 0
	}
	var err error
	if inst.plugin != nil {
		err = installOpencodePlugin(inst, s.hookBin())
	} else {
		err = updateAgentSettings(inst, inst.configPath(), s.hookBin(), true)
	}
	if err != nil {
		i18n.Sae("  ✗ "+err.Error(), "  ✗ "+err.Error())
		s.rc = 1
		return 0
	}
	if rowAgentHook(key).status != stOK {
		s.rc = 1
		return 0
	}
	i18n.Say("  ✓ configured. Restart running sessions to load it", "  ✓ 已配置。运行中的会话需要重启才能加载")
	if inst.note != nil {
		fmt.Println(inst.note())
	}
	return 1
}
