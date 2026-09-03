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

// The registration is the half a behaviour test cannot see. PostToolUse classifies as
// Resumed and Resumed clears a waiting mark, both of which stayed true the whole time
// this was broken — because a matcher meant the event never arrived for the tools that
// actually gate. Scoping it again would restore the bug with every other test green.
func TestPostToolUseIsRegisteredForEveryTool(t *testing.T) {
	for _, h := range hookEvents {
		if h.event != "PostToolUse" {
			continue
		}
		if h.matcher != "" {
			t.Fatalf("PostToolUse is scoped to %q — a permission on a Read or a Bash then has "+
				"nothing to clear its waiting mark, which is the 2026-09-03 bug", h.matcher)
		}
		return
	}
	t.Fatal("PostToolUse is not registered at all — an approved tool would never clear its wait")
}
