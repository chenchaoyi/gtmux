package hqwake

import (
	"encoding/json"
	"os"
	"path/filepath"
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
}

// Defaults returns the documented default config.
func Defaults() Config {
	return Config{Done: DoneUnattended, PaneMinGapSec: 120, TickMinutes: 10, TickBurst: 5}
}

// Load reads the hqWake config, falling back per-field to defaults.
func Load() Config {
	return loadFrom(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json"))
}

func loadFrom(path string) Config {
	cfg := Defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	var c struct {
		HQWake struct {
			Done          *string `json:"done"`
			PaneMinGapSec *int64  `json:"paneMinGapSec"`
			TickMinutes   *int64  `json:"tickMinutes"`
			TickBurst     *int    `json:"tickBurst"`
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
	return cfg
}
