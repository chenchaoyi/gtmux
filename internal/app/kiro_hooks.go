package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

var kiroVersionPattern = regexp.MustCompile(`(?:^|\s)v?([0-9]+)\.[0-9]+`)

func kiroV3Version(version string) bool {
	m := kiroVersionPattern.FindStringSubmatch(version)
	if len(m) <= 1 {
		return false
	}
	major, err := strconv.Atoi(m[1])
	return err == nil && major >= 3
}

func kiroInstaller(v3 bool) agentInstaller {
	inst := agentInstallers["kiro"]
	if !v3 {
		return inst
	}
	inst.relDir, inst.envSubpath, inst.format = ".kiro/hooks", "hooks", formatKiroV3
	inst.bindings = []agentHookBinding{
		{key: "SessionStart", event: "SessionStart"},
		{key: "UserPromptSubmit", event: "UserPromptSubmit"},
		{key: "PreToolUse", event: "PreToolUse", feed: true},
		{key: "PostToolUse", event: "PostToolUse", feed: true},
		{key: "Stop", event: "Stop"},
	}
	inst.note = func() string {
		return i18n.Tr("Kiro V3 loads these global hooks for every agent profile. Restart running sessions to load them.", "Kiro V3 会为每个 agent 加载这些全局 hook；已运行的会话需要重启才能加载。")
	}
	return inst
}

func selectedAgentInstaller(key string) (agentInstaller, bool) {
	inst, ok := agentInstallers[key]
	if !ok || key != "kiro" {
		return inst, ok
	}
	if fileExists(kiroInstaller(true).configPath()) {
		return kiroInstaller(true), true
	}
	if profile, err := loadJSONObject(kiroInstaller(false).configPath()); err == nil {
		if _, migrated := profile["hooks"].([]any); migrated {
			return kiroInstaller(true), true
		}
	}
	if bin := lookTool("kiro-cli"); bin != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, bin, "--version").Output(); err == nil && kiroVersionPattern.Match(out) {
			return kiroInstaller(kiroV3Version(string(out))), true
		}
	}
	// An existing V3 integration remains V3 when version discovery is unavailable.
	return kiroInstaller(fileExists(kiroInstaller(true).configPath())), true
}

func updateKiroV3Settings(inst agentInstaller, path, bin string, install bool) error {
	m, err := loadJSONObject(path)
	if err != nil {
		return err
	}
	if hooks, exists := m["hooks"]; exists {
		if _, ok := hooks.([]any); !ok {
			return fmt.Errorf("kiro V3 hooks must be an array")
		}
	}
	backupFile(path)
	var hooks []any
	for _, raw := range asArray(m["hooks"]) {
		h, _ := raw.(map[string]any)
		action := asObject(h["action"])
		if !isGtmuxHookCommand(asString(action["command"])) {
			hooks = append(hooks, raw)
		}
	}
	if install {
		for _, b := range inst.bindings {
			hooks = append(hooks, inst.entry(bin, b))
		}
	}
	if len(hooks) == 0 {
		delete(m, "hooks")
		delete(m, "version")
		if len(m) == 0 {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
	} else {
		m["hooks"], m["version"] = hooks, "v1"
	}
	return writeJSONObject(path, m)
}
