package hqfeed

import (
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

// The surfacing-threshold config (hq-attention-system §4): how high the bar sits for
// HQ to PRINT an item to the user. `surfaceTier` (critical|normal|quiet) is the
// minimum tier to surface; a `quiet` bool is a fast toggle equivalent to
// critical-only. Resolution mirrors agentenv (env override → config → default),
// env-overridable for a per-session switch. HQ reads the resolved threshold and gates
// its own prints — the config sets the bar, HQ makes the call.

// DefaultThreshold is the surfacing bar when nothing is configured: surface NORMAL and
// above, matching today's attention level (this change is a de-flood, not a regression).
const DefaultThreshold = SurfaceNormal

type surfaceConfig struct {
	SurfaceTier *string `json:"surfaceTier"`
	Quiet       *bool   `json:"quiet"`
}

func loadSurfaceConfig() surfaceConfig {
	var c surfaceConfig
	_ = usercfg.Load(&c)
	return c
}

// validTier reports whether a value names a known surfacing tier.
func validTier(v string) bool {
	switch v {
	case SurfaceCritical, SurfaceNormal, SurfaceQuiet:
		return true
	}
	return false
}

// ResolveThreshold returns the MINIMUM surfacing tier to show the user, resolved in
// order (first decisive wins): env GTMUX_SURFACE_TIER (a valid tier), env GTMUX_QUIET
// (truthy → critical-only), config `quiet:true` (→ critical-only), config
// `surfaceTier` (a valid tier), else DefaultThreshold (normal).
func ResolveThreshold() string {
	if v := strings.TrimSpace(strings.ToLower(os.Getenv("GTMUX_SURFACE_TIER"))); validTier(v) {
		return v
	}
	if truthy(os.Getenv("GTMUX_QUIET")) {
		return SurfaceCritical
	}
	c := loadSurfaceConfig()
	if c.Quiet != nil && *c.Quiet {
		return SurfaceCritical
	}
	if c.SurfaceTier != nil {
		if v := strings.TrimSpace(strings.ToLower(*c.SurfaceTier)); validTier(v) {
			return v
		}
	}
	return DefaultThreshold
}

// QuietOn reports whether the resolved threshold is critical-only (quiet mode).
func QuietOn() bool { return ResolveThreshold() == SurfaceCritical }

// ShouldSurface reports whether an event of the given SEVERITY clears the resolved
// threshold — i.e. HQ would print it. Degradation is not gated here: the watchdog
// surfaces a feed-degraded unconditionally (a perception outage is never quieted),
// and the seed teaches HQ the same, so this is only the ordinary event gate.
func ShouldSurface(severity string) bool {
	return SurfaceRank(SurfaceTier(severity)) >= SurfaceRank(ResolveThreshold())
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}
