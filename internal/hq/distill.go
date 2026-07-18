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
// the rate limit: the ZERO-CHANGE gate (nothing notable accrued → never fire), then the
// VOLUME floor (a busy period — distil before the log rotates), then the WEEKLY floor.
//
//	notable  = notable-or-above events accrued since the watermark (the zero-change gate)
//	newCount = ALL events accrued since the watermark (curSeq - lastSeq; rotation pressure)
func shouldDistill(now, lastAt int64, notable, newCount int) (bool, string) {
	if now-lastAt < distillMinInterval {
		return false, "" // rate limited
	}
	if notable == 0 {
		return false, "" // zero-change gate — a pure-routine (or empty) period has nothing to distil
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
	if hqpane.Find() == "" {
		return
	}
	lastAt, lastSeq := readDistillMark()
	if now-lastAt < distillMinInterval {
		return // cheap rate-limit gate — skip the event scan entirely
	}
	// The event DELTA since the watermark: how many accrued (rotation pressure) and how
	// many are notable-or-above (the zero-change gate). Mirrors recentAttentionEvent's
	// use of the stored severity.
	recs, _ := events.ReadSince(lastSeq)
	curSeq := lastSeq
	notable := 0
	for _, r := range recs {
		if r.Seq > curSeq {
			curSeq = r.Seq
		}
		if events.SeverityRank(r.Severity) >= events.SeverityRank(events.SevNotable) {
			notable++
		}
	}
	fire, reason := shouldDistill(now, lastAt, notable, len(recs))
	if !fire {
		return
	}
	writeDistillMark(now, curSeq)
	hqfeed.EmitControl(hqfeed.ControlDistill,
		"distill due ("+reason+") — distil the period's fleet activity into the KB (update-over-append) and prune stale; silent unless real curation",
		events.SevNotable, now)
}
