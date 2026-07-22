package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// The chat payload must be bounded by SIZE, not by turn count (transcript-render-bounds).
// The bug these pin: `maxTranscriptTurns = 300` was documented as "set high so nothing is
// truncated", and a real session at 147 turns — half the cap — still marshaled to 1.76 MB
// carrying 1,885 reply bubbles and 2,974 tool steps, which the phone renders eagerly. A
// count cannot imply a payload a client can hold.

// turn builds a turn whose marshaled size is dominated by a response of n bytes.
func turn(tag string, n int) transcript.Turn {
	return transcript.Turn{Prompt: tag, Response: strings.Repeat("x", n)}
}

func marshalLen(t *testing.T, v any) int {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return len(b)
}

func TestTurnsWithinBudgetKeepsTheNewestTail(t *testing.T) {
	// Five turns of ~1 KB each; a budget that fits about two.
	turns := []transcript.Turn{turn("a", 1000), turn("b", 1000), turn("c", 1000), turn("d", 1000), turn("e", 1000)}
	kept, dropped := turnsWithinBudget(turns, 2400)

	if len(kept)+dropped != len(turns) {
		t.Fatalf("kept %d + dropped %d != %d input turns", len(kept), dropped, len(turns))
	}
	if len(kept) == 0 || len(kept) == len(turns) {
		t.Fatalf("kept %d turns; want a proper subset for this budget", len(kept))
	}
	// The tail is what the reader opened chat to see — the newest turn must survive.
	if kept[len(kept)-1].Prompt != "e" {
		t.Errorf("last kept turn = %q; want the NEWEST turn 'e'", kept[len(kept)-1].Prompt)
	}
	if marshalLen(t, kept) > 2400 {
		t.Errorf("kept payload = %d bytes; want within the 2400 budget", marshalLen(t, kept))
	}
}

func TestTurnsWithinBudgetLeavesAShortHistoryWhole(t *testing.T) {
	turns := []transcript.Turn{turn("a", 50), turn("b", 50)}
	kept, dropped := turnsWithinBudget(turns, transcriptByteBudget)
	if len(kept) != 2 || dropped != 0 {
		t.Errorf("kept %d dropped %d; want the whole 2-turn history untouched", len(kept), dropped)
	}
}

// Serving nothing is worse than serving the heavy turn the user is actually looking at.
func TestTurnsWithinBudgetAlwaysKeepsTheNewestTurn(t *testing.T) {
	turns := []transcript.Turn{turn("old", 100), turn("huge", 5000)}
	kept, dropped := turnsWithinBudget(turns, 500)
	if len(kept) != 1 || kept[0].Prompt != "huge" {
		t.Fatalf("kept %+v; want exactly the newest turn even though it exceeds the budget", kept)
	}
	if dropped != 1 {
		t.Errorf("dropped = %d; want 1", dropped)
	}
}

// The regression in its own terms: a turn COUNT under the cap says nothing about size.
func TestATurnCountUnderTheCapCanStillBeAHugePayload(t *testing.T) {
	// 147 turns — half of maxTranscriptTurns — each ~12 KB, as measured on the real
	// session that crashed the app.
	var turns []transcript.Turn
	for i := 0; i < 147; i++ {
		turns = append(turns, turn("t", 12_000))
	}
	if n := len(turns); n >= maxTranscriptTurns {
		t.Fatalf("fixture has %d turns; the point is that it is UNDER the %d cap", n, maxTranscriptTurns)
	}
	if raw := marshalLen(t, turns); raw <= transcriptByteBudget {
		t.Fatalf("fixture marshals to %d bytes; it must exceed the %d budget to test anything", raw, transcriptByteBudget)
	}
	kept, dropped := turnsWithinBudget(turns, transcriptByteBudget)
	if dropped == 0 {
		t.Fatal("nothing was dropped — the turn cap alone would have served the whole 1.7 MB")
	}
	if got := marshalLen(t, kept); got > transcriptByteBudget {
		t.Errorf("served payload = %d bytes; want within %d", got, transcriptByteBudget)
	}
}
