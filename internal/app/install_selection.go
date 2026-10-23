package app

import (
	"fmt"
	"sort"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func installableHookAgentKeys() []string {
	keys := []string{"claude", "kimi"}
	for key := range agentInstallers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func detectedHookAgents() []string {
	var keys []string
	for _, key := range installableHookAgentKeys() {
		present := agentHookPresent(key)
		if present {
			keys = append(keys, key)
		}
	}
	return keys
}

func hookSelectionDisabled(key string) bool {
	if key == "claude" {
		return hookPolicyDisabled(key, claudeSettingsPath())
	}
	if inst, ok := selectedAgentInstaller(key); ok {
		return hookPolicyDisabled(key, inst.configPath())
	}
	return false
}

// Resolve selection once. Each provider keeps its explicit installer and policy;
// discovery is read-only and automatic setup preserves disabled hook settings.
func installDetectedHooks(keys, args []string, disabled func(string) bool, run func([]string) int) int {
	if len(keys) == 0 {
		i18n.Say("No supported agent CLIs detected.", "未检测到支持的 agent CLI。")
		return 0
	}
	var common []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--agent" {
			if i+1 < len(args) {
				i++
			}
			continue
		}
		common = append(common, args[i])
	}
	rc := 0
	for _, key := range keys {
		if disabled(key) {
			i18n.Say("Skipping "+key+": hooks are disabled by settings.", "跳过 "+key+"：配置已禁用 hook。")
			continue
		}
		manifest, _ := agents.For(key)
		fmt.Println(i18n.Tr("Installing hooks for ", "正在安装 hook：") + manifest.Label)
		perAgent := append(append([]string(nil), common...), "--agent", key)
		if run(perAgent) != 0 {
			rc = 1
		}
	}
	return rc
}
