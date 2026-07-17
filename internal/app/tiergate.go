package app

// The tier gate (hq-wake-reliability): the TIMING half of the anti-flap rules, kept
// deliberately apart from the hysteresis half. Hysteresis is a property of the
// thresholds and lives in internal/resource; how often a tier may SPEAK is a property
// of the alert and lives here.
//
// Two rules, one exemption:
//
//   - a tier change commits only after N consecutive samples agree (a `df` caught
//     mid-write, a load spike from one compile, must not commit anything);
//   - a committed tier stays quiet for a restate interval after nudging, so
//     amber→normal→amber inside a minute is one nudge, not two;
//   - UNLESS the new tier is strictly MORE severe than the one last nudged. A disk
//     walking amber→red must never be silenced by an anti-flap rule.
//
// The state machine is pure and the state is a small JSON marker, so the whole thing
// is testable without df/tmux/an HQ.

import (
	"encoding/json"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// tierState is one gated alert's persisted state.
type tierState struct {
	Tier    string `json:"tier"`     // the tier currently HELD (committed)
	Cand    string `json:"cand"`     // a differing tier awaiting confirmation
	Count   int    `json:"count"`    // consecutive samples supporting Cand
	Nudged  string `json:"nudged"`   // the tier the last nudge announced
	NudgeAt int64  `json:"nudge_at"` // when that nudge fired (unix seconds)
}

// tierRank orders the tiers for the escalation exemption. Unknown/"" (fine) is 0.
func tierRank(tier string) int {
	switch tier {
	case "amber":
		return 1
	case "red":
		return 2
	default:
		return 0
	}
}

// tierStep is the pure transition. obs is the tier observed this sample ("" = fine);
// confirm is how many consecutive samples commit a change; minRestate is the quiet
// period in seconds. It returns the next state and whether to nudge NOW.
func tierStep(s tierState, obs string, now int64, confirm int, minRestate int64) (tierState, bool) {
	if obs == s.Tier {
		s.Cand, s.Count = "", 0 // the machine came back to where we already are
		return s, false
	}
	if obs == s.Cand {
		s.Count++
	} else {
		s.Cand, s.Count = obs, 1
	}
	if s.Count < confirm {
		return s, false // not yet believed
	}
	s.Tier, s.Cand, s.Count = obs, "", 0
	if obs == "" {
		return s, false // recovered: commit the improvement, say nothing
	}
	// Escalation is never suppressed; anything else waits out the quiet period.
	if tierRank(obs) <= tierRank(s.Nudged) && now-s.NudgeAt < minRestate {
		return s, false
	}
	s.Nudged, s.NudgeAt = obs, now
	return s, true
}

// tierGatePath is where a gated alert's state lives.
func tierGatePath(name string) string { return filepath.Join(state.Dir(), name+"-gate") }

// tierGate runs one sample of the named alert through tierStep, persisting the state.
// It reports whether to nudge — the single-writer serve tick is the only caller, so
// the read-modify-write has no race (the same reason the by-tier dedup lives there).
func tierGate(name, obs string, now int64, confirm int, minRestate int64) bool {
	var s tierState
	_ = json.Unmarshal([]byte(state.ReadMarker(tierGatePath(name))), &s)
	next, nudge := tierStep(s, obs, now, confirm, minRestate)
	if b, err := json.Marshal(next); err == nil {
		_ = state.WriteMarker(tierGatePath(name), string(b))
	}
	return nudge
}
