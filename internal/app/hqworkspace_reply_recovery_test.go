package app

import "testing"

func TestReplyRecoversOnlyProvenUnsentAction(t *testing.T) {
	for _, scenario := range []string{
		"enabled-counter-final", "enabled-counter", "enabled-counter-message", "enabled-counter-resume",
		"enabled-counter-fallback", "enabled-counter-fallback-resume",
		"final-before-dispatch", "final-event", "final-message", "final-same-authority", "final-older-event",
		"final-after-claim", "final-unknown", "final-accepted", "final-unproven",
	} {
		t.Run(scenario, func(t *testing.T) { exerciseReplyCoverage(t, scenario) })
	}
}
