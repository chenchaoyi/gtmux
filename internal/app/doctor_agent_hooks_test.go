package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func executableHookFixture(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gtmux")
	body := "#!/bin/sh\nif [ -n \"$GTMUX_TEST_HOOK_MARKER\" ]; then : > \"$GTMUX_TEST_HOOK_MARKER\"; fi\n"
	if err := os.WriteFile(bin, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestDoctorRejectsWrongDisabledAndScopedBindings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bin := executableHookFixture(t)
	for _, key := range []string{"codex", "cursor", "gemini", "copilot", "kiro"} {
		t.Run(key, func(t *testing.T) {
			inst := agentInstallers[key]
			if err := updateAgentSettings(inst, inst.configPath(), bin, true); err != nil {
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
				asObject(asArray(entry["hooks"])[0])["command"] = bin + " hook --agent " + key + " StopFailure"
			case formatCopilot:
				entry["bash"] = bin + " hook --agent copilot StopFailure"
			default:
				entry["command"] = bin + " hook --agent " + key + " StopFailure"
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
			if err := updateAgentSettings(inst, inst.configPath(), bin, true); err != nil {
				t.Fatal(err)
			}
			if len(missingAgentHookEvents(key)) != 0 {
				t.Fatal("repair remained incomplete")
			}
		})
	}
}

func TestAppHookFixturesIgnoreInheritedCodexHome(t *testing.T) {
	caller := t.TempDir()
	path := filepath.Join(caller, "hooks.json")
	sentinel := []byte("{\"hooks\":{},\"userSetting\":true}\n")
	if err := os.WriteFile(path, sentinel, 0600); err != nil {
		t.Fatal(err)
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(testBinary, "-test.run=^TestDoctorRejectsWrongDisabledAndScopedBindings$/^codex$", "-test.v")
	command.Env = append(os.Environ(), "HOME="+t.TempDir(), "CODEX_HOME="+caller)
	if output, err := command.CombinedOutput(); err != nil || !strings.Contains(string(output), "--- PASS: TestDoctorRejectsWrongDisabledAndScopedBindings/codex") {
		t.Fatalf("child fixture did not pass: %v\n%s", err, output)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(sentinel) {
		t.Fatalf("inherited CODEX_HOME was modified: %v, %q", err, got)
	}
	entries, err := os.ReadDir(caller)
	if err != nil || len(entries) != 1 {
		t.Fatalf("inherited CODEX_HOME received extra files: %v, %v", err, entries)
	}
}

func TestDoctorRepairsUnusableCodexHookExecutable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	marker := filepath.Join(t.TempDir(), "hook-ran")
	t.Setenv("GTMUX_TEST_HOOK_MARKER", marker)
	withHookExecutables(t, "codex")
	bin := executableHookFixture(t)
	t.Setenv("PATH", filepath.Dir(bin))
	inst := agentInstallers["codex"]
	foreign := map[string]any{"type": "command", "command": "foreign stop notifier"}
	for _, kind := range []string{"missing", "non-executable", "directory"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("CODEX_HOME", t.TempDir())
			path := inst.configPath()
			ensureCodexFeaturesHooks(codexConfigPath())
			unusable := filepath.Join(t.TempDir(), "gtmux")
			switch kind {
			case "non-executable":
				if err := os.WriteFile(unusable, []byte("#!/bin/sh\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(unusable, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := updateAgentSettings(inst, path, bin, true); err != nil {
				t.Fatal(err)
			}
			m, err := loadJSONObject(path)
			if err != nil {
				t.Fatal(err)
			}
			hooks := asObject(m["hooks"])
			stop := asObject(asArray(hooks["Stop"])[0])
			stop["hooks"] = []any{map[string]any{
				"type": "command", "command": unusable + " hook --agent codex Stop",
			}, foreign}
			prompt := asObject(asArray(hooks["UserPromptSubmit"])[0])
			asObject(asArray(prompt["hooks"])[0])["command"] = "gtmux hook --agent codex UserPromptSubmit"
			if err := writeJSONObject(path, m); err != nil {
				t.Fatal(err)
			}
			if missing := missingAgentHookEvents("codex"); !reflect.DeepEqual(missing, []string{"Stop"}) {
				t.Fatalf("missing events = %v, want only Stop", missing)
			}
			if row := rowCodexHook(); row.status != stRec || !strings.Contains(row.note, "Stop") {
				t.Fatalf("doctor did not identify the broken Stop hook: %+v", row)
			}
			fix := &fixState{yes: true, bin: bin}
			if applied := fix.stepCodexHook(); applied != 1 || fix.rc != 0 {
				t.Fatalf("doctor repair returned %d, exit status %d", applied, fix.rc)
			}
			if row := rowCodexHook(); row.status != stOK {
				t.Fatalf("repaired hook did not validate: %+v", row)
			}
			m, err = loadJSONObject(path)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, group := range asArray(asObject(m["hooks"])["Stop"]) {
				for _, handler := range asArray(asObject(group)["hooks"]) {
					found = found || reflect.DeepEqual(handler, foreign)
				}
			}
			if !found {
				t.Fatal("foreign Stop hook was removed")
			}
		})
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("doctor executed a hook or could not verify the marker: %v", err)
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
	withHookExecutables(t, "gemini")
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
