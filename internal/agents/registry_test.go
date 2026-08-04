package agents

import (
	"reflect"
	"sort"
	"testing"
)

// These golden snapshots are the legacy hand-kept maps, copied verbatim from the
// subsystems the registry replaces. They pin the registry's projections so a
// subsystem can switch to an accessor with ZERO behavior change. If a snapshot
// and its accessor disagree, either the registry drifted or a real migration is
// intended — update both deliberately.

func sortedSet(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}

func TestHookEquippedKeys_matchesLegacy(t *testing.T) {
	// internal/driver/driver.go hookEquippedAgents
	want := []string{"claude", "codex", "gemini", "cursor", "cursor-agent", "opencode", "copilot", "kiro"}
	if got := HookEquippedKeys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("HookEquippedKeys()=%v, want %v", got, want)
	}
}

func TestProfiles_matchesLegacy(t *testing.T) {
	// internal/radar/agents.go builtinProfiles
	want := []Profile{
		{Label: "Claude Code", Commands: []string{"claude"}, IdleGlyph: "✳", Icon: "/Applications/Claude.app"},
		{Label: "Codex", Commands: []string{"codex"}, Icon: "/Applications/ChatGPT.app"},
		{Label: "Gemini", Commands: []string{"gemini"}},
		{Label: "Aider", Commands: []string{"aider"}},
		{Label: "opencode", Commands: []string{"opencode"}},
		{Label: "Crush", Commands: []string{"crush"}},
		{Label: "Cursor", Commands: []string{"cursor-agent", "cursor"}, Icon: "/Applications/Cursor.app"},
		{Label: "Amp", Commands: []string{"amp"}},
	}
	if got := Profiles(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Profiles()=%#v\nwant %#v", got, want)
	}
}

func TestDisplayNames_matchesLegacy(t *testing.T) {
	// internal/hook/hook.go agentDisplay
	want := map[string]string{
		"claude": "Claude Code", "codex": "Codex", "gemini": "Gemini", "cursor": "Cursor",
		"opencode": "opencode", "copilot": "Copilot", "hermes-agent": "Hermes", "kiro": "Kiro",
	}
	if got := DisplayNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DisplayNames()=%v, want %v", got, want)
	}
}

func TestResumeArgv_matchesLegacy(t *testing.T) {
	// internal/resume/command.go resumeArgv
	want := map[string][]string{
		"claude":       {"claude", "--resume"},
		"codex":        {"codex", "resume"},
		"cursor":       {"cursor-agent", "--resume"},
		"gemini":       {"gemini", "--resume"},
		"kiro":         {"kiro-cli", "chat", "--resume-id"},
		"copilot":      {"copilot", "--resume"},
		"opencode":     {"opencode", "--session"},
		"hermes-agent": {"hermes", "--resume"},
		"grok":         {"grok", "-r"},
	}
	if got := ResumeArgv(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ResumeArgv()=%v, want %v", got, want)
	}
}

func TestResourceNames_matchesLegacy(t *testing.T) {
	// internal/resource/attribute.go agent list
	want := []string{"claude", "codex", "cursor", "gemini", "aider", "opencode", "crush", "amp"}
	if got := ResourceNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ResourceNames()=%v, want %v", got, want)
	}
}

func TestContentAndHeadlessKeys_matchesLegacy(t *testing.T) {
	// Content parsers: claude + codex read the agent's own on-disk log; opencode has
	// no readable log, so gtmux keeps its OWN transcript (internal/transcript/opencode.go,
	// fed by the plugin via `gtmux hook`). Headless one-shot stays claude+codex only.
	if got, want := sortedSet(ContentKeys()), []string{"claude", "codex", "opencode"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ContentKeys()=%v, want %v", got, want)
	}
	if got, want := sortedSet(HeadlessKeys()), []string{"claude", "codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("HeadlessKeys()=%v, want %v", got, want)
	}
}

func TestDedicatedSemanticsKeys_matchesLegacy(t *testing.T) {
	// internal/hook/classify.go agentEventSemantics
	want := []string{"claude", "codex", "hermes-agent", "kiro"}
	if got := sortedSet(DedicatedSemanticsKeys()); !reflect.DeepEqual(got, want) {
		t.Fatalf("DedicatedSemanticsKeys()=%v, want %v", got, want)
	}
}

func TestForResolvesAliases(t *testing.T) {
	if a, ok := For("cursor-agent"); !ok || a.Key != "cursor" {
		t.Fatalf("For(cursor-agent)=%+v ok=%v, want cursor", a, ok)
	}
	if _, ok := For("nope"); ok {
		t.Fatalf("For(nope) should be not-found")
	}
}

func TestKeyForLabel(t *testing.T) {
	cases := map[string]string{"opencode": "opencode", "Claude Code": "claude", "Gemini": "gemini", "Grok": "grok", "nope": ""}
	for label, want := range cases {
		if got := KeyForLabel(label); got != want {
			t.Errorf("KeyForLabel(%q)=%q, want %q", label, got, want)
		}
	}
}

// CommandKeys is what identifies an agent from a command line no process backs any
// more — the `pane_full_command` a tmux-resurrect save recorded before a reboot.
// Every resumable agent must be findable by the binary its own resume argv launches,
// or restore cannot tell "an agent was running here" from "a shell was".
func TestCommandKeys(t *testing.T) {
	got := CommandKeys()
	for _, c := range []struct{ cmd, key string }{
		{"claude", "claude"},
		{"codex", "codex"},
		{"cursor-agent", "cursor"}, // the binary, mapped to the canonical key
		{"cursor", "cursor"},
		{"opencode", "opencode"},
		{"kiro-cli", "kiro"},
		{"aider", "aider"}, // detect-only agents are identifiable too
	} {
		if got[c.cmd] != c.key {
			t.Errorf("CommandKeys()[%q]=%q, want %q", c.cmd, got[c.cmd], c.key)
		}
	}
	for _, cmd := range []string{"bash", "zsh", "vim", "node", ""} {
		if k, ok := got[cmd]; ok {
			t.Errorf("CommandKeys() must not claim %q (got %q)", cmd, k)
		}
	}
	for _, a := range manifests {
		if len(a.Resume) == 0 {
			continue
		}
		if got[a.Resume[0]] != a.Key {
			t.Errorf("%s: its resume binary %q must map back to it", a.Key, a.Resume[0])
		}
	}
}
