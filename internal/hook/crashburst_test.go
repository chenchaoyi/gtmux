package hook

import (
	"testing"
)

// A dropped connection is not N incidents because N agents noticed it.
//
// Measured 2026-09-04: the commander's network went down while a pane ran a batch of
// parallel subagents, and the drop landed as twelve `StopFailure server_error` records —
// one per dying subagent, one per retry of the main session. HQ was knocked twelve times
// about one thing, plus five unread knocks behind them. The wake line became the noise it
// exists to cut through.
func TestCrashBurstIsOneIncident(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const err = "Connection dropped (ECONNRESET) (error type server_error)"
	now := int64(1_788_800_000)

	n, first := crashBurstCount("%7", err, now)
	if !first || n != 1 {
		t.Fatalf("the first failure must open the incident: n=%d first=%v", n, first)
	}
	// The eleven that followed, all within a couple of minutes.
	for i := 1; i < 12; i++ {
		n, first = crashBurstCount("%7", err, now+int64(i*10))
		if first {
			t.Fatalf("record %d opened a second incident", i+1)
		}
	}
	if n != 12 {
		t.Errorf("the incident counted %d records, want 12 — the count is what tells HQ it was a storm", n)
	}
}

func TestADifferentFailureIsADifferentIncident(t *testing.T) {
	// Keyed on pane AND error: a real crash arriving during a network storm must not be
	// swallowed by it.
	t.Setenv("HOME", t.TempDir())
	now := int64(1_788_800_000)
	crashBurstCount("%7", "ECONNRESET", now)
	if _, first := crashBurstCount("%7", "panic: nil map", now+5); !first {
		t.Error("a different error on the same pane was suppressed")
	}
	if _, first := crashBurstCount("%9", "ECONNRESET", now+5); !first {
		t.Error("the same error on a different pane was suppressed")
	}
}

func TestALaterFailureSpeaksAgain(t *testing.T) {
	// Suppression must not become forgetting: past the window it is news again.
	t.Setenv("HOME", t.TempDir())
	const err = "server_error"
	now := int64(1_788_800_000)
	crashBurstCount("%7", err, now)
	if _, first := crashBurstCount("%7", err, now+crashBurstWindow-1); first {
		t.Error("a repeat inside the window opened a new incident")
	}
	if _, first := crashBurstCount("%7", err, now+crashBurstWindow); !first {
		t.Error("a failure after the window stayed silent — suppression became forgetting")
	}
}
