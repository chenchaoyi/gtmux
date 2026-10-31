package hook

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestAcceptedStopRecordsCompletionAndNewTurnClearsIt(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("TMUX_PANE", "%3")
	run := func(event, turn string) {
		t.Helper()
		Run(strings.NewReader(`{"session_id":"session","turn_id":"`+turn+`","prompt":"work"}`), []string{"--agent", "kimi", event})
	}
	run("UserPromptSubmit", "one")
	run("Stop", "one")
	if c, ok := state.ReadCompletion("%3"); !ok || c.Agent != "kimi" || c.SessionID != "session" || c.TurnID != "one" {
		t.Fatalf("accepted Stop lost its exact completion: %+v %v", c, ok)
	}
	run("UserPromptSubmit", "two")
	if _, ok := state.ReadCompletion("%3"); ok {
		t.Fatal("new work retained the old completion proof")
	}
	run("Stop", "one")
	if _, ok := state.ReadCompletion("%3"); ok {
		t.Fatal("late old Stop completed the new turn")
	}
	run("Interrupt", "two")
	if _, ok := state.ReadCompletion("%3"); ok {
		t.Fatal("interruption became a successful completion")
	}
}
