package hq

import "testing"

// AUDIT 1.1 (standing-wake-backoff): could the CURRENT gate produce the measured
// `resource·warn` incident — 4 knocks in 40 minutes, byte-identical payload, while the
// metrics IMPROVED (2026-08-04)?
//
// Answer: no. This test drives the real transition function at the shipped defaults and
// shows the incident is unreachable, which settles the audit's fork: the spec'd dedup was
// not merely written, it forbids the behavior — so the incident serve was running older
// code. (A launchd serve keeps its binary until restarted; the repo had tierGate from
// 2026-07-18 #496 and hysteresis from 2026-08-01 #653, both before the incident.)
//
// Consequence for the change: form 1 (identical-payload repeats) needs no new mechanism.
// What remains is delivery-side revalidation, which is a different failure — the premise
// dying between the decision and the delivery.
func TestAudit_ResourceWarnIdenticalRepeatsAreUnreachable(t *testing.T) {
	const confirm, minRestate = 3, int64(30 * 60)
	var s tierState
	nudges := []int64{}
	// 40 minutes of samples at the 20 s tick, amber throughout (the incident's shape:
	// one steady tier, payload byte-identical).
	for now := int64(0); now <= 40*60; now += 20 {
		var nudge bool
		s, nudge = tierStep(s, "amber", now, confirm, minRestate)
		if nudge {
			nudges = append(nudges, now)
		}
	}
	if len(nudges) != 1 {
		t.Fatalf("a steady amber knocked %d times in 40m (at %v) — want exactly 1", len(nudges), nudges)
	}

	// Even the adversarial version — the tier DITHERING amber↔normal all 40 minutes,
	// which is the only way to re-arm the gate — cannot reach 4, because a restated
	// tier still owes the 30-minute quiet period.
	s = tierState{}
	nudges = nudges[:0]
	for i, now := 0, int64(0); now <= 40*60; i, now = i+1, now+20 {
		obs := "amber"
		if i%6 < 3 { // three samples normal, three amber — commits each way, fastest flap
			obs = ""
		}
		var nudge bool
		s, nudge = tierStep(s, obs, now, confirm, minRestate)
		if nudge {
			nudges = append(nudges, now)
		}
	}
	if len(nudges) > 2 {
		t.Fatalf("a dithering tier knocked %d times in 40m (at %v) — the quiet period should cap it at 2", len(nudges), nudges)
	}
}

// The direction that must never be lost to any anti-flap rule: a genuine ESCALATION
// (amber → red) speaks immediately, even inside the quiet period.
func TestAudit_EscalationIsNeverSuppressed(t *testing.T) {
	const confirm, minRestate = 3, int64(30 * 60)
	var s tierState
	for now := int64(0); now < 3*20; now += 20 {
		s, _ = tierStep(s, "amber", now, confirm, minRestate)
	}
	var nudge bool
	for now := int64(60); now < 60+3*20; now += 20 {
		s, nudge = tierStep(s, "red", now, confirm, minRestate)
	}
	if !nudge {
		t.Fatal("amber→red inside the quiet period was suppressed — an escalation must always speak")
	}
}
