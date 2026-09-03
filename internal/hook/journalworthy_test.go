package hook

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func waitingExists(pane string) bool { return state.Exists(state.WaitingPath(pane)) }

func eventLifecycle(agent, raw string) string { return classify(agent, raw, "").Lifecycle }

// A Claude turn runs about ten tools (measured: 4711 calls over 459 turns), so the
// unscoped PostToolUse registration that lets an approved permission clear its own mark
// would add roughly 3500 records a day against a total of 970. That stream is what HQ's
// consumption watermark reads, so a tool finishing must not enter it. A WAIT ending must.
func TestJournalWorthy(t *testing.T) {
	if journalWorthy("Resumed", false) {
		t.Error("a tool finishing with no wait to clear is not news")
	}
	if !journalWorthy("Resumed", true) {
		t.Error("a wait ending IS news — it is the record that says the block is over")
	}
	// Everything else is unconditional. A rule that quietly widened would drop real
	// lifecycle records, which is a far worse failure than a noisy stream.
	for _, e := range []string{"Stop", "StopFailure", "UserPromptSubmit", "Waiting", "Notification", "SessionStart", "SessionEnd", "PreCompact"} {
		for _, had := range []bool{false, true} {
			if !journalWorthy(e, had) {
				t.Errorf("%s (hadWaiting=%v) must always be journalled", e, had)
			}
		}
	}
}

// The chain this whole change exists for: a permission is asked, the mark goes down, you
// approve, the tool runs and finishes, and THAT is what clears the mark. Before the
// PostToolUse registration was widened the last step never arrived for a Read or a Bash,
// so a pane answered at 09:48 read as needs-you until a 15-minute staleness guard
// expired — on a pane whose main thread was parked behind a background agent and would
// produce no Stop either.
func TestAnApprovedToolClearsTheWait(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const pane = "%7"

	// Mid-turn permission request.
	applyState(decide("UserPromptSubmit", false, false), pane)
	applyState(decide("Waiting", true, false), pane)
	if !waitingExists(pane) {
		t.Fatal("the request should have marked the pane as waiting")
	}

	// You approve; the tool runs and finishes. PostToolUse classifies as Resumed.
	if got := eventLifecycle("claude", "PostToolUse"); got != "Resumed" {
		t.Fatalf("PostToolUse classifies as %q, want Resumed — the clear rides on that", got)
	}
	applyState(decide("Resumed", true, false), pane)
	if waitingExists(pane) {
		t.Error("the wait survived the tool it was blocking on")
	}
}
