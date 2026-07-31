// Package servermode is the sensing half of server mode (see openspec change
// server-mode): keeping a Mac running with the lid closed so `serve`/`tunnel`/the
// phone keep working, then reliably giving sleep back.
//
// This package only ever READS. Nothing here changes a system setting — the
// privileged half (the admin authorization and the de-escalation-only guard) is a
// separate phase. Sensing is cgo-free: it shells out to `ioreg` and `pmset`, the
// way internal/resource parses `df`/`memory_pressure`.
//
// Three measured facts drive the design here; each one was a live bug first, so
// none of them is optional (research §3.1):
//
//  1. `pmset -g` / `-g custom` / `-g live` NEVER report `disablesleep`, in either
//     state. Reading state from pmset makes gtmux announce "sleep restored" on a
//     Mac that cannot sleep — the worst failure this feature has.
//  2. The power-management plist LAGS a write (still the old value a second after a
//     successful change) and describes what survives a REBOOT, not what the kernel
//     is doing now. It answers a different question, so it is kept — separately.
//  3. `ioreg`'s IOPMrootDomain.SleepDisabled is the live, unprivileged truth.
//
// And a fourth, about writes rather than reads: `pmset` exits 0 when it refuses for
// lack of privilege. No caller may infer success from an exit code; confirm by
// reading back.
package servermode

// Tiers, named for what the LID can do — the only distinction a user cares about.
// `lid-open` holds a sleep assertion (idle sleep only: a closed lid still sleeps);
// `clamshell` disables sleep outright so the lid may close. Surfaces must never
// present `lid-open` as surviving a closed lid.
const (
	TierLidOpen   = "lid-open"
	TierClamshell = "clamshell"
)

// States reported to every surface.
const (
	StateOn  = "on"  // sleep is disabled right now
	StateOff = "off" // sleep is enabled — the normal machine state
	// StateLapsed: gtmux's own record says server mode is on, but the kernel says
	// sleep is enabled. Something turned it off underneath us (an MDM profile, an OS
	// update, another tool). A closed-lid session that quietly died is worse than one
	// that ended loudly, so this is a state, not a silent correction.
	StateLapsed = "lapsed"
)

// Power sources.
const (
	PowerAC      = "ac"
	PowerBattery = "battery"
)

// Exit reasons. `battery-low` (not "unplugged"): measurement showed closed-lid
// operation survives unplugging, and carrying a closed laptop between rooms is a
// core use case — what ends server mode is running out of charge, not losing the
// adapter.
const (
	ReasonRevoked        = "revoked"
	ReasonBatteryLow     = "battery-low"
	ReasonStaleHeartbeat = "stale-heartbeat"
	ReasonBootReconcile  = "boot-reconcile"
	ReasonThermal        = "thermal"
	ReasonUninstalled    = "uninstalled"
	ReasonLapsed         = "lapsed"
)

// Guard is the de-escalation-only privileged component's presence and health. It is
// the thing that gives sleep back when gtmux is gone, so "absent" is not a neutral
// fact — it means nothing will clean up after us.
type Guard struct {
	Installed bool `json:"installed"`
	Healthy   bool `json:"healthy"`
}

// Exit records why server mode ended. Written by the guard (root, possibly with no
// user session) and merged in on read; treat it as a message, never as authority
// over the current state.
type Exit struct {
	At     int64  `json:"at"`
	Reason string `json:"reason"`
}

// Status is the machine-readable server-mode document — the shape behind
// `gtmux awake --json` and `GET /api/awake`.
//
// There is deliberately NO expiry field: server mode runs until the user turns it
// off or a guardrail ends it.
type Status struct {
	State string `json:"state"`          // on | off | lapsed
	Tier  string `json:"tier,omitempty"` // awake | clamshell

	Since       int64 `json:"since,omitempty"`
	HeartbeatAt int64 `json:"heartbeat_at,omitempty"` // liveness, NOT an expiry

	Power      string `json:"power"`                 // ac | battery
	BatteryPct int    `json:"battery_pct,omitempty"` // omitted when no internal battery

	Guard Guard `json:"guard"`

	// SystemDisableSleep is the LIVE kernel readback (ioreg) — the authority for
	// "will this Mac sleep right now".
	SystemDisableSleep bool `json:"system_disablesleep"`
	// PersistedDisableSleep answers the different question of whether the setting
	// would still be in force after a reboot (it persists), which is what the
	// boot-reconcile and doctor checks care about.
	PersistedDisableSleep bool `json:"persisted_disablesleep"`

	// OwnedByGtmux is the ownership stamp. False with SystemDisableSleep true means
	// somebody else disabled sleep; gtmux reports that and changes nothing.
	OwnedByGtmux bool `json:"owned_by_gtmux"`

	LastExit *Exit `json:"last_exit,omitempty"`

	// Platform is the preflight verdict. It is part of STATUS, not just of the
	// enable path, because a user on an unverified OS deserves to know that before
	// they rely on this — not at the moment they are walking away from the machine.
	Platform Support `json:"platform"`
}

// Current reads the machine and gtmux's own record and reconciles them. It never
// writes and never changes a system setting.
func Current() Status {
	live := SleepDisabled()
	persisted, _ := PersistedSleepDisabled()
	src, pct, hasBattery := Power()

	s := Status{
		Power:                 src,
		SystemDisableSleep:    live,
		PersistedDisableSleep: persisted,
		Guard:                 GuardStatus(),
		Platform:              Supported(),
	}
	if hasBattery {
		s.BatteryPct = pct
	}

	rec, haveRec := LoadState()
	if haveRec {
		s.Tier = rec.Tier
		s.Since = rec.Since
		s.HeartbeatAt = rec.HeartbeatAt
		s.OwnedByGtmux = true
	}
	if e, ok := LoadLastExit(); ok {
		s.LastExit = &e
	}

	switch {
	case live:
		// The kernel is the authority on "on". Whether WE own it is a separate axis.
		s.State = StateOn
	case haveRec:
		// We think it is on; the kernel disagrees. Report the disagreement.
		s.State = StateLapsed
	default:
		s.State = StateOff
	}
	return s
}
