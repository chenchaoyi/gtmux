package driver

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// The driver registry is now DERIVED from the agent registry. These tests pin that
// wiring: every hook-equipped agent gets Receipt+Ready, every content agent gets a
// Content parser, and an agent the registry does not hook-equip gets the zero
// Driver (Layer-1 everywhere). Adding an agent to internal/agents flows here with
// no edit to this package.

func TestEveryHookEquippedAgentHasReceiptAndReady(t *testing.T) {
	for _, k := range agents.HookEquippedKeys() {
		d := For(k)
		if d.Receipt == nil {
			t.Errorf("agent %q is hook-equipped in the registry but has no Receipt", k)
		}
		if d.Ready == nil {
			t.Errorf("agent %q is hook-equipped in the registry but has no Ready", k)
		}
	}
}

func TestContentWiredForRegistryContentKeys(t *testing.T) {
	for _, k := range agents.ContentKeys() {
		if For(k).Content == nil {
			t.Errorf("agent %q declares a transcript parser in the registry but Content is nil", k)
		}
	}
}

func TestNonHookedAgentGetsZeroDriver(t *testing.T) {
	// aider is radar-detected but NOT hook-equipped: it must fall to Layer 1.
	if d := For("aider"); d.Receipt != nil || d.Ready != nil {
		t.Fatalf("aider is not hook-equipped; want zero Driver, got Receipt=%v Ready=%v", d.Receipt != nil, d.Ready != nil)
	}
}
