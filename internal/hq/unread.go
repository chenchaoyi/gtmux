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
	"fmt"
	"path/filepath"
	"sort"
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

// unreadBlinkPairSec bounds the liveness pairing that separates a pane-less process BLINK
// from a native session coming online. Measured on the live stream (hq-unread-noise audit,
// 2026-08-08): a 5 s and a 60 s window catch the identical 15 of 28 pane-less starts, so
// this constant sits on the flat part of the curve rather than being tuned to it.
const unreadBlinkPairSec = 10

// unreadMaxGroups bounds the knock line's composition. One delivery is capped at 800 chars
// across up to 8 lines, so the breakdown gets a small budget and says "+N more" past it.
const unreadMaxGroups = 3

// unreadGroup is one source's contribution to the debt (`%21 ×2`).
type unreadGroup struct {
	Key string
	N   int
}

// unreadTally is what a scan of the debt found: how much HQ owes, how far the scan reached,
// and — the diagnosability half — WHAT the debt is made of.
type unreadTally struct {
	N      int
	MaxSeq int64
	Groups []unreadGroup // most numerous first, then by key; deterministic
}

// unreadSourceOf names the source a debt record is attributed to in the knock line. The
// three tokens are gtmux vocabulary (like the class names), not prose — they are not
// translated, so the line reads identically to an HQ in either language.
func unreadSourceOf(r events.Record) string {
	switch {
	case strings.HasPrefix(r.Event, "gtmux:"):
		return "control" // gtmux's own maintenance triggers
	case r.Pane != "":
		return r.Pane
	default:
		return "native" // a non-tmux agent session — real work, just not in a pane
	}
}

// unreadBlinks marks the pane-less lifecycle BLINKS in a scanned delta: a `SessionStart`
// with no pane whose matching pane-less `SessionEnd` from the same agent lands within the
// pairing window, plus that end. A short-lived child process or subagent whose hook fires
// without a pane is not something HQ can act on or attribute, so it is not a read HQ owes.
//
// The pairing — not the empty pane — is the whole criterion, and that is the load-bearing
// part. An empty pane is NOT a blink signature: it is carried just as much by every native
// (non-tmux) agent's turns and by gtmux's own `gtmux:*` control records, and the `session`
// field cannot tell them apart because it holds the tmux session name and is therefore
// empty on every pane-less record by construction (hook.go: session is read only when a
// pane exists). Excluding by empty pane would have stopped counting 39 real agent turns and
// the 9 maintenance triggers #647 shipped precisely so they would reach HQ — and those are
// the records that can LEAST afford it, because the class-wake channel fires only for a
// pane (hook.go: `if pane != ""`), which makes this knock their only channel at all.
func unreadBlinks(recs []events.Record) []bool {
	blink := make([]bool, len(recs))
	var ends []int
	for i, r := range recs {
		if r.Pane == "" && r.Event == "SessionEnd" {
			ends = append(ends, i)
		}
	}
	used := make([]bool, len(ends))
	for i, s := range recs {
		if s.Pane != "" || s.Event != "SessionStart" {
			continue
		}
		for j, ei := range ends {
			if used[j] {
				continue
			}
			if e := recs[ei]; e.Agent == s.Agent && e.Ts >= s.Ts && e.Ts-s.Ts <= unreadBlinkPairSec {
				used[j], blink[i], blink[ei] = true, true, true
				break
			}
		}
	}
	return blink
}

// unreadScan counts the events past the watermark that HQ actually has to consume, tallies
// what they are, and reports the highest sequence it scanned.
//
// It excludes the HQ pane's OWN records, and that exclusion is load-bearing rather than
// cosmetic: every knock is typed into the HQ pane, so it lands back in the stream as a
// UserPromptSubmit, and HQ's reply lands as a Stop. Counting those would make the sensor
// self-feeding — knock → two new events → debt → knock — a perpetual-motion machine that
// would knock forever on a fleet where nothing whatsoever happened. It also excludes
// pane-less blinks (see unreadBlinks). Everything else counts, including gtmux's own
// control records: a maintenance trigger IS something HQ owes a pass on, and it is rare
// enough (≈1/day) to never be the loop's fuel.
func unreadScan(watermark int64, hqPane string) unreadTally {
	recs, _ := events.ReadSince(watermark)
	blink := unreadBlinks(recs)
	t := unreadTally{MaxSeq: watermark}
	by := map[string]int{}
	for i, r := range recs {
		if r.Seq > t.MaxSeq {
			t.MaxSeq = r.Seq
		}
		if hqPane != "" && r.Pane == hqPane {
			continue // HQ's own echo — it wrote it, it does not need to read it
		}
		if blink[i] {
			continue
		}
		t.N++
		by[unreadSourceOf(r)]++
	}
	for k, n := range by {
		t.Groups = append(t.Groups, unreadGroup{Key: k, N: n})
	}
	sort.Slice(t.Groups, func(i, j int) bool {
		if t.Groups[i].N != t.Groups[j].N {
			return t.Groups[i].N > t.Groups[j].N
		}
		return t.Groups[i].Key < t.Groups[j].Key
	})
	return t
}

// composition renders the tally as ` (%21 ×2 · control ×1)` for the knock line — the
// diagnosability half of this sensor. A bare "1 unconsumed" cost HQ four rounds and a
// manual stream read to see the shape of a suspected feedback loop on 2026-08-04; naming
// the sources makes an echo-dominated or single-source accumulation visible from the
// delivered line itself, and often answers the question without a pull at all.
func (t unreadTally) composition() string {
	if len(t.Groups) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(" (")
	for i, g := range t.Groups {
		if i >= unreadMaxGroups {
			fmt.Fprintf(&b, " · +%d more", len(t.Groups)-i)
			break
		}
		if i > 0 {
			b.WriteString(" · ")
		}
		b.WriteString(g.Key)
		if g.N > 1 {
			fmt.Fprintf(&b, " ×%d", g.N)
		}
	}
	b.WriteString(")")
	return b.String()
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
	t := unreadScan(wm, pane)
	if t.N == 0 {
		// Everything past the watermark is HQ's own echo or a pane-less blink. Nothing to
		// say — and step the watermark over it, so this scan is not repeated every interval
		// for the rest of the day on a fleet that is genuinely quiet.
		hqwake.Consume(t.MaxSeq)
		return
	}
	hqnudge.Deliver(pane, hqwake.Line(hqwake.ClassUnread,
		strconv.Itoa(t.N)+i18n.Tr(" unconsumed", " 条未消费")+t.composition(),
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
	n := unreadScan(wm, hqpane.Find()).N
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
