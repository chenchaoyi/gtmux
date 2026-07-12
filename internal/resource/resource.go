// Package resource is the resource-watch layer (see openspec resource-watch):
// a deterministic, cgo-free snapshot of local machine resources (disk / memory /
// CPU), per-agent attribution (RSS + CPU by pane process tree, isomorphic to
// token accounting), and actionable reclaim candidates (heavy orphan processes
// no live pane owns). HQ weighs these when dispatching and, when severe, advises
// reclaim or holding new sessions.
//
// Sampling shells out to df / sysctl / memory_pressure / ps (macOS built-ins;
// Linux fallbacks where noted) — no cgo.
package resource

import "fmt"

// Tier is a severity level shared by the resources.
type Tier int

const (
	TierNormal Tier = iota
	TierAmber       // warn
	TierRed         // critical
)

func (t Tier) String() string {
	switch t {
	case TierAmber:
		return "amber"
	case TierRed:
		return "red"
	default:
		return "normal"
	}
}

// Machine is the whole-machine resource snapshot.
type Machine struct {
	DiskFreeGB int     `json:"disk_free_gb"`
	DiskUsePct int     `json:"disk_use_pct"`
	MemFreePct int     `json:"mem_free_pct"`
	MemTier    string  `json:"mem_tier"`   // normal | warn | critical (kernel pressure level)
	LoadRatio  float64 `json:"load_ratio"` // 1-min loadavg ÷ ncpu
	NCPU       int     `json:"ncpu"`
	Warn       string  `json:"warn,omitempty"` // the first resource at/over amber, "" = fine
}

// AgentUse is one agent's attributed resource use (its pane process tree).
type AgentUse struct {
	RSSMB int     `json:"rss_mb"`
	CPU   float64 `json:"cpu"` // summed %CPU across the tree
}

// Orphan is a heavy process no live pane owns — a reclaim candidate.
type Orphan struct {
	PID   int     `json:"pid"`
	RSSMB int     `json:"rss_mb"`
	CPU   float64 `json:"cpu"`
	Comm  string  `json:"comm"`
	Kind  string  `json:"kind,omitempty"` // curated label: "simulator" | "dev-server" | "tmux" | ""
	Hint  string  `json:"hint,omitempty"` // how to reclaim
}

// Report is the full resource-watch snapshot (CLI/API/digest shape).
type Report struct {
	Machine Machine             `json:"machine"`
	Agents  map[string]AgentUse `json:"agents,omitempty"` // keyed by pane id
	Orphans []Orphan            `json:"orphans,omitempty"`
}

// Snapshot samples the machine and (given the live panes' pids) attributes use +
// finds reclaim candidates. panePIDs maps a pane id → its root pid (0 to skip).
func Snapshot(panePIDs map[string]int) Report {
	cfg := loadConfig()
	m := sampleMachine()
	m.Warn = evalMachine(m, cfg)
	rep := Report{Machine: m}
	if procs := sampleProcs(); len(procs) > 0 {
		rep.Agents, rep.Orphans = attribute(procs, panePIDs, cfg)
	}
	return rep
}

// evalMachine returns the first resource at/over amber as a compact warn string
// ("" = all normal). Disk first (hard to recover), then memory, then load.
func evalMachine(m Machine, cfg config) string {
	switch diskTier(m, cfg) {
	case TierRed:
		return fmt.Sprintf("disk %dGB free (red)", m.DiskFreeGB)
	case TierAmber:
		return fmt.Sprintf("disk %dGB free", m.DiskFreeGB)
	}
	switch memTierOf(m.MemTier) {
	case TierRed:
		return "memory critical"
	case TierAmber:
		return "memory warn"
	}
	switch loadTier(m.LoadRatio, cfg) {
	case TierRed:
		return fmt.Sprintf("load %.1f×cores (red)", m.LoadRatio)
	case TierAmber:
		return fmt.Sprintf("load %.1f×cores", m.LoadRatio)
	}
	return ""
}

func diskTier(m Machine, cfg config) Tier {
	switch {
	case m.DiskFreeGB > 0 && m.DiskFreeGB < cfg.DiskRedGB:
		return TierRed
	case m.DiskFreeGB > 0 && m.DiskFreeGB < cfg.DiskAmberGB:
		return TierAmber
	default:
		return TierNormal
	}
}

func memTierOf(tier string) Tier {
	switch tier {
	case "critical":
		return TierRed
	case "warn":
		return TierAmber
	default:
		return TierNormal
	}
}

func loadTier(ratio float64, cfg config) Tier {
	switch {
	case ratio >= cfg.LoadRed:
		return TierRed
	case ratio >= cfg.LoadAmber:
		return TierAmber
	default:
		return TierNormal
	}
}

// WarnTier reports the overall machine tier (for surfaces that want a color).
func (m Machine) WarnTier(cfg config) Tier {
	t := diskTier(m, cfg)
	if x := memTierOf(m.MemTier); x > t {
		t = x
	}
	if x := loadTier(m.LoadRatio, cfg); x > t {
		t = x
	}
	return t
}

// MachineTier is the exported overall tier using the live config.
func MachineTier(m Machine) Tier { return m.WarnTier(loadConfig()) }
