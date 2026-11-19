package app

import "testing"

func TestReplyRequiresProvenRecordingBaseline(t *testing.T) {
	for _, scenario := range []string{
		"cold-message", "cold-legacy", "cold-zero", "cold-message-resume",
		"counter-read-only-message", "counter-read-only", "counter-read-only-after-arm", "counter-read-only-final",
		"counter-read-only-resume", "counter-read-only-remove-policy",
	} {
		t.Run(scenario, func(t *testing.T) { exerciseReplyCoverage(t, scenario) })
	}
}
