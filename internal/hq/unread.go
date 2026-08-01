// The UNCONSUMED-EVENTS sensor: HQ perception's completeness net (hq-watermark-wakes).
//
// Every other wake class answers "is THIS event worth a knock?" — a question gtmux cannot
// actually answer, because the context that decides it (what HQ is currently waiting on)
// lives only in HQ. So the wake vocabulary has always been a whitelist of guesses, and
// every scenario it failed to anticipate disappeared in silence: on 2026-08-01 a pane
// finished an install the commander was waiting on, the turn-end landed in the stream
// complete and notable (seq 6665), and nothing knocked — the completion belonged to no
// tracked dispatch (so not `done`) and asked nothing (so not `asks`). Three minutes later
// the commander asked by hand. That failure mode had already been patched five times, each
// time by adding one more entry to the whitelist.
//
// This sensor asks a different question, one that needs no context: has the stream moved
// past HQ's consumption watermark? If it has, and stayed there for the aggregation window,
// gtmux knocks with a COUNT — no severity claim, no classification — and HQ pulls and
// judges, which is the one place the judgment can be made correctly. The debt clears only
// when HQ consumes; until then the knock repeats. Nothing here can have a blind spot,
// because nothing here inspects the events at all.
package hq

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// unreadStatePath holds the sensor's own memory: "<watermark> <since> <checkedAt>".
// Single-writer (the serve slow tick), so it needs no locking.
func unreadStatePath() string { return filepath.Join(state.Dir(), "hqwake", "unread-state") }

// unreadState is the standing-debt window the sensor tracks.
type unreadState struct {
	// Watermark is the consumption position this window is about. When HQ consumes, the
	// watermark moves and the window RESTARTS — a partial catch-up earns a fresh
	// aggregation window rather than an immediate second knock.
	Watermark int64
	// Since is when the debt against that watermark was first observed (0 = no debt).
	Since int64
	// CheckedAt is when the sensor last EVALUATED the debt (counted, and knocked if there
	// was anything to knock about). It paces both the re-knock and the only expensive
	// operation here, the log scan.
	CheckedAt int64
}

func readUnreadState() unreadState {
	f := strings.Fields(state.ReadMarker(unreadStatePath()))
	var st unreadState
	if len(f) >= 3 {
		st.Watermark, _ = strconv.ParseInt(f[0], 10, 64)
		st.Since, _ = strconv.ParseInt(f[1], 10, 64)
		st.CheckedAt, _ = strconv.ParseInt(f[2], 10, 64)
	}
	return st
}

func writeUnreadState(st unreadState) {
	_ = state.WriteMarker(unreadStatePath(), strconv.FormatInt(st.Watermark, 10)+" "+
		strconv.FormatInt(st.Since, 10)+" "+strconv.FormatInt(st.CheckedAt, 10))
}

// unreadVerdict is unreadDecide's outcome for one tick.
type unreadVerdict int

const (
	unreadCaughtUp unreadVerdict = iota // the watermark is at the stream end — nothing owed
	unreadHold                          // owed, but inside the aggregation window or repeat interval
	unreadEvaluate                      // owed long enough: count it and knock
)

// unreadDecide is the PURE tick decision (no clock, no disk, no tmux — the testable core),
// returning the verdict and the state to persist.
//
//	latest    — the end of the event stream (last sequence assigned)
//	watermark — how far HQ has consumed
//	debounce  — how long a debt must stand before the first knock
//	repeat    — how often to re-knock while the watermark does not move
func unreadDecide(now, latest, watermark int64, st unreadState, debounce, repeat int64) (unreadVerdict, unreadState) {
	if latest <= watermark {
		return unreadCaughtUp, unreadState{} // caught up → forget the window entirely
	}
	if st.Watermark != watermark || st.Since == 0 {
		// Either the first sighting of this debt, or HQ consumed something since the last
		// tick. Both start the aggregation window fresh: a knock is a request to pull, and
		// HQ that just pulled deserves the full window before being asked again.
		return unreadHold, unreadState{Watermark: watermark, Since: now}
	}
	if now-st.Since < debounce {
		return unreadHold, st // still coalescing — this is what keeps a burst to one line
	}
	if st.CheckedAt != 0 && now-st.CheckedAt < repeat {
		return unreadHold, st
	}
	st.CheckedAt = now
	return unreadEvaluate, st
}

// unreadCount counts the events past the watermark that HQ actually has to consume, and
// returns them with the highest sequence it scanned.
//
// It excludes the HQ pane's OWN records, and that exclusion is load-bearing rather than
// cosmetic: every knock is typed into the HQ pane, so it lands back in the stream as a
// UserPromptSubmit, and HQ's reply lands as a Stop. Counting those would make the sensor
// self-feeding — knock → two new events → debt → knock — a perpetual-motion machine that
// would knock forever on a fleet where nothing whatsoever happened. Everything else counts,
// including gtmux's own control records: a maintenance trigger IS something HQ owes a pass
// on, and it is rare enough (≈1/day) to never be the loop's fuel.
func unreadCount(watermark int64, hqPane string) (n int, maxSeq int64) {
	recs, _ := events.ReadSince(watermark)
	maxSeq = watermark
	for _, r := range recs {
		if r.Seq > maxSeq {
			maxSeq = r.Seq
		}
		if hqPane != "" && r.Pane == hqPane {
			continue // HQ's own echo — it wrote it, it does not need to read it
		}
		n++
	}
	return n, maxSeq
}

// unreadSensor is the slow-tick entry point. Cheap by construction: the common case (HQ
// caught up) is one small file read and nothing else, and the log scan happens at most
// once per repeat interval, only while a debt is actually standing.
func unreadSensor(now int64) {
	pane := hqpane.Find()
	if pane == "" {
		// No supervisor: nothing to wake — and, critically, the watermark must NOT move.
		// A watermark advanced with no HQ to tell would mean an HQ that came online right
		// after was declared caught up on events it had never seen, which is precisely the
		// silent loss this whole mechanism exists to make impossible.
		return
	}
	unreadSensorFor(pane, now)
}

// unreadSensorFor is the sensor body for an already-resolved HQ pane (the seam that lets
// the guarantee be tested end-to-end, queue and all, without a tmux server).
func unreadSensorFor(pane string, now int64) {
	latest := events.CurrentSeq()
	wm := hqwake.Consumed()
	if wm == hqwake.WatermarkUnset {
		// Bootstrap: adopt the current stream end. An install upgrading into this mechanism
		// has a full history and no cursor; without this its first knock would claim
		// thousands of unread events, all of them long since dealt with or irrelevant.
		hqwake.Consume(latest)
		return
	}
	cfg := hqwake.Load()
	v, next := unreadDecide(now, latest, wm, readUnreadState(),
		cfg.UnreadDebounceSec, cfg.UnreadRepeatSec)
	if v == unreadEvaluate && hqnudge.Pending() {
		// A knock is already queued and has not landed (a half-typed draft, a pane in
		// copy-mode). Adding a second line tells HQ nothing new — it will pull when the
		// first one lands. State is deliberately left untouched, so this fires immediately
		// once the queue clears rather than waiting out another repeat interval.
		return
	}
	writeUnreadState(next)
	if v != unreadEvaluate {
		return
	}
	n, maxSeq := unreadCount(wm, pane)
	if n == 0 {
		// Everything past the watermark is HQ's own echo. Nothing to say — and step the
		// watermark over it, so this scan is not repeated every interval for the rest of
		// the day on a fleet that is genuinely quiet.
		hqwake.Consume(maxSeq)
		return
	}
	hqnudge.Deliver(pane, hqwake.Line(hqwake.ClassUnread,
		strconv.Itoa(n)+i18n.Tr(" unconsumed", " 条未消费"),
		"pull: gtmux events --since-seq "+strconv.FormatInt(wm, 10)+" --json"))
}

// ── consumption status (the read side `gtmux doctor` reports) ────────────────

// Consumption-lag thresholds. Either one trips the doctor row: a big backlog is a
// problem even if it appeared a minute ago, and a small one that has stood for half an
// hour means the channel — not the volume — is broken.
const (
	consumptionLagCount = 20      // events behind
	consumptionLagSecs  = 30 * 60 // or standing this long
)

// ConsumptionRow is HQ's event-consumption verdict (consumed by `gtmux doctor`).
type ConsumptionRow struct {
	Unread      int              // events past the watermark HQ still owes a read
	StandingSec int64            // how long the current debt has stood (0 = none)
	State       MaintenanceState // Never (no watermark yet) | OK | Slipped
}

// ConsumptionStatus reports whether HQ is actually keeping up with the stream — the
// observability half of the guarantee. Without it, a wake channel that stops landing is
// invisible again: every knock is silent by design, so "HQ consumed nothing for two hours"
// looks exactly like "nothing happened for two hours", and the only detector left is the
// commander noticing his question went unanswered. That is the hole this closes.
func ConsumptionStatus(now int64) ConsumptionRow {
	wm := hqwake.Consumed()
	if wm == hqwake.WatermarkUnset {
		return ConsumptionRow{State: MaintenanceNever}
	}
	n, _ := unreadCount(wm, hqpane.Find())
	if n == 0 {
		return ConsumptionRow{State: MaintenanceOK}
	}
	standing := int64(0)
	if st := readUnreadState(); st.Watermark == wm && st.Since > 0 {
		standing = now - st.Since
	}
	row := ConsumptionRow{Unread: n, StandingSec: standing, State: MaintenanceOK}
	if n >= consumptionLagCount || standing >= consumptionLagSecs {
		row.State = MaintenanceSlipped
	}
	return row
}
