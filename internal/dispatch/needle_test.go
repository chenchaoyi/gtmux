package dispatch

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// TestNormalizeNeedle_OneTrackWithTheHook pins the single-track guarantee: for any
// payload the hook would record (CleanUserPrompt ok), the needle equals the head of
// the CLEANED text — exactly what hook.eventSummary writes as the event Summary.
func TestNormalizeNeedle_OneTrackWithTheHook(t *testing.T) {
	payloads := []string{
		"cut a new release and update the cask",
		"  Please  refactor   the verifier  ",
		// A trailing harness block must not change the fingerprint.
		"fix the flaky test in deliver_test\n<system-reminder>context low</system-reminder>",
		// A leading gtmux wake line is stripped by the hook's cleaning — the OLD
		// dual-track needle kept it and never matched the event (the misjudgment bug).
		"» gtmux·done  gtmux:0.0 (%14) │ goal:\"x\"\n继续 P2，按 tasks.md 逐项落地",
		strings.Repeat("多字节长指令 ", 30),
	}
	for _, p := range payloads {
		clean, ok := transcript.CleanUserPrompt(p)
		if !ok {
			t.Fatalf("payload should survive cleaning: %q", p)
		}
		if got, want := NormalizeNeedle(p), NormalizeHead(clean); got != want {
			t.Errorf("NormalizeNeedle(%q) = %q, want the cleaned head %q", p, got, want)
		}
	}
}

// A payload that is ALL injected content records no event Summary, so the needle
// falls back to the raw head — it will match nothing on the stream (NoEvidence)
// and the screen, which shows the raw paste, judges instead.
func TestNormalizeNeedle_AllInjectedFallsBackToRaw(t *testing.T) {
	p := "<system-reminder>context low</system-reminder>"
	if _, ok := transcript.CleanUserPrompt(p); ok {
		t.Fatal("fixture must not survive cleaning")
	}
	if got, want := NormalizeNeedle(p), NormalizeHead(p); got != want {
		t.Errorf("fallback needle = %q, want raw head %q", got, want)
	}
	if NormalizeNeedle(p) == "" {
		t.Error("fallback needle must not be empty (the screen match still needs a fingerprint)")
	}
}
