package limits

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadConfig reads the limits keys from ~/.config/gtmux/usage.json (shared with
// usage-watch's thresholds), merging over DefaultConfig. Absent file/keys → the
// defaults. `limitsCommand:""` disables the feature.
//
//	{"limitsCommand": "claude -p /usage", "limitsTTLMin": 15,
//	 "limitsTTLNearMin": 5, "limitsNearPct": 70, "limitsWarnPct": 85}
func LoadConfig() Config {
	cfg := DefaultConfig
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "usage.json"))
	if err != nil {
		return cfg
	}
	var raw struct {
		Command *string `json:"limitsCommand"`
		TTLMin  *int    `json:"limitsTTLMin"`
		NearMin *int    `json:"limitsTTLNearMin"`
		NearPct *int    `json:"limitsNearPct"`
		WarnPct *int    `json:"limitsWarnPct"`
	}
	if json.Unmarshal(b, &raw) != nil {
		return cfg
	}
	if raw.Command != nil {
		cfg.Command = *raw.Command
	}
	if raw.TTLMin != nil && *raw.TTLMin > 0 {
		cfg.TTLMin = *raw.TTLMin
	}
	if raw.NearMin != nil && *raw.NearMin > 0 {
		cfg.NearMin = *raw.NearMin
	}
	if raw.NearPct != nil && *raw.NearPct > 0 {
		cfg.NearPct = *raw.NearPct
	}
	if raw.WarnPct != nil && *raw.WarnPct > 0 {
		cfg.WarnPct = *raw.WarnPct
	}
	return cfg
}
