package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func legacyOpencodePluginPath() string {
	return filepath.Join(homeDir(), ".config", "opencode", "plugin", "gtmux.js")
}

func ownedOpencodePlugin(path string) bool {
	raw, err := os.ReadFile(path)
	return err == nil && strings.HasPrefix(string(raw), "// gtmux hook bridge for opencode") && strings.Contains(string(raw), "export const GtmuxHook")
}

func installOpencodePlugin(inst agentInstaller, bin string) error {
	path := inst.configPath()
	if fileExists(path) && !ownedOpencodePlugin(path) {
		return fmt.Errorf("refusing to replace an unrecognized plugin at %s", path)
	}
	backupFile(path)
	if err := writeAgentPlugin(path, inst.plugin(bin)); err != nil {
		return err
	}
	legacy := legacyOpencodePluginPath()
	if legacy != path && ownedOpencodePlugin(legacy) {
		backupFile(legacy)
		return os.Remove(legacy)
	}
	return nil
}

func removeOpencodePlugins(inst agentInstaller) error {
	for _, path := range []string{inst.configPath(), legacyOpencodePluginPath()} {
		if !fileExists(path) {
			continue
		}
		if !ownedOpencodePlugin(path) {
			return fmt.Errorf("refusing to remove an unrecognized plugin at %s", path)
		}
		backupFile(path)
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// V3 still understands V2 agent-profile hooks. Remove only gtmux's old bindings
// when installing globals, otherwise --agent gtmux runs two copies of each hook.
func updateKiroHooks(inst agentInstaller, path, bin string, install bool) error {
	legacy := kiroInstaller(false)
	legacyPath := legacy.configPath()
	if legacyPath != path && fileExists(legacyPath) {
		m, err := loadJSONObject(legacyPath)
		if err != nil {
			return err
		}
		if value, ok := m["hooks"]; ok {
			switch value.(type) {
			case map[string]any, []any:
			default:
				return fmt.Errorf("unrecognized Kiro profile hooks format")
			}
		}
	}
	if err := updateKiroV3Settings(inst, path, bin, install); err != nil {
		return err
	}
	if legacyPath != path && fileExists(legacyPath) {
		return stripLegacyKiroProfile(legacy, legacyPath)
	}
	return nil
}

func stripLegacyKiroProfile(inst agentInstaller, path string) error {
	m, err := loadJSONObject(path)
	if err != nil {
		return err
	}
	if list, ok := m["hooks"].([]any); ok {
		kept := []any{}
		for _, raw := range list {
			if !isGtmuxHookCommand(asString(asObject(asObject(raw)["action"])["command"])) {
				kept = append(kept, raw)
			}
		}
		if len(kept) == len(list) {
			return nil
		}
		backupFile(path)
		if len(kept) == 0 {
			delete(m, "hooks")
		} else {
			m["hooks"] = kept
		}
		return writeJSONObject(path, m)
	}
	return updateAgentSettings(inst, path, "", false)
}
