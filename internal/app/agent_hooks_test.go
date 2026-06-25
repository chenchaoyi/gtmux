package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testBin = "/opt/gtmux/gtmux"

// firstCommand returns the gtmux command string for an event in a freshly written
// file, across all three formats (flat/kiro: entry.command; nested: entry.hooks[].command).
func firstCommand(t *testing.T, m map[string]any, event string) string {
	t.Helper()
	hooks, _ := m["hooks"].(map[string]any)
	for _, raw := range asArray(hooks[event]) {
		entry, _ := raw.(map[string]any)
		if c := asString(entry["command"]); c != "" { // flat / kiro
			return c
		}
		for _, h := range asArray(entry["hooks"]) { // nested
			hm, _ := h.(map[string]any)
			if c := asString(hm["command"]); c != "" {
				return c
			}
		}
	}
	return ""
}

func countAgentHooks(t *testing.T, m map[string]any, event string) int {
	t.Helper()
	hooks, _ := m["hooks"].(map[string]any)
	n := 0
	for _, raw := range asArray(hooks[event]) {
		entry, _ := raw.(map[string]any)
		if asString(entry["command"]) != "" {
			n++
		}
		for _, h := range asArray(entry["hooks"]) {
			hm, _ := h.(map[string]any)
			if asString(hm["command"]) != "" {
				n++
			}
		}
	}
	return n
}

// TestInstallCursorFlat: the .flat shape, native-event → gtmux-token mapping,
// the version:1 wrapper, idempotency, and clean uninstall.
func TestInstallCursorFlat(t *testing.T) {
	inst := agentInstallers["cursor"]
	path := filepath.Join(t.TempDir(), "hooks.json")

	for i := 0; i < 2; i++ { // install twice → must converge to one entry each
		if err := updateAgentSettings(inst, path, testBin, true); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	m := readSettings(t, path)
	if m["version"] != float64(1) {
		t.Errorf("flat format must set version:1, got %v", m["version"])
	}
	if got := firstCommand(t, m, "beforeSubmitPrompt"); got != testBin+" hook --agent cursor UserPromptSubmit" {
		t.Errorf("beforeSubmitPrompt cmd = %q", got)
	}
	if got := firstCommand(t, m, "beforeShellExecution"); got != testBin+" hook --agent cursor PermissionRequest" {
		t.Errorf("beforeShellExecution should map to PermissionRequest, got %q", got)
	}
	if n := countAgentHooks(t, m, "beforeSubmitPrompt"); n != 1 {
		t.Errorf("reinstall left %d entries, want 1 (idempotent)", n)
	}

	if err := updateAgentSettings(inst, path, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		// cursor isn't dedicated, but with no foreign hooks the file's hooks map
		// empties out; version is dropped and the now-empty object is removed.
		if m := readSettings(t, path); len(m) != 0 {
			t.Errorf("after uninstall, file should be empty/gone, got %v", m)
		}
	}
}

// TestInstallGeminiNested: the nested shape and the feed-vs-lifecycle timeouts.
func TestInstallGeminiNested(t *testing.T) {
	inst := agentInstallers["gemini"]
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := updateAgentSettings(inst, path, testBin, true); err != nil {
		t.Fatal(err)
	}
	m := readSettings(t, path)
	if _, ok := m["version"]; ok {
		t.Error("nested format must NOT add a top-level version")
	}
	if got := firstCommand(t, m, "BeforeAgent"); got != testBin+" hook --agent gemini UserPromptSubmit" {
		t.Errorf("BeforeAgent cmd = %q", got)
	}
	// Lifecycle timeout = 10000; feed (PreToolUse) = 120000.
	if to := nestedTimeout(t, m, "AfterAgent"); to != 10000 {
		t.Errorf("AfterAgent timeout = %d, want 10000", to)
	}
	if to := nestedTimeout(t, m, "PreToolUse"); to != feedTimeoutMs {
		t.Errorf("PreToolUse (feed) timeout = %d, want %d", to, feedTimeoutMs)
	}
}

func nestedTimeout(t *testing.T, m map[string]any, event string) int {
	t.Helper()
	hooks, _ := m["hooks"].(map[string]any)
	for _, raw := range asArray(hooks[event]) {
		entry, _ := raw.(map[string]any)
		for _, h := range asArray(entry["hooks"]) {
			hm, _ := h.(map[string]any)
			if to, ok := hm["timeout"].(float64); ok {
				return int(to)
			}
		}
	}
	return -1
}

// TestInstallKiroDedicated: the kiro agent-def wrapper, timeout_ms, and that the
// wholly-gtmux file is removed on uninstall.
func TestInstallKiroDedicated(t *testing.T) {
	inst := agentInstallers["kiro"]
	path := filepath.Join(t.TempDir(), "gtmux.json")
	if err := updateAgentSettings(inst, path, testBin, true); err != nil {
		t.Fatal(err)
	}
	m := readSettings(t, path)
	if m["name"] != "gtmux" {
		t.Errorf("kiro file must carry name:gtmux, got %v", m["name"])
	}
	if _, ok := m["tools"]; !ok {
		t.Error("kiro file must carry a tools grant")
	}
	if got := firstCommand(t, m, "preToolUse"); got != testBin+" hook --agent kiro preToolUse" {
		t.Errorf("preToolUse cmd = %q (feed events pass the native token)", got)
	}
	hooks, _ := m["hooks"].(map[string]any)
	entry := asArray(hooks["stop"])[0].(map[string]any)
	if entry["timeout_ms"] != float64(5000) {
		t.Errorf("kiro lifecycle timeout_ms = %v, want 5000", entry["timeout_ms"])
	}

	if err := updateAgentSettings(inst, path, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a dedicated kiro file should be removed entirely on uninstall")
	}
}

// TestInstallPreservesForeignHooks: a non-gtmux hook in the same file/event survives.
func TestInstallPreservesForeignHooks(t *testing.T) {
	inst := agentInstallers["gemini"]
	path := filepath.Join(t.TempDir(), "settings.json")
	seed := `{"theme":"dark","hooks":{"AfterAgent":[{"hooks":[{"type":"command","command":"/usr/bin/say hi"}]}]}}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateAgentSettings(inst, path, testBin, true); err != nil {
		t.Fatal(err)
	}
	m := readSettings(t, path)
	if m["theme"] != "dark" {
		t.Errorf("unrelated key 'theme' must survive, got %v", m["theme"])
	}
	if n := countAgentHooks(t, m, "AfterAgent"); n != 2 {
		t.Errorf("AfterAgent should hold the foreign + gtmux hook, got %d", n)
	}
	// Uninstall: foreign survives, gtmux gone.
	if err := updateAgentSettings(inst, path, "", false); err != nil {
		t.Fatal(err)
	}
	m = readSettings(t, path)
	if got := firstCommand(t, m, "AfterAgent"); !strings.Contains(got, "say hi") {
		t.Errorf("foreign hook should survive uninstall, got %q", got)
	}
	if n := countAgentHooks(t, m, "AfterAgent"); n != 1 {
		t.Errorf("after uninstall only the foreign hook should remain, got %d", n)
	}
}
