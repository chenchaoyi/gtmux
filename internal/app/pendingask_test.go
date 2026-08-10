package app

import (
	"os"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// hasPendingAsk is what serve wires into the approval card's gate. The question it must
// answer is "did the AGENT ask?" — NOT "does a waiting marker exist", which is the
// contract it used to have and which is a different question, because gtmux's own slow
// tick writes that marker too. Both directions are pinned here, over REAL marker files.
func TestHasPendingAsk(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.WaitingDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(pane, kind string) {
		if err := state.WriteMarker(state.WaitingPath(pane), kind); err != nil {
			t.Fatal(err)
		}
	}

	// No marker at all → nothing is pending.
	if hasPendingAsk("%none") {
		t.Error("a pane with no waiting marker must not read as an ask")
	}

	// The agent asked — every kind the hook writes.
	for _, k := range []string{"permission", "plan", "question"} {
		write("%"+k, k)
		if !hasPendingAsk("%" + k) {
			t.Errorf("kind %q is the agent asking; the approval card must be offered", k)
		}
	}

	// gtmux inferred it from the screen. THIS is the 2026-08-10 false card: the marker
	// exists, so the old gate opened, and an agent's numbered prose was served as
	// choices — from a pane whose agent had asked nothing.
	for _, k := range []string{"startup", "draft"} {
		write("%"+k, k)
		if hasPendingAsk("%" + k) {
			t.Errorf("kind %q is gtmux's own screen verdict — it must not open the options gate", k)
		}
	}

	// A marker with no kind is not evidence of a question.
	write("%empty", "")
	if hasPendingAsk("%empty") {
		t.Error("an empty kind is the absence of evidence, not evidence of an ask")
	}
}
