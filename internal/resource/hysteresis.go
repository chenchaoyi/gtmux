package resource

// Threshold hysteresis (hq-wake-reliability). By-tier dedup already stops a value
// jittering WITHIN a tier from re-nudging (40→39→38 GB, all amber). It does nothing
// for a value dithering ON a tier boundary: 15.1→14.9→15.1 GB crosses the red line
// each way, and each crossing is a "new" tier — so HQ gets nudged, again, about a disk
// that has not meaningfully changed. Load is worse: the amber line (1.0× cores) sits
// exactly where a busy machine's load ratio oscillates.
//
// The fix is one threshold per DIRECTION. A tier is ENTERED at its configured
// threshold and LEFT only once the sample clears an exit margin — red at under 15 GB
// free clears only at 17 GB or more.
//
// Deliberately NOT applied to the reported snapshot: `gtmux resource`, the digest, and
// GET /api/usage keep reporting raw truth. Hysteresis governs the ALERT (who gets
// woken), not the readout (a display that jitters wakes nobody).

// relaxed returns cfg with every threshold moved AWAY from the alarm by its
// hysteresis margin — the EXIT thresholds. Memory is absent by design: its tier is
// the kernel's own pressure level, already discrete and already damped.
func relaxed(cfg config) config {
	out := cfg
	out.DiskAmberGB += cfg.DiskHysteresisGB
	out.DiskRedGB += cfg.DiskHysteresisGB
	out.LoadAmber -= cfg.LoadHysteresis
	out.LoadRed -= cfg.LoadHysteresis
	if out.LoadAmber < 0 {
		out.LoadAmber = 0
	}
	if out.LoadRed < 0 {
		out.LoadRed = 0
	}
	return out
}

// MachineTierSticky returns m's tier given the tier currently HELD (prev): rising or
// holding is decided by the entry thresholds, while falling requires the sample to
// clear the exit band. A sample inside the band holds prev — the tier does not move
// until the machine has really moved.
//
// prev is TierNormal on a first sample, which makes the first crossing behave exactly
// like the un-damped rule (hysteresis can only ever delay a FALL).
func MachineTierSticky(prev Tier, m Machine) Tier {
	cfg := loadConfig()
	if raw := m.WarnTier(cfg); raw >= prev {
		return raw // rising or unchanged: the entry thresholds decide
	}
	if exit := m.WarnTier(relaxed(cfg)); exit < prev {
		return exit // cleared the exit band — fall (to the relaxed verdict)
	}
	return prev // inside the band: hold
}

// ConfirmSamples is how many consecutive samples must agree before a tier change is
// believed. It bounds the OTHER flap source hysteresis cannot see: a single anomalous
// reading (a `df` mid-write, a load spike from one compile).
func ConfirmSamples() int { return loadConfig().ConfirmSamples }

// MinRestateSecs is how long the same tier stays quiet after being nudged. An
// ESCALATION is exempt — see the tier gate.
func MinRestateSecs() int64 { return int64(loadConfig().MinRestateMinutes) * 60 }
