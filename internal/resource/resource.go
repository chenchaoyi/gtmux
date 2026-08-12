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
	// Tier is the overall severity — "amber" (soft heads-up) | "red" (genuine
	// bottleneck); omitted when normal. Consumers use it to decide how loud to be:
	// only "red" is an act-now bottleneck (the mobile HQ disc reddens on "red" alone,
	// never on a soft "amber" — a 37GB-free amber is not an emergency).
	Tier string `json:"tier,omitempty"`
	// Battery is the power/charge snapshot (macOS `pmset`; nil on a desktop / Linux with
	// no battery). A LOW charge counts toward Warn/Tier only while ON BATTERY — on AC the
	// level is irrelevant. So HQ (and the surfaces) know to plug in before the fleet dies.
	Battery *Battery `json:"battery,omitempty"`
}

// Battery is the machine's power/charge state (macOS `pmset -g batt`).
type Battery struct {
	Present  bool   `json:"present"`             // an internal battery exists (false on a desktop)
	Percent  int    `json:"percent"`             // charge 0–100
	OnAC     bool   `json:"on_ac"`               // drawing from AC power (charging or topped up)
	State    string `json:"state,omitempty"`     // charging | discharging | charged | …(pmset's word)
	TimeLeft string `json:"time_left,omitempty"` // "H:MM" to empty (discharging) / full (charging); "" if none
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
// MachineSnapshot samples ONLY the machine (disk/memory/load + its warn and tier),
// skipping the process attribution Snapshot does.
//
// The distinction is not micro-optimization. Snapshot's second half runs a full-table
// `ps`, and a wedged process once hung that call and froze the whole radar with it
// (memory ps-wedge-freezes-radar). A caller that needs the tier and nothing else —
// the HQ verdict on every digest — must not take that risk on a hot path.
func MachineSnapshot() Machine {
	cfg := loadConfig()
	m := sampleMachine()
	m.Warn = evalMachine(m, cfg)
	if t := m.WarnTier(cfg); t != TierNormal {
		m.Tier = t.String()
	}
	return m
}

func Snapshot(panePIDs map[string]int) Report {
	cfg := loadConfig()
	m := sampleMachine()
	m.Warn = evalMachine(m, cfg)
	if t := m.WarnTier(cfg); t != TierNormal {
		m.Tier = t.String()
	}
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
	switch batteryTier(m, cfg) {
	case TierRed:
		return fmt.Sprintf("battery %d%% (red)", m.Battery.Percent)
	case TierAmber:
		return fmt.Sprintf("battery %d%%", m.Battery.Percent)
	}
	return ""
}

// batteryTier reports the battery's severity — but ONLY while on battery power. On AC
// (charging or topped up) the charge level is not a concern, so it never warns. nil /
// no battery (a desktop) is always normal.
func batteryTier(m Machine, cfg config) Tier {
	b := m.Battery
	if b == nil || !b.Present || b.OnAC || b.Percent <= 0 {
		return TierNormal
	}
	switch {
	case b.Percent < cfg.BatteryRedPct:
		return TierRed
	case b.Percent < cfg.BatteryAmberPct:
		return TierAmber
	default:
		return TierNormal
	}
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
	if x := batteryTier(m, cfg); x > t {
		t = x
	}
	return t
}

// MachineTier is the exported overall tier using the live config.
func MachineTier(m Machine) Tier { return m.WarnTier(loadConfig()) }
