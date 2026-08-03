package hqwake

import (
	"encoding/json"
	"os"

	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

// Done-wake modes (config hqWake.done).
const (
	DoneUnattended = "unattended" // default: wake unless the pane is focused+attached
	DoneAlways     = "always"     // wake on every completion
	DoneTick       = "tick"       // never wake immediately; tick-batch only
)

// Config are the wake-channel knobs (~/.config/gtmux/config.json, `hqWake` object).
// Absent file/keys → defaults; every field is optional.
type Config struct {
	Done          string // done-wake mode: unattended (default) | always | tick
	PaneMinGapSec int64  // per-pane done merge window, seconds (default 120)
	TickMinutes   int64  // summary-tick minimum interval (default 10)
	TickBurst     int    // outcome count that fires the tick early (default 5)
	// UnreadDebounceSec is how long unconsumed events must STAND before the completeness
	// net knocks — the aggregation window that turns a burst into one line. It must
	// comfortably outlast the ordinary knock → wake → pull round trip, or the net would
	// fire about events HQ is already on its way to reading. 0 knocks on the next tick.
	UnreadDebounceSec int64
	// UnreadRepeatSec is the re-knock interval while the watermark stays put. This is the
	// "still owed" cadence: consumption is the ONLY thing that stops it, which is what
	// makes the guarantee a guarantee rather than a single best-effort knock.
	UnreadRepeatSec int64
	// Self-rotation thresholds — when the HQ session itself is no longer fit to judge.
	// Each is INDEPENDENTLY disableable by setting it to 0 or less, because they measure
	// genuinely different kinds of wear: context occupancy is what blurs the boundary
	// between HQ's own output and its input, age is what accumulates stale posture, and
	// turn count is what a burst-heavy day runs up long before either clock says so.
	// Defaults are deliberately conservative — a knock the user does not believe is worse
	// than no knock, and each criterion has room to be tightened per machine.
	SelfRotateCtx   float64 // live context fraction 0–1 (default 0.75)
	SelfRotateHours int64   // session age in hours (default 12)
	SelfRotateTurns int     // HQ turns since the session began (default 300)
	// SelfRotateRepeatSec paces the re-knock while the breach stands. Long by design: a
	// rotation is a handoff, not a reflex, and HQ needs room to finish the board and
	// knowledge-base writes that are the whole point of doing it deliberately.
	SelfRotateRepeatSec int64
	// SelfRotateCheckSec is how often the sensor may EVALUATE — the pacing on its only
	// non-trivial reads (the usage snapshot and the event-delta scan behind the turn
	// count). The 20 s slow tick is far finer than anything this measures.
	SelfRotateCheckSec int64
}

// Defaults returns the documented default config.
func Defaults() Config {
	return Config{Done: DoneUnattended, PaneMinGapSec: 120, TickMinutes: 10, TickBurst: 5,
		UnreadDebounceSec: 120, UnreadRepeatSec: 300,
		SelfRotateCtx: 0.75, SelfRotateHours: 12, SelfRotateTurns: 300,
		SelfRotateRepeatSec: 1800, SelfRotateCheckSec: 300}
}

// Load reads the hqWake config, falling back per-field to defaults.
func Load() Config {
	return loadFrom(usercfg.Path())
}

func loadFrom(path string) Config {
	cfg := Defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	var c struct {
		HQWake struct {
			Done              *string `json:"done"`
			PaneMinGapSec     *int64  `json:"paneMinGapSec"`
			TickMinutes       *int64  `json:"tickMinutes"`
			TickBurst         *int    `json:"tickBurst"`
			UnreadDebounceSec *int64  `json:"unreadDebounceSec"`
			UnreadRepeatSec   *int64  `json:"unreadRepeatSec"`

			SelfRotateCtx       *float64 `json:"selfRotateCtx"`
			SelfRotateHours     *int64   `json:"selfRotateHours"`
			SelfRotateTurns     *int     `json:"selfRotateTurns"`
			SelfRotateRepeatSec *int64   `json:"selfRotateRepeatSec"`
			SelfRotateCheckSec  *int64   `json:"selfRotateCheckSec"`
		} `json:"hqWake"`
	}
	if json.Unmarshal(b, &c) != nil {
		return cfg
	}
	if c.HQWake.Done != nil {
		switch *c.HQWake.Done {
		case DoneUnattended, DoneAlways, DoneTick:
			cfg.Done = *c.HQWake.Done
		}
	}
	if c.HQWake.PaneMinGapSec != nil && *c.HQWake.PaneMinGapSec >= 0 {
		cfg.PaneMinGapSec = *c.HQWake.PaneMinGapSec
	}
	if c.HQWake.TickMinutes != nil && *c.HQWake.TickMinutes > 0 {
		cfg.TickMinutes = *c.HQWake.TickMinutes
	}
	if c.HQWake.TickBurst != nil && *c.HQWake.TickBurst > 0 {
		cfg.TickBurst = *c.HQWake.TickBurst
	}
	if c.HQWake.UnreadDebounceSec != nil && *c.HQWake.UnreadDebounceSec >= 0 {
		cfg.UnreadDebounceSec = *c.HQWake.UnreadDebounceSec
	}
	// A non-positive repeat would mean "knock every tick forever" — the one setting that
	// could turn the completeness net into the noise source it is designed not to be.
	if c.HQWake.UnreadRepeatSec != nil && *c.HQWake.UnreadRepeatSec > 0 {
		cfg.UnreadRepeatSec = *c.HQWake.UnreadRepeatSec
	}
	// The three THRESHOLDS take any value, non-positive included — unlike every knob
	// above, 0 here is meaningful rather than absent: it switches that one criterion off
	// while the other two keep watching. A user who distrusts the turn count should be
	// able to silence it without losing the context-occupancy knock that matters most.
	if c.HQWake.SelfRotateCtx != nil {
		cfg.SelfRotateCtx = *c.HQWake.SelfRotateCtx
	}
	if c.HQWake.SelfRotateHours != nil {
		cfg.SelfRotateHours = *c.HQWake.SelfRotateHours
	}
	if c.HQWake.SelfRotateTurns != nil {
		cfg.SelfRotateTurns = *c.HQWake.SelfRotateTurns
	}
	// The two CADENCES keep the usual rule: a non-positive value would mean "knock every
	// tick" / "re-scan every tick", turning a standing hint into the noise (and the cost)
	// it is designed not to be.
	if c.HQWake.SelfRotateRepeatSec != nil && *c.HQWake.SelfRotateRepeatSec > 0 {
		cfg.SelfRotateRepeatSec = *c.HQWake.SelfRotateRepeatSec
	}
	if c.HQWake.SelfRotateCheckSec != nil && *c.HQWake.SelfRotateCheckSec > 0 {
		cfg.SelfRotateCheckSec = *c.HQWake.SelfRotateCheckSec
	}
	return cfg
}
