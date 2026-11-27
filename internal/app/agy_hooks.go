package app

import (
	"fmt"
	"os"
)

// Antigravity CLI (`agy`) reads global lifecycle hooks from ~/.gemini/config/hooks.json.
// Unlike Claude/Gemini/Codex (whose events sit under a top-level "hooks" map), agy's
// hooks.json maps each top-level hook NAME (here "gtmux") to an event spec object:
//
//   - Flat events (SessionStart, PreInvocation, PostInvocation, Stop):
//     [{"type": "command", "command": "...", "timeout": 10}]
//   - Grouped tool events (PreToolUse, PostToolUse):
//     [{"matcher": "*", "hooks": [{"type": "command", "command": "...", "timeout": 10}]}]
//
// PreToolUse is intentionally NOT registered: agy requires PreToolUse hooks to emit an
// explicit permission verdict ("allow"|"deny"|"ask"|"force_ask"), so an observational
// bridge must stay off PreToolUse to avoid altering the user's tool approval policy.
const (
	agyHookName       = "gtmux"
	agyHookTimeoutSec = 10
)

func agyEventGrouped(eventKey string) bool {
	return eventKey == "PreToolUse" || eventKey == "PostToolUse"
}

func agyBindingEntry(inst agentInstaller, bin string, b agentHookBinding) map[string]any {
	handler := map[string]any{
		"type":    "command",
		"command": inst.hookCommand(bin, b),
		"timeout": agyHookTimeoutSec,
	}
	if agyEventGrouped(b.key) {
		return map[string]any{
			"matcher": "*",
			"hooks":   []any{handler},
		}
	}
	return handler
}

// updateAgyHooks writes or removes only the top-level "gtmux" entry in hooks.json,
// leaving all other named hooks untouched and backing up the file first.
func updateAgyHooks(inst agentInstaller, path, bin string, install bool) error {
	m, err := loadJSONObject(path)
	if err != nil {
		return err
	}
	if existing, ok := m[agyHookName]; ok {
		if _, valid := existing.(map[string]any); !valid {
			return fmt.Errorf("%s entry in hooks.json must be an object", agyHookName)
		}
	}
	backupFile(path)

	if !install {
		delete(m, agyHookName)
		if len(m) == 0 {
			_ = os.Remove(path)
			return nil
		}
		return writeJSONObject(path, m)
	}

	entry := map[string]any{}
	for _, b := range inst.bindings {
		entry[b.key] = []any{agyBindingEntry(inst, bin, b)}
	}
	m[agyHookName] = entry
	return writeJSONObject(path, m)
}

func agyHookPolicyDisabled(path string) bool {
	m, err := loadJSONObject(path)
	if err != nil {
		return false
	}
	entry := asObject(m[agyHookName])
	return entry["enabled"] == false
}

func agyHooksInstalled(path string) bool {
	return len(agyHookEventsIn(path)) > 0
}

func agyHookEventsIn(path string) map[string]bool {
	have := map[string]bool{}
	m, err := loadJSONObject(path)
	if err != nil {
		return have
	}
	entry := asObject(m[agyHookName])
	for event, rawList := range entry {
		if event == "enabled" {
			continue
		}
		for _, raw := range asArray(rawList) {
			obj := asObject(raw)
			if isGtmuxHookCommand(asString(obj["command"])) {
				have[event] = true
			}
			for _, sub := range asArray(obj["hooks"]) {
				if isGtmuxHookCommand(asString(asObject(sub)["command"])) {
					have[event] = true
				}
			}
		}
	}
	return have
}

func agyInstalledBindings(inst agentInstaller) map[string]bool {
	have := map[string]bool{}
	m, err := loadJSONObject(inst.configPath())
	if err != nil {
		return have
	}
	entry := asObject(m[agyHookName])
	if len(entry) == 0 || entry["enabled"] == false {
		return have
	}
	for _, b := range inst.bindings {
		for _, raw := range asArray(entry[b.key]) {
			obj := asObject(raw)
			if obj["enabled"] == false {
				continue
			}
			if agyEventGrouped(b.key) {
				matcher := asString(obj["matcher"])
				if matcher != "" && matcher != "*" {
					continue
				}
				for _, sub := range asArray(obj["hooks"]) {
					h := asObject(sub)
					t := asString(h["type"])
					if h["enabled"] == false || (t != "" && t != "command") {
						continue
					}
					if hookCommandMatches(asString(h["command"]), inst.key, b.event) {
						have[b.key] = true
					}
				}
				continue
			}
			t := asString(obj["type"])
			if t != "" && t != "command" {
				continue
			}
			if hookCommandMatches(asString(obj["command"]), inst.key, b.event) {
				have[b.key] = true
			}
		}
	}
	return have
}
