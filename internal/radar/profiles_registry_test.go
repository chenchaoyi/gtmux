package radar

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// builtinProfiles is now derived from the agent registry. This pins that every
// registry Profile becomes a radar agentProfile (Label→Name), so adding a
// detected agent to internal/agents surfaces it in the radar with no edit here.

func TestBuiltinProfilesDerivedFromRegistry(t *testing.T) {
	ps := agents.Profiles()
	if len(builtinProfiles) != len(ps) {
		t.Fatalf("builtinProfiles=%d, registry Profiles=%d", len(builtinProfiles), len(ps))
	}
	for i, p := range ps {
		got := builtinProfiles[i]
		if got.Name != p.Label || got.IdleGlyph != p.IdleGlyph || got.Icon != p.Icon {
			t.Errorf("profile %d: got {%q,%q,%q}, want {%q,%q,%q}", i, got.Name, got.IdleGlyph, got.Icon, p.Label, p.IdleGlyph, p.Icon)
		}
		if len(got.Commands) != len(p.Commands) {
			t.Errorf("profile %d commands: got %v, want %v", i, got.Commands, p.Commands)
			continue
		}
		for j := range p.Commands {
			if got.Commands[j] != p.Commands[j] {
				t.Errorf("profile %d command %d: got %q, want %q", i, j, got.Commands[j], p.Commands[j])
			}
		}
	}
}

func TestOpencodeAndCursorProfilesPresent(t *testing.T) {
	byName := map[string][]string{}
	for _, p := range builtinProfiles {
		byName[p.Name] = p.Commands
	}
	if got := byName["opencode"]; len(got) != 1 || got[0] != "opencode" {
		t.Errorf("opencode profile commands = %v, want [opencode]", got)
	}
	if got := byName["Cursor"]; len(got) != 2 || got[0] != "cursor-agent" || got[1] != "cursor" {
		t.Errorf("Cursor profile commands = %v, want [cursor-agent cursor]", got)
	}
}
