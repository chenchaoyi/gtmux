package app

import (
	"os"
	"strings"
	"testing"
)

// seedHQHome creates the home + instructions once, then NEVER overwrites — the
// file is the user's to edit and the supervisor's accumulated knowledge.
func TestSeedHQHomeIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	seeded, err := seedHQHome()
	if err != nil || !seeded {
		t.Fatalf("first seed = (%v, %v), want (true, nil)", seeded, err)
	}
	b, err := os.ReadFile(hqInstructionsPath())
	if err != nil || !strings.Contains(string(b), "gtmux digest --json") {
		t.Fatalf("seeded instructions should teach the digest toolbox: %v %q", err, b)
	}

	// User edits it — a re-run must keep the edit.
	if err := os.WriteFile(hqInstructionsPath(), []byte("MY EDITS"), 0o644); err != nil {
		t.Fatal(err)
	}
	seeded, err = seedHQHome()
	if err != nil || seeded {
		t.Fatalf("second seed = (%v, %v), want (false, nil)", seeded, err)
	}
	b, _ = os.ReadFile(hqInstructionsPath())
	if string(b) != "MY EDITS" {
		t.Errorf("re-seed clobbered the user's edits: %q", b)
	}
}

func TestHQAgentCommand(t *testing.T) {
	t.Setenv("GTMUX_HQ_AGENT", "")
	if got := hqAgentCommand(); got != "claude" {
		t.Errorf("default hq agent = %q, want claude", got)
	}
	t.Setenv("GTMUX_HQ_AGENT", "codex")
	if got := hqAgentCommand(); got != "codex" {
		t.Errorf("override hq agent = %q, want codex", got)
	}
}
