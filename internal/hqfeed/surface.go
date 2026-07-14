package hqfeed

import "github.com/chenchaoyi/gtmux/internal/events"

// Surfacing tiers (hq-attention-system §2) — how an event maps onto the user-facing
// gate. They are DERIVED from the event's deterministic severity, not a second
// classifier: important → CRITICAL (surface), notable → NORMAL (surface, per the
// threshold), routine → QUIET (ledger only). HQ owns the actual print decision; this
// just names the tiers the threshold config and the ledger reason about.
const (
	SurfaceCritical = "critical"
	SurfaceNormal   = "normal"
	SurfaceQuiet    = "quiet"
)

// SurfaceTier maps an event severity to its surfacing tier. An empty/unknown
// severity (a legacy record) maps to QUIET — the safe, non-flooding default.
func SurfaceTier(severity string) string {
	switch severity {
	case events.SevImportant:
		return SurfaceCritical
	case events.SevNotable:
		return SurfaceNormal
	default:
		return SurfaceQuiet
	}
}

// SurfaceRank orders the tiers for "this tier and above" threshold comparisons
// (quiet < normal < critical). An unknown tier ranks as quiet (0).
func SurfaceRank(tier string) int {
	switch tier {
	case SurfaceNormal:
		return 1
	case SurfaceCritical:
		return 2
	default:
		return 0
	}
}
