package hq

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resource"
)

// run feeds a sequence of observations through the gate and returns the tiers it
// nudged about, in order.
func run(confirm int, minRestate int64, samples []struct {
	obs string
	at  int64
}) []string {
	var s tierState
	var nudged []string
	for _, sm := range samples {
		next, nudge := tierStep(s, sm.obs, sm.at, confirm, minRestate)
		s = next
		if nudge {
			nudged = append(nudged, sm.obs)
		}
	}
	return nudged
}

type sample = struct {
	obs string
	at  int64
}

// A tier change must survive the confirmation window: a single anomalous reading
// (a `df` caught mid-write, a load spike from one compile) commits nothing.
func TestTierStep_ConfirmationWindow(t *testing.T) {
	// Two amber samples then back to fine, with confirm=3 → nothing was ever believed.
	got := run(3, 0, []sample{{"amber", 10}, {"amber", 30}, {"", 50}})
	if len(got) != 0 {
		t.Fatalf("an unconfirmed spike must not nudge; got %v", got)
	}
	// Three in a row → committed, nudged once.
	got = run(3, 0, []sample{{"amber", 10}, {"amber", 30}, {"amber", 50}, {"amber", 70}})
	if len(got) != 1 || got[0] != "amber" {
		t.Fatalf("a confirmed tier nudges exactly once; got %v", got)
	}
}

// The candidate resets when the observation changes: alternating samples never
// accumulate a confirmation.
func TestTierStep_AlternatingNeverCommits(t *testing.T) {
	got := run(3, 0, []sample{{"amber", 10}, {"red", 30}, {"amber", 50}, {"red", 70}, {"amber", 90}})
	if len(got) != 0 {
		t.Fatalf("a dithering reading confirms nothing; got %v", got)
	}
}

// The commander's痛点: amber → normal → amber inside the restate interval is ONE
// event, not a nudge each way.
func TestTierStep_MinRestateSuppressesAReturn(t *testing.T) {
	const restate = 30 * 60
	got := run(1, restate, []sample{
		{"amber", 100},  // nudge
		{"", 200},       // recovered — commits, says nothing
		{"amber", 300},  // back inside the quiet period → suppressed
		{"", 400},       //
		{"amber", 4000}, // past it → speaks again
	})
	if len(got) != 2 {
		t.Fatalf("a flap inside the quiet period is one nudge, then one after it; got %v", got)
	}
}

// A recovery is committed but never announced — the alert is for trouble.
func TestTierStep_RecoveryIsSilent(t *testing.T) {
	got := run(1, 0, []sample{{"amber", 10}, {"", 20}})
	if len(got) != 1 || got[0] != "amber" {
		t.Fatalf("only the warning speaks; got %v", got)
	}
}

// An ESCALATION must never be silenced by the anti-flap rules. A disk walking
// amber→red inside the quiet period is exactly when HQ needs to hear it.
func TestTierStep_EscalationBypassesTheQuietPeriod(t *testing.T) {
	const restate = 30 * 60
	got := run(1, restate, []sample{
		{"amber", 100}, // nudge
		{"red", 200},   // 100s later — WELL inside the quiet period, but worse
	})
	if len(got) != 2 || got[1] != "red" {
		t.Fatalf("an escalation always speaks; got %v", got)
	}
	// De-escalation red→amber inside the period stays quiet (it's not news).
	got = run(1, restate, []sample{{"red", 100}, {"amber", 200}})
	if len(got) != 1 {
		t.Fatalf("an improvement inside the quiet period is not news; got %v", got)
	}
}

// tierGate persists across calls (the serve tick is a sequence of processes' worth of
// samples, not one loop).
func TestTierGate_PersistsAcrossCalls(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if tierGate("resourcewarn", "amber", 100, 2, 0) {
		t.Fatal("one sample must not commit with confirm=2")
	}
	if !tierGate("resourcewarn", "amber", 120, 2, 0) {
		t.Fatal("the second agreeing sample commits — and the state came from disk")
	}
	if tierGate("resourcewarn", "amber", 140, 2, 0) {
		t.Fatal("a held tier does not re-nudge")
	}
}

// The wiring: a machine sample goes through hysteresis (resource) and then the gate.
func TestResourceTierGate_DitheringOnTheRedLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	red := resource.Machine{DiskFreeGB: 14, MemTier: "normal", NCPU: 8}
	// Three agreeing samples commit red and nudge once.
	var nudges int
	for i, at := 0, int64(100); i < 3; i, at = i+1, at+20 {
		if resourceTierGate(red, at) {
			nudges++
		}
	}
	if nudges != 1 {
		t.Fatalf("crossing into red nudges exactly once; got %d", nudges)
	}
	// Now it dithers just above the entry line but inside the exit band (15.x GB):
	// hysteresis holds red, so there is no crossing to re-announce.
	dither := resource.Machine{DiskFreeGB: 16, MemTier: "normal", NCPU: 8}
	for i, at := 0, int64(200); i < 6; i, at = i+1, at+20 {
		if resourceTierGate(dither, at) || resourceTierGate(red, at+10) {
			t.Fatalf("a value dithering on the threshold must not re-alert (sample %d)", i)
		}
	}
}
