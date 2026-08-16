// The HQ knowledge-distillation sensor (hq-knowledge-distillation): gtmux SENSES,
// LLM-free in the serve slow-tick, when HQ should run a periodic distillation pass and
// raises a `distill` control record into the feed. HQ (the LLM) does the actual
// curation — reading the fleet's event delta since the last distill, folding durable
// cross-cutting facts into the knowledge base and pruning stale. gtmux never runs the
// distillation itself (no LLM in the timing loop; the same split as the self-check
// sensor). It is the RETROSPECTIVE counterpart to the moment-of-learning capture: the
// watermark bounds each pass to the delta so it consolidates rather than duplicates.
package hq

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqfeed"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Distill timing — deliberately coarse. Cadence maps to the commander's "按天/按周":
// a quiet fleet distills WEEKLY; a busy one distills sooner (the volume floor) so its
// delta is distilled before the size-bounded event log rotates it away — but never
// more than once per rate interval (an at-most-daily cap).
const (
	distillMinInterval = 24 * 60 * 60     // rate limit: at most one distill per day
	distillWeeklyFloor = 7 * 24 * 60 * 60 // a weekly pass regardless (quiet fleet)
	// distillVolumeFloor is the new-event count that forces a distill before the weekly
	// floor. PROVISIONAL: sized to fire well inside one rotated generation of the
	// current 20 MB (~×2) event log; reconcile with the events-retention work (%84)
	// once its final retention sizing lands.
	distillVolumeFloor = 5000
	// distillSpoolFloor is the pending-candidate count that forces a distill before the
	// weekly floor (hq-capture-loop layer ③(c), default N=5). Without it a lesson someone
	// took the trouble to `gtmux capture` can sit unfiled for up to a week — long enough
	// for the next agent to hit the same footgun, which is the whole thing capture exists
	// to prevent. Still behind the ≤1/day rate limit.
	distillSpoolFloor = 5
)

func distillMarkPath() string { return filepath.Join(state.Dir(), "hq-feed", "last-distill") }

// readDistillMark returns the last distill's (unix time, event seq watermark). A
// missing/unparseable marker reads as (0, 0) — treated as "never distilled".
func readDistillMark() (at, seq int64) {
	fields := strings.Fields(state.ReadMarker(distillMarkPath()))
	if len(fields) >= 1 {
		at, _ = strconv.ParseInt(fields[0], 10, 64)
	}
	if len(fields) >= 2 {
		seq, _ = strconv.ParseInt(fields[1], 10, 64)
	}
	return at, seq
}

func writeDistillMark(at, seq int64) {
	_ = state.WriteMarker(distillMarkPath(), strconv.FormatInt(at, 10)+" "+strconv.FormatInt(seq, 10))
}

// shouldDistill is the pure decision (testable without tmux/disk). Precedence once past
// the rate limit: the ZERO-CHANGE gate (nothing accrued and nothing captured → never
// fire), then the SPOOL floor (someone captured a lesson — file it), then the VOLUME
// floor (a busy period — distil before the log rotates), then the WEEKLY floor.
//
//	notable  = notable-or-above FLEET events accrued since the watermark (gtmux's own
//	           control records excluded — see events.IsControl)
//	newCount = ALL fleet events accrued since the watermark (rotation pressure)
//	pending  = candidates sitting in the pending-distill spool
func shouldDistill(now, lastAt int64, notable, newCount, pending int) (bool, string) {
	if now-lastAt < distillMinInterval {
		return false, "" // rate limited
	}
	if notable == 0 && pending == 0 {
		return false, "" // zero-change gate — nothing happened and nothing was captured
	}
	if pending >= distillSpoolFloor {
		return true, "spool"
	}
	if newCount >= distillVolumeFloor {
		return true, "volume"
	}
	if now-lastAt >= distillWeeklyFloor {
		return true, "weekly"
	}
	return false, ""
}

// distillSensor raises a distill trigger to HQ when due. It runs from the serve
// slow-tick; only with a live HQ, and the expensive event scan runs only after the
// cheap rate-limit gate passes.
func distillSensor(now int64) {
	pane := hqpane.Find()
	if pane == "" {
		return
	}
	lastAt, lastSeq := readDistillMark()
	if now-lastAt < distillMinInterval {
		return // cheap rate-limit gate — skip the event scan entirely
	}
	// The event DELTA since the watermark: how many accrued (rotation pressure) and how
	// many are notable-or-above (the zero-change gate). Mirrors recentAttentionEvent's
	// use of the stored severity. gtmux's OWN control records are excluded from both
	// counts — the previous distill's record is not something to distil, and counting it
	// would keep the zero-change gate permanently satisfied on a dead-quiet fleet.
	recs, _ := events.ReadSince(lastSeq)
	curSeq := lastSeq
	notable, fleet := 0, 0
	for _, r := range recs {
		if r.Seq > curSeq {
			curSeq = r.Seq
		}
		if events.IsControl(r) {
			continue
		}
		fleet++
		if events.SeverityRank(r.Severity) >= events.SeverityRank(events.SevNotable) {
			notable++
		}
	}
	pending := pendingCandidateCount()
	fire, reason := shouldDistill(now, lastAt, notable, fleet, pending)
	if !fire {
		return
	}
	writeDistillMark(now, curSeq)
	hint := "then: gtmux capture --list"
	if pending == 0 {
		hint = "then: gtmux events --since-seq " + strconv.FormatInt(lastSeq, 10)
	}
	raiseMaintenance(pane, hqwake.ClassDistill, hqfeed.ControlDistill, "due ("+reason+")",
		"drain captures (knowledge add --capture / dismiss) + distil the period (add/supersede/retire --why); silent unless real curation",
		hint, events.SevNotable, now)
}
