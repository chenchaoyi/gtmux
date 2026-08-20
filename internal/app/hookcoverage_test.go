package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hook"
)

// TestEveryRegisteredEventClassifies pairs the two halves of the hook feature: the
// events gtmux REGISTERS with each agent (this package) and the table that says what
// they MEAN (internal/hook). Nothing compared them before, so registering an event
// with no mapping produced no error, no warning and no event — `StopFailure` was
// registered with Claude, given a branch in decide, documented in CLAUDE.md as a wake
// class and written into two specs, while every one that fired was dropped: zero in
// 19k recorded events.
//
// A hook we ask an agent to fire and then ignore is worse than one we never asked
// for: it costs the agent a subprocess per event and tells us nothing.
func TestEveryRegisteredEventClassifies(t *testing.T) {
	for _, h := range hookEvents {
		if !hook.EventIsMapped("claude", h.event) {
			t.Errorf("claude: %s is registered but classifies to nothing", h.event)
		}
	}

	for key, inst := range agentInstallers {
		for _, b := range inst.bindings {
			if !hook.EventIsMapped(key, b.event) {
				t.Errorf("%s: %s (native key %q) is registered but classifies to nothing",
					key, b.event, b.key)
			}
		}
	}
}

// The reverse is NOT an error and must not be asserted: a mapping with no
// registration is how a table stays ready for an agent that gains the event later,
// and for events other agents already send.
