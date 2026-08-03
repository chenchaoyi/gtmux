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
		{Label: "Codex", Commands: []string{"codex"}, Icon: "/Applications/Codex.app"},
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
	// internal/driver/driver.go Content + Headless are wired for claude+codex only
	want := []string{"claude", "codex"}
	if got := sortedSet(ContentKeys()); !reflect.DeepEqual(got, want) {
		t.Fatalf("ContentKeys()=%v, want %v", got, want)
	}
	if got := sortedSet(HeadlessKeys()); !reflect.DeepEqual(got, want) {
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
