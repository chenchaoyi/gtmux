package app

import (
	"os"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// opencode is the first PLUGIN-model agent: its only extension point is a JS
// plugin, not a command-hook JSON file. These tests pin that install writes the
// plugin and uninstall removes it cleanly (a dedicated file), plus the
// conformance invariant that keeps the next agent from shipping dark.

func TestOpencodeInstallerWritesAndRemovesPlugin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	inst, ok := agentInstallers["opencode"]
	if !ok || inst.plugin == nil {
		t.Fatal("opencode must be a plugin installer")
	}

	if rc := installAgentHooks(inst, true); rc != 0 {
		t.Fatalf("install rc=%d", rc)
	}
	path := inst.configPath()
	if !strings.HasSuffix(path, "/.config/opencode/plugin/gtmux.js") {
		t.Fatalf("plugin path = %q, want …/.config/opencode/plugin/gtmux.js", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("plugin not written: %v", err)
	}
	js := string(data)
	for _, want := range []string{
		"export const",
		"event: async ({ event })",         // opencode's real single event hook (not per-name keys)
		"\"chat.message\": async",          // the user-submit hook (send receipt)
		"hook --agent opencode ${event}",   // generic event fire
		"UserPromptSubmit",                 // receipt path
		"JSON.stringify({ prompt: text })", // prompt piped to stdin so eventSummary records the needle
		"\"session.idle\": \"Stop\"",       // turn-done → Stop (verified against opencode 1.18.11)
		"\"permission.asked\": \"PermissionRequest\"",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("generated plugin missing %q", want)
		}
	}
	// Guard against the wrong-API regression: opencode does NOT use per-event-name
	// hook keys, so a bare "session.idle": async () => key would silently never fire.
	if strings.Contains(js, "\"session.idle\": async") {
		t.Error("plugin uses per-event-name hook keys — opencode's API is a single `event` hook")
	}

	if rc := uninstallAgentHooks(inst); rc != 0 {
		t.Fatalf("uninstall rc=%d", rc)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("plugin still present after uninstall: %v", err)
	}
}

// TestEveryHookEquippedAgentHasInstaller is the conformance guard: an agent the
// registry marks hook-equipped MUST have an install path, or its whole event
// layer stays dark (exactly what opencode did for months). Claude uses the
// dedicated install-hooks path; every other hook-equipped agent needs an
// agentInstallers entry (resolved to its canonical key).
func TestEveryHookEquippedAgentHasInstaller(t *testing.T) {
	for _, key := range agents.HookEquippedKeys() {
		m, _ := agents.For(key)
		canonical := m.Key
		if canonical == "claude" {
			continue // dedicated cmdInstallHooks path
		}
		if _, ok := agentInstallers[canonical]; !ok {
			t.Errorf("agent %q (canonical %q) is hook-equipped but has no installer — its event layer would stay dark", key, canonical)
		}
	}
}
