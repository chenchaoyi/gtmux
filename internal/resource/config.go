package resource

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// config holds the resource-watch thresholds (from ~/.config/gtmux/config.json's
// "resource" object; sensible defaults when absent).
type config struct {
	DiskAmberGB int     `json:"diskAmberGB"` // amber when free < this (GB)
	DiskRedGB   int     `json:"diskRedGB"`   // red   when free < this (GB)
	LoadAmber   float64 `json:"loadAmber"`   // amber when load÷cores >= this
	LoadRed     float64 `json:"loadRed"`     // red   when load÷cores >= this
	OrphanRSSMB int     `json:"orphanRssMB"` // a proc is a reclaim candidate only if RSS >= this
}

// Defaults tuned to this machine's acceptance target: 40 GB free → amber (so the
// amber line sits above 40), red well below. Load per-core 1.0/1.5.
var defaultConfig = config{DiskAmberGB: 50, DiskRedGB: 15, LoadAmber: 1.0, LoadRed: 1.5, OrphanRSSMB: 300}

func loadConfig() config {
	cfg := defaultConfig
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json"))
	if err != nil {
		return cfg
	}
	var wrap struct {
		Resource *config `json:"resource"`
	}
	if json.Unmarshal(b, &wrap) != nil || wrap.Resource == nil {
		return cfg
	}
	r := *wrap.Resource
	if r.DiskAmberGB <= 0 {
		r.DiskAmberGB = defaultConfig.DiskAmberGB
	}
	if r.DiskRedGB <= 0 {
		r.DiskRedGB = defaultConfig.DiskRedGB
	}
	if r.LoadAmber <= 0 {
		r.LoadAmber = defaultConfig.LoadAmber
	}
	if r.LoadRed <= 0 {
		r.LoadRed = defaultConfig.LoadRed
	}
	if r.OrphanRSSMB <= 0 {
		r.OrphanRSSMB = defaultConfig.OrphanRSSMB
	}
	return r
}
