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
}

// Defaults returns the documented default config.
func Defaults() Config {
	return Config{Done: DoneUnattended, PaneMinGapSec: 120, TickMinutes: 10, TickBurst: 5,
		UnreadDebounceSec: 120, UnreadRepeatSec: 300}
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
	return cfg
}
