package app

import (
	"os"
	"strings"
	"testing"
)

func TestDoctorRejectsWrongDisabledAndScopedBindings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"codex", "cursor", "gemini", "copilot", "kiro"} {
		t.Run(key, func(t *testing.T) {
			inst := agentInstallers[key]
			if err := updateAgentSettings(inst, inst.configPath(), "/opt/gtmux", true); err != nil {
				t.Fatal(err)
			}
			if missing := missingAgentHookEvents(key); len(missing) != 0 {
				t.Fatal(missing)
			}
			m, _ := loadJSONObject(inst.configPath())
			hooks := asObject(m["hooks"])
			binding := inst.bindings[0]
			entry := asObject(asArray(hooks[binding.key])[0])
			switch inst.format {
			case formatNested, formatCodex:
				asObject(asArray(entry["hooks"])[0])["command"] = "/opt/gtmux hook --agent " + key + " StopFailure"
			case formatCopilot:
				entry["bash"] = "/opt/gtmux hook --agent copilot StopFailure"
			default:
				entry["command"] = "/opt/gtmux hook --agent " + key + " StopFailure"
			}
			if err := writeJSONObject(inst.configPath(), m); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(missingAgentHookEvents(key), ","), binding.key) {
				t.Fatal("wrong command passed doctor")
			}
			entry["enabled"] = false
			if err := writeJSONObject(inst.configPath(), m); err != nil {
				t.Fatal(err)
			}
			if len(missingAgentHookEvents(key)) == 0 {
				t.Fatal("disabled handler passed doctor")
			}
			if err := updateAgentSettings(inst, inst.configPath(), "/opt/gtmux", true); err != nil {
				t.Fatal(err)
			}
			if len(missingAgentHookEvents(key)) != 0 {
				t.Fatal("repair remained incomplete")
			}
		})
	}
}

func TestKiroV3MigrationPreservesForeignHooksAndRemovesDuplicateProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := kiroInstaller(false)
	if err := updateAgentSettings(old, old.configPath(), "/opt/gtmux", true); err != nil {
		t.Fatal(err)
	}
	m, _ := loadJSONObject(old.configPath())
	asObject(m["hooks"])["stop"] = append(asArray(asObject(m["hooks"])["stop"]), map[string]any{"command": "echo foreign", "timeout_ms": 1000})
	if err := writeJSONObject(old.configPath(), m); err != nil {
		t.Fatal(err)
	}
	inst := kiroInstaller(true)
	for range 2 {
		if err := updateAgentSettings(inst, inst.configPath(), "/opt/gtmux", true); err != nil {
			t.Fatal(err)
		}
	}
	if have := installedAgentBindings(inst); len(have) != 5 {
		t.Fatalf("v3 bindings: %v", have)
	}
	raw, _ := os.ReadFile(old.configPath())
	if strings.Contains(string(raw), "gtmux hook") || !strings.Contains(string(raw), "echo foreign") {
		t.Fatal(string(raw))
	}
	for _, version := range []string{"kiro-cli 3.0.0", "v3.1.0", "kiro-cli 10.0.0"} {
		if !kiroV3Version(version) {
			t.Fatal(version)
		}
	}
	if kiroV3Version("kiro-cli 2.9.0") {
		t.Fatal("legacy version selected v3")
	}
	if err := updateAgentSettings(inst, inst.configPath(), "", false); err != nil {
		t.Fatal(err)
	}
	if fileExists(inst.configPath()) {
		t.Fatal("v3 hooks survived uninstall")
	}
}

func TestOpencodePluginMigrationAndDoctorFreshness(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	inst := agentInstallers["opencode"]
	legacy := legacyOpencodePluginPath()
	if err := writeAgentPlugin(legacy, "// gtmux hook bridge for opencode\nexport const GtmuxHook = async () => ({});\n"); err != nil {
		t.Fatal(err)
	}
	if rowAgentHook("opencode").status == stOK {
		t.Fatal("obsolete legacy bridge passed doctor")
	}
	if err := installOpencodePlugin(inst, "/opt/gtmux"); err != nil {
		t.Fatal(err)
	}
	if fileExists(legacy) || rowAgentHook("opencode").status != stOK {
		t.Fatal("plugin migration not healthy")
	}
	if err := os.WriteFile(inst.configPath(), []byte("// user-owned plugin"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := installOpencodePlugin(inst, "/opt/gtmux"); err == nil {
		t.Fatal("overwrote foreign plugin")
	}
	raw, _ := os.ReadFile(inst.configPath())
	if string(raw) != "// user-owned plugin" {
		t.Fatal("foreign bytes changed")
	}
}

func TestDoctorPreservesGlobalDisablePolicyAndMalformedHookShapes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	inst := agentInstallers["gemini"]
	if err := updateAgentSettings(inst, inst.configPath(), "/opt/gtmux", true); err != nil {
		t.Fatal(err)
	}
	m, _ := loadJSONObject(inst.configPath())
	m["hooksConfig"] = map[string]any{"enabled": false}
	if err := writeJSONObject(inst.configPath(), m); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(inst.configPath())
	s := &fixState{yes: true, bin: "/opt/gtmux"}
	if rowAgentHook("gemini").status == stOK || s.stepAgentHook("gemini") != 0 || s.rc != 1 {
		t.Fatal("disabled policy reported as repaired")
	}
	after, _ := os.ReadFile(inst.configPath())
	if string(before) != string(after) {
		t.Fatal("doctor changed global disable policy")
	}
	delete(m, "hooksConfig")
	m["hooks"] = []any{map[string]any{"command": "foreign"}}
	if err := writeJSONObject(inst.configPath(), m); err != nil {
		t.Fatal(err)
	}
	before, _ = os.ReadFile(inst.configPath())
	if err := updateAgentSettings(inst, inst.configPath(), "/opt/gtmux", true); err == nil {
		t.Fatal("malformed foreign hook shape accepted")
	}
	after, _ = os.ReadFile(inst.configPath())
	if string(before) != string(after) {
		t.Fatal("malformed config was overwritten")
	}
}

func TestKiroMigratedArrayForeignProfileSurvivesGlobalInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	legacy := kiroInstaller(false)
	if err := writeJSONObject(legacy.configPath(), map[string]any{"name": "gtmux", "hooks": []any{
		map[string]any{"trigger": "Stop", "action": map[string]any{"type": "command", "command": "/opt/gtmux hook --agent kiro Stop"}},
		map[string]any{"trigger": "Stop", "action": map[string]any{"type": "command", "command": "echo foreign"}},
	}}); err != nil {
		t.Fatal(err)
	}
	inst, _ := selectedAgentInstaller("kiro")
	if inst.format != formatKiroV3 {
		t.Fatal("migrated array was downgraded")
	}
	if err := updateAgentSettings(inst, inst.configPath(), "/opt/gtmux", true); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(legacy.configPath())
	if strings.Contains(string(raw), "gtmux hook") || !strings.Contains(string(raw), "echo foreign") {
		t.Fatal(string(raw))
	}
}
