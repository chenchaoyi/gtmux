package events

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// A refusal is not a failure. The draft guard declining to type into a pane that holds
// someone's unsubmitted line is the guard working, and recording it as `failed` put a red
// mark in the Diagnostics window on gtmux doing the right thing (2026-09-20).
func TestOutcomeSeparatesARefusalFromAFailure(t *testing.T) {
	cases := map[string]string{
		"landed":            diag.OK,
		"queued":            diag.OK,
		"sent":              diag.OK,
		"staged":            diag.OK,
		"failed":            diag.Failed,
		"refused-draft":     diag.Refused,
		"refused-duplicate": diag.Refused,
	}
	for state, want := range cases {
		if got := Outcome(state); got != want {
			t.Errorf("Outcome(%q) = %q, want %q", state, got, want)
		}
	}
}

// The sentence follows the outcome, and the two unverified paths keep their own wording.
func TestSendOutcomeKeepsItsSentences(t *testing.T) {
	if out, msg := sendOutcome("refused-draft"); out != diag.Refused || msg != "typing into a pane was refused" {
		t.Errorf("refused-draft → %q, %q", out, msg)
	}
	if out, msg := sendOutcome("staged"); out != diag.OK || msg != "typed into a pane without submitting" {
		t.Errorf("staged → %q, %q", out, msg)
	}
	if out, msg := sendOutcome("landed"); out != diag.OK || msg != "typed into a pane" {
		t.Errorf("landed → %q, %q", out, msg)
	}
}
