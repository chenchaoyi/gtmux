package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAgyHooksAndForeignPreservation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AGY_CONFIG_DIR", "")

	inst := agentInstallers["agy"]
	path := inst.configPath()
	wantPath := filepath.Join(home, ".gemini", "config", "hooks.json")
	if path != wantPath {
		t.Fatalf("configPath() = %q, want %q", path, wantPath)
	}

	// Pre-populate a foreign named hook that must survive install, reinstall, and uninstall.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := `{"lint-checker":{"PostToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"./scripts/lint.sh"}]}]}}`
	if err := os.WriteFile(path, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := updateAgentSettings(inst, path, testBin, true); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}

	m := readSettings(t, path)
	if _, ok := m["lint-checker"]; !ok {
		t.Fatal("foreign named hook was removed on install")
	}
	gtmuxEntry := asObject(m[agyHookName])
	if len(gtmuxEntry) == 0 {
		t.Fatal("missing named gtmux hook entry")
	}
	if _, hasPreTool := gtmuxEntry["PreToolUse"]; hasPreTool {
		t.Fatal("observational bridge must not register agy PreToolUse decision hook")
	}
	for _, ev := range []string{"SessionStart", "PreInvocation", "Stop"} {
		list := asArray(gtmuxEntry[ev])
		if len(list) != 1 {
			t.Fatalf("%s entries = %d, want 1", ev, len(list))
		}
		h := asObject(list[0])
		if asString(h["type"]) != "command" || asString(h["command"]) != testBin+" hook --agent agy "+ev {
			t.Errorf("%s handler = %v", ev, h)
		}
		if h["timeout"] != float64(agyHookTimeoutSec) {
			t.Errorf("%s timeout = %v, want %d seconds", ev, h["timeout"], agyHookTimeoutSec)
		}
	}
	postGroups := asArray(gtmuxEntry["PostToolUse"])
	if len(postGroups) != 1 {
		t.Fatalf("PostToolUse groups = %d, want 1", len(postGroups))
	}
	group := asObject(postGroups[0])
	if asString(group["matcher"]) != "*" {
		t.Errorf("PostToolUse matcher = %q, want *", group["matcher"])
	}
	postHandlers := asArray(group["hooks"])
	if len(postHandlers) != 1 || asString(asObject(postHandlers[0])["command"]) != testBin+" hook --agent agy PostToolUse" {
		t.Errorf("PostToolUse handlers = %v", postHandlers)
	}

	if err := updateAgentSettings(inst, path, "", false); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	m = readSettings(t, path)
	if _, exists := m[agyHookName]; exists {
		t.Fatal("gtmux entry survived uninstall")
	}
	if _, exists := m["lint-checker"]; !exists {
		t.Fatal("foreign named hook was lost on uninstall")
	}

	// Uninstall when gtmux is the only entry removes the file cleanly.
	standalone := filepath.Join(t.TempDir(), "hooks.json")
	if err := updateAgentSettings(inst, standalone, testBin, true); err != nil {
		t.Fatal(err)
	}
	if err := updateAgentSettings(inst, standalone, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(standalone); !os.IsNotExist(err) {
		t.Fatalf("standalone hooks.json should be removed on uninstall, err=%v", err)
	}
}

func TestDoctorAgyHookDetectionRepairAndPolicy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AGY_CONFIG_DIR", "")
	bin := executableHookFixture(t)
	inst := agentInstallers["agy"]

	if err := updateAgentSettings(inst, inst.configPath(), bin, true); err != nil {
		t.Fatal(err)
	}
	if missing := missingAgentHookEvents("agy"); len(missing) != 0 {
		t.Fatalf("complete agy hooks reported missing: %v", missing)
	}
	if row := rowAgentHook("agy"); row.status != stOK {
		t.Fatalf("healthy agy hook row = %+v, want stOK", row)
	}

	// Scoped PostToolUse matcher must be flagged as incomplete.
	m := readSettings(t, inst.configPath())
	entry := asObject(m[agyHookName])
	asObject(asArray(entry["PostToolUse"])[0])["matcher"] = "run_command"
	if err := writeJSONObject(inst.configPath(), m); err != nil {
		t.Fatal(err)
	}
	if missing := missingAgentHookEvents("agy"); !strings.Contains(strings.Join(missing, ","), "PostToolUse") {
		t.Fatalf("scoped PostToolUse was not flagged missing: %v", missing)
	}

	// Explicit enabled:false is a user policy choice preserved by doctor.
	entry["enabled"] = false
	if err := writeJSONObject(inst.configPath(), m); err != nil {
		t.Fatal(err)
	}
	if !hookPolicyDisabled("agy", inst.configPath()) {
		t.Fatal("enabled:false was not recognized as policy disabled")
	}
	if row := rowAgentHook("agy"); row.status != stRec {
		t.Fatalf("disabled agy hook row = %+v, want stRec", row)
	}

	// Reinstall repairs scoped/disabled bindings cleanly.
	if err := updateAgentSettings(inst, inst.configPath(), bin, true); err != nil {
		t.Fatal(err)
	}
	if missing := missingAgentHookEvents("agy"); len(missing) != 0 {
		t.Fatalf("repaired agy hooks still missing: %v", missing)
	}
}
