package app

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

func withHookExecutables(t *testing.T, names ...string) {
	t.Helper()
	old := hookExecutableLookup
	allowed := map[string]bool{}
	for _, name := range names {
		allowed[name] = true
	}
	hookExecutableLookup = func(name string) string {
		if allowed[name] {
			return "/fixture/bin/" + name
		}
		return ""
	}
	t.Cleanup(func() { hookExecutableLookup = old })
}

func hookPresenceHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex"))
	t.Setenv("KIMI_CODE_HOME", filepath.Join(home, "kimi"))
	for key, inst := range agentInstallers {
		if inst.envDir != "" {
			t.Setenv(inst.envDir, filepath.Join(home, key))
		}
	}
	return home
}

func homeFiles(t *testing.T, home string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(home, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			out[rel] = "directory"
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSavedAgentHomesDoNotSelectHooksOrDoctorRepairs(t *testing.T) {
	home := hookPresenceHome(t)
	withHookExecutables(t)
	// Both complete gtmux registrations and malformed retained settings can survive
	// an uninstall. Neither is executable-installation evidence.
	for _, key := range []string{"gemini", "kiro"} {
		inst := agentInstallers[key]
		if err := updateAgentSettings(inst, inst.configPath(), "/fixture/gtmux", true); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{claudeSettingsPath(), kimiConfigPath()}
	for _, inst := range agentInstallers {
		if inst.key != "gemini" && inst.key != "kiro" {
			paths = append(paths, inst.configPath())
		}
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("retained user configuration\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := homeFiles(t, home)
	if got := detectedHookAgents(); len(got) != 0 {
		t.Fatal("saved homes selected automatic installation", got)
	}
	if got := doctorAgentHookChecks(); len(got) != 0 {
		t.Fatal("saved homes earned installed-agent rows", got)
	}
	rc := installDetectedHooks(detectedHookAgents(), []string{"--agent", "auto", "--yes"}, func(string) bool { t.Fatal("unused policy read"); return false }, func([]string) int { t.Fatal("unused installer ran"); return 1 })
	if rc != 0 {
		t.Fatal(rc)
	}
	s := &fixState{yes: true, bin: "/fixture/gtmux"}
	applied := s.stepClaudeHook() + s.stepClaudeTUI() + s.stepCodexHook() + s.stepKimiHook()
	for _, key := range additionalHookAgents {
		applied += s.stepAgentHook(key)
	}
	if applied != 0 || s.rc != 0 {
		t.Fatalf("unused agent repaired: applied=%d rc=%d", applied, s.rc)
	}
	if after := homeFiles(t, home); !reflect.DeepEqual(before, after) {
		t.Fatal("automatic setup/doctor changed retained settings or created files")
	}
}

func TestHookPresenceUsesRegistryExecutableProbes(t *testing.T) {
	hookPresenceHome(t)
	withHookExecutables(t, "gemini", "kiro-cli", "copilot")
	if got, want := detectedHookAgents(), []string{"copilot", "gemini", "kiro"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selection %v want %v", got, want)
	}
	var labels []string
	for _, c := range doctorAgentHookChecks() {
		labels = append(labels, c.label)
	}
	sort.Strings(labels)
	if want := []string{"Copilot hook", "Gemini hook", "Kiro hook"}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("doctor rows %v want %v", labels, want)
	}
	for _, key := range []string{"kiro", "copilot"} {
		m, _ := agents.For(key)
		if len(m.Detect) != 0 || len(m.InstallDetect) == 0 {
			t.Fatal("install-only probe altered radar support", key)
		}
	}
}

func TestDetectedCLIWithoutHomeCanConfigureHooks(t *testing.T) {
	hookPresenceHome(t)
	t.Setenv("GTMUX_LANG", "en")
	withHookExecutables(t, "gemini")
	path := agentInstallers["gemini"].configPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("fixture already configured")
	}
	row := rowAgentHook("gemini")
	if row.status != stRec || row.value != "not configured" {
		t.Fatal("missing settings presented as a broken file", row)
	}
	s := &fixState{yes: true, bin: "/fixture/gtmux"}
	if got := s.stepAgentHook("gemini"); got != 1 || s.rc != 0 {
		t.Fatalf("detected CLI not repaired: %d rc=%d", got, s.rc)
	}
	if row := rowAgentHook("gemini"); row.status != stOK || row.value != "configured" {
		t.Fatal("hook state confused with CLI installation", row)
	}
}

func TestExplicitHookSelectionStillAllowsPreconfiguration(t *testing.T) {
	hookPresenceHome(t)
	withHookExecutables(t)
	if rc := installHooks([]string{"--agent", "gemini", "--yes"}); rc != 0 {
		t.Fatal(rc)
	}
	raw, err := os.ReadFile(agentInstallers["gemini"].configPath())
	if err != nil || !strings.Contains(string(raw), " hook --agent gemini Stop") {
		t.Fatal("explicit setup lost", err)
	}
	if len(detectedHookAgents()) != 0 || len(doctorAgentHookChecks()) != 0 {
		t.Fatal("preconfiguration became installation evidence")
	}
}
