package hook

import (
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
	"strings"
	"testing"
)

// CET-006: manual compaction is state-neutral, not a late automatic rearm.
func TestCompletedCodexTurnPreservesManualCompactReceipt(t *testing.T) {
	run := completedTurnHookFixture(t)
	run("UserPromptSubmit", "thread", "one")
	run("Stop", "thread", "one")
	before, ok := state.ReadCompletion("%71")
	if !ok {
		t.Fatal("missing completed-turn proof")
	}
	records, _ := events.ReadSince(0)
	Run(strings.NewReader(`{"session_id":"thread","turn_id":"one","trigger":"manual"}`), []string{"--agent", "codex", "--detached", "PostCompact"})
	after, ok := state.ReadCompletion("%71")
	if !ok || before != after || state.Exists(state.ActivePath("%71")) || state.ReadMarker(state.TurnIDPath("%71")) != "one" {
		t.Fatal("manual compaction changed settled turn state")
	}
	afterRecords, _ := events.ReadSince(0)
	if len(afterRecords) != len(records)+1 || afterRecords[len(afterRecords)-1].Event != "PostCompact" || afterRecords[len(afterRecords)-1].AgentSession != "thread" {
		t.Fatal("manual compaction lost its actual lifecycle receipt")
	}
}
