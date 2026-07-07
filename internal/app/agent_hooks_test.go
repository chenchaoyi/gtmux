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
	// Lifecycle timeout = 10000; feed (BeforeTool) = 120000. Gemini's tool hooks are
	// BeforeTool/AfterTool (NOT Pre/Post) — the gtmux TOKEN stays PreToolUse.
	if to := nestedTimeout(t, m, "AfterAgent"); to != 10000 {
		t.Errorf("AfterAgent timeout = %d, want 10000", to)
	}
	if got := firstCommand(t, m, "BeforeTool"); got != testBin+" hook --agent gemini PreToolUse" {
		t.Errorf("BeforeTool cmd = %q", got)
	}
	if to := nestedTimeout(t, m, "BeforeTool"); to != feedTimeoutMs {
		t.Errorf("BeforeTool (feed) timeout = %d, want %d", to, feedTimeoutMs)
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

// TestInstallCopilotDedicated: Copilot's ~/.copilot/hooks/gtmux.json shape —
// {"version":1,"hooks":{"<Event>":[{"type":"command","bash":…,"timeoutSec":N}]}},
// PascalCase keys, and the wholly-gtmux file removed on uninstall.
func TestInstallCopilotDedicated(t *testing.T) {
	inst := agentInstallers["copilot"]
	path := filepath.Join(t.TempDir(), "gtmux.json")
	for i := 0; i < 2; i++ { // idempotent
		if err := updateAgentSettings(inst, path, testBin, true); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	m := readSettings(t, path)
	if m["version"] != float64(1) {
		t.Errorf("copilot file must set version:1, got %v", m["version"])
	}
	hooks, _ := m["hooks"].(map[string]any)
	list := asArray(hooks["PermissionRequest"])
	if len(list) != 1 {
		t.Fatalf("PermissionRequest should hold one gtmux entry, got %d", len(list))
	}
	e := list[0].(map[string]any)
	if e["type"] != "command" {
		t.Errorf("copilot entry type = %v, want command", e["type"])
	}
	if got := asString(e["bash"]); got != testBin+" hook --agent copilot PermissionRequest" {
		t.Errorf("copilot bash cmd = %q", got)
	}
	if e["timeoutSec"] != float64(5) { // 5000ms / 1000
		t.Errorf("copilot timeoutSec = %v, want 5", e["timeoutSec"])
	}

	if err := updateAgentSettings(inst, path, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a dedicated copilot file should be removed entirely on uninstall")
	}
}

// TestInstallCodexHooks: Codex's hooks.json shape — events under a top-level "hooks"
// object (root-level event keys are REJECTED by Codex's HooksFile), each handler
// {type:"command",command,timeout(SECONDS),async:true}. Verifies the mapping,
// idempotency, and that a foreign hook + the file's `description` survive.
func TestInstallCodexHooks(t *testing.T) {
	inst := agentInstallers["codex"]
	path := filepath.Join(t.TempDir(), "hooks.json")
	// Seed with a description + a foreign PreToolUse hook (both must survive).
	seed := `{"description":"my gate","hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"/bin/echo hi"}]}]}}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // idempotent
		if err := updateAgentSettings(inst, path, testBin, true); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	m := readSettings(t, path)
	// Events MUST live under a top-level "hooks" object — nothing at the root.
	if _, ok := m["hooks"].(map[string]any); !ok {
		t.Fatal("codex events must sit under a top-level \"hooks\" object")
	}
	if _, rooted := m["UserPromptSubmit"]; rooted {
		t.Error("codex must NOT put event keys at the JSON root (Codex rejects that)")
	}
	if m["description"] != "my gate" {
		t.Errorf("codex file's description must survive, got %v", m["description"])
	}
	if got := firstCommand(t, m, "PermissionRequest"); got != testBin+" hook --agent codex PermissionRequest" {
		t.Errorf("PermissionRequest cmd = %q", got)
	}
	// timeout is in SECONDS (10000ms / 1000 = 10), and async:true keeps it off the
	// turn's critical path.
	if to := nestedTimeout(t, m, "Stop"); to != 10 {
		t.Errorf("codex Stop timeout = %d, want 10 (SECONDS)", to)
	}
	hooks, _ := m["hooks"].(map[string]any)
	e := asArray(hooks["Stop"])[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	if e["async"] != true {
		t.Errorf("codex handler must set async:true, got %v", e["async"])
	}
	if n := countAgentHooks(t, m, "PreToolUse"); n != 1 {
		t.Errorf("foreign PreToolUse hook must survive, got %d", n)
	}
	if n := countAgentHooks(t, m, "Stop"); n != 1 {
		t.Errorf("reinstall left %d Stop entries, want 1 (idempotent)", n)
	}

	// Uninstall: gtmux entries gone, foreign hook + description survive.
	if err := updateAgentSettings(inst, path, "", false); err != nil {
		t.Fatal(err)
	}
	m = readSettings(t, path)
	if got := firstCommand(t, m, "PreToolUse"); !strings.Contains(got, "echo hi") {
		t.Errorf("foreign hook should survive uninstall, got %q", got)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("codex file has a foreign hook — it must NOT be removed on uninstall")
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
