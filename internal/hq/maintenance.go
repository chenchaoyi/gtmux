// HQ periodic MAINTENANCE — the shared raise path behind the `distill` and `self-check`
// sensors, and the staleness verdict `gtmux doctor` reports.
//
// Both sensors used to end in `hqsurface.EmitControl`, which writes the trigger to the feed
// SPOOL and nowhere else. Neither reader of the spool could receive it: the seeded
// playbook tells HQ it does NOT need to tail the feed (arousal is the wake line), and
// `gtmux events` reads the journal, never the spool. So the triggers fired on schedule for
// weeks — the live spool holds the records — while zero passes ran and nothing in the
// event stream showed they had ever happened.
//
// The fix is one path with both halves. AUDIT: append to the JOURNAL, so `gtmux events`
// carries it and the feed daemon spools it on its normal tail (one record, not two — a
// hand-written spool copy on top of the journal append would double it). ARRIVAL: deliver
// the wake line on the same acked, draft-guarded channel every other class uses, at
// standing priority so it never preempts a blocked agent.
package hq

import (
	"strconv"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// raiseMaintenance records a due maintenance pass and knocks. `class` is the wake class,
// `control` the `gtmux:*` journal event name, `summary` the human reason (it is both the
// stream record's summary and the wake line's payload), and `hint` the trailing wake field
// telling HQ what to do. The caller has already gated on a live HQ pane and on its own
// cadence; this only delivers.
func raiseMaintenance(pane, class, control, reason, summary, hint string, sev string, now int64) {
	events.Append(events.Record{
		Ts: now, Event: control, Summary: reason + " — " + summary, Severity: sev,
	})
	hqnudge.Deliver(pane, hqwake.Line(class, reason, summary, hint))
}

// Maintenance staleness thresholds (seconds). The FLOOR is the sensor's own cadence; the
// GRACE on top absorbs the legitimate reasons a pass slips a little — the zero-change gate
// skipping a quiet period, a Mac that was asleep, serve restarting. Past floor+grace,
// something is actually wrong (serve down, no HQ home resolvable, a wedged sensor) and
// doctor says so.
const (
	distillGraceSecs   = 2 * 24 * 60 * 60 // 2 days on top of the weekly floor
	selfCheckGraceSecs = 12 * 60 * 60     // 12 hours on top of the daily floor
)

// MaintenanceState is a maintenance pass's staleness verdict.
type MaintenanceState int

const (
	// MaintenanceNever — no pass has ever been raised. Neutral, not a failure: a fresh
	// install, or an HQ that has simply not reached its first floor yet.
	MaintenanceNever MaintenanceState = iota
	// MaintenanceOK — the last pass is within its floor.
	MaintenanceOK
	// MaintenanceDue — past the floor but inside the grace window. Expected on a quiet
	// fleet (the zero-change gate), so it reads as a note, not a warning.
	MaintenanceDue
	// MaintenanceSlipped — past floor+grace. The cadence is NOT running.
	MaintenanceSlipped
)

// maintenanceState is the pure verdict (no clock, no disk — the testable core).
func maintenanceState(now, lastAt, floor, grace int64) MaintenanceState {
	if lastAt <= 0 {
		return MaintenanceNever
	}
	age := now - lastAt
	switch {
	case age <= floor:
		return MaintenanceOK
	case age <= floor+grace:
		return MaintenanceDue
	default:
		return MaintenanceSlipped
	}
}

// MaintenanceRow is one pass's reported status (consumed by `gtmux doctor`).
type MaintenanceRow struct {
	LastAt int64            // unix seconds of the last raised pass (0 = never)
	AgeSec int64            // seconds since it, meaningless when LastAt == 0
	Floor  int64            // the cadence floor it is judged against
	State  MaintenanceState // the verdict
}

// HumanAgeShort renders an age in seconds as a terse "3d" / "40h" / "12m" / "just now".
// Maintenance ages span minutes to weeks, so a single coarse unit is the readable choice —
// but hours run to 48, not 24: the daily self-check's interesting window is "a bit over a
// day", and rounding 40h down to "1d" made a flagged row read as if it were on time.
func HumanAgeShort(secs int64) string {
	switch {
	case secs < 60:
		return i18n.Tr("just now", "刚刚")
	case secs < 3600:
		return strconv.FormatInt(secs/60, 10) + "m"
	case secs < 48*3600:
		return strconv.FormatInt(secs/3600, 10) + "h"
	default:
		return strconv.FormatInt(secs/(24*3600), 10) + "d"
	}
}

// MaintenanceStatus reports the distill + self-check cadences at `now` — the read side of
// "is the periodic ritual actually happening?". It is pure disk reads (marker files), so
// it is safe on any host with no tmux and no live HQ.
func MaintenanceStatus(now int64) (distill, selfCheck MaintenanceRow) {
	dAt, _ := readDistillMark()
	sAt := readSelfCheckAt()
	return MaintenanceRow{
			LastAt: dAt, AgeSec: now - dAt, Floor: distillWeeklyFloor,
			State: maintenanceState(now, dAt, distillWeeklyFloor, distillGraceSecs),
		}, MaintenanceRow{
			LastAt: sAt, AgeSec: now - sAt, Floor: selfCheckDailyFloor,
			State: maintenanceState(now, sAt, selfCheckDailyFloor, selfCheckGraceSecs),
		}
}
