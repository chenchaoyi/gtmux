package dispatch

import "github.com/chenchaoyi/gtmux/internal/transcript"

// NormalizeNeedle is the ONE pipeline that produces the content fingerprint both
// sides of the delivery receipt compare: the hook records a UserPromptSubmit's
// Summary with it, and Deliver derives its event-matching needle with it. Before
// it existed the two sides normalized on separate tracks — the hook CLEANED the
// prompt (stripping harness blocks and gtmux's own wake lines) before taking the
// head, while the verifier took the head of the RAW payload — so a payload with a
// strippable prefix produced two different fingerprints, the genuine submit event
// was silently ignored, and the delivery was misjudged NOT delivered by the
// screen fallback (openspec change agent-drivers, P1).
//
// A payload that does not survive cleaning (all-injected content) falls back to
// the head of the raw text: the hook records no Summary for such a submission, so
// no event will match either way and the judgment defers to the screen (I2 — the
// absence of driver evidence is never failure, only a fall to Layer 1).
func NormalizeNeedle(s string) string {
	if clean, ok := transcript.CleanUserPrompt(s); ok {
		return NormalizeHead(clean)
	}
	return NormalizeHead(s)
}
