package resource

import "github.com/chenchaoyi/gtmux/internal/usercfg"

// config holds the resource-watch thresholds (from ~/.config/gtmux/config.json's
// "resource" object; sensible defaults when absent).
type config struct {
	DiskAmberGB int     `json:"diskAmberGB"` // amber when free < this (GB)
	DiskRedGB   int     `json:"diskRedGB"`   // red   when free < this (GB)
	LoadAmber   float64 `json:"loadAmber"`   // amber when load÷cores >= this
	LoadRed     float64 `json:"loadRed"`     // red   when load÷cores >= this
	OrphanRSSMB int     `json:"orphanRssMB"` // a proc is a reclaim candidate only if RSS >= this
	// Anti-flap (hq-wake-reliability). A threshold is entered at its value above and
	// left only past the hysteresis margin; a change must survive ConfirmSamples
	// consecutive samples; and a tier stays quiet for MinRestateMinutes after it
	// nudged (an escalation is exempt).
	DiskHysteresisGB  int     `json:"diskHysteresisGB"`  // GB of free space above the entry line before it clears
	LoadHysteresis    float64 `json:"loadHysteresis"`    // load÷cores below the entry line before it clears
	ConfirmSamples    int     `json:"confirmSamples"`    // consecutive agreeing samples to commit a tier change
	MinRestateMinutes int     `json:"minRestateMinutes"` // quiet period before the same tier nudges again
}

// Defaults tuned to this machine's acceptance target: 40 GB free → amber (so the
// amber line sits above 40), red well below. Load per-core 1.0/1.5. The anti-flap
// defaults come from the observed dither: disk red at <15 GB clears at 17 GB, and
// load amber at ≥1.0× clears below 0.85× — both wide enough to cover the oscillation
// that was re-alerting, narrow enough that a real trend still crosses in one step.
var defaultConfig = config{
	DiskAmberGB: 50, DiskRedGB: 15, LoadAmber: 1.0, LoadRed: 1.5, OrphanRSSMB: 300,
	DiskHysteresisGB: 2, LoadHysteresis: 0.15, ConfirmSamples: 3, MinRestateMinutes: 30,
}

func loadConfig() config {
	cfg := defaultConfig
	var wrap struct {
		Resource *config `json:"resource"`
	}
	if usercfg.Load(&wrap) != nil || wrap.Resource == nil {
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
	// Anti-flap knobs. Like every field above, an unset one reads as 0 and takes the
	// default — a config that names only `diskAmberGB` still gets working damping.
	// To damp less, set a small value (confirmSamples 1 = commit on one sample).
	if r.DiskHysteresisGB <= 0 {
		r.DiskHysteresisGB = defaultConfig.DiskHysteresisGB
	}
	if r.LoadHysteresis <= 0 {
		r.LoadHysteresis = defaultConfig.LoadHysteresis
	}
	if r.ConfirmSamples <= 0 {
		r.ConfirmSamples = defaultConfig.ConfirmSamples
	}
	if r.MinRestateMinutes <= 0 {
		r.MinRestateMinutes = defaultConfig.MinRestateMinutes
	}
	return r
}
