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
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// A sensor marker records a request, not work completed by HQ. A separate receipt
// references that request so a newer request cannot be mistaken for an older pass.
func maintenanceReceiptPath(kind string) string {
	return filepath.Join(state.Dir(), "hq-feed", "last-"+kind+"-complete")
}

// The version marker keeps old pre-receipt trigger files eligible for a fresh wake
// after upgrade. New requests wait for their receipt instead of advancing the
// distill sequence again and silently dropping an unprocessed event range.
func maintenanceRequestVersionPath(kind string) string {
	return filepath.Join(state.Dir(), "hq-feed", "last-"+kind+"-request-v2")
}

func maintenanceRequestPending(kind string, requestAt int64) bool {
	if requestAt == 0 || !state.Exists(maintenanceRequestVersionPath(kind)) {
		return false
	}
	completedAt, completedRequest := readMaintenanceReceipt(kind)
	return completedAt <= 0 || completedRequest != requestAt
}

func readMaintenanceReceipt(kind string) (completedAt, requestAt int64) {
	parts := strings.Fields(state.ReadMarker(maintenanceReceiptPath(kind)))
	if len(parts) == 2 {
		completedAt, _ = strconv.ParseInt(parts[0], 10, 64)
		requestAt, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	return
}

// completeMaintenance is called by HQ after the pass, from its home directory.
// Replaying the same receipt is harmless; the journal has one completion per request.
func completeMaintenance(kind string, now int64) error {
	if kind != "distill" && kind != "self-check" {
		return errors.New("expected distill or self-check")
	}
	if !fromHQHome() {
		return errors.New("run from the HQ home")
	}
	requestAt := readSelfCheckAt()
	if kind == "distill" {
		requestAt, _ = readDistillMark()
		if n := knowledge.PendingCandidateCount(); n != 0 {
			return fmt.Errorf("%d capture candidates still pending", n)
		}
	}
	if requestAt == 0 {
		return errors.New("no maintenance request to complete")
	}
	completedAt, previousRequest := readMaintenanceReceipt(kind)
	if completedAt > 0 && previousRequest == requestAt {
		return nil
	}
	if err := state.WriteMarker(maintenanceReceiptPath(kind),
		strconv.FormatInt(now, 10)+" "+strconv.FormatInt(requestAt, 10)); err != nil {
		return err
	}
	events.Append(events.Record{Ts: now, Event: "gtmux:maintenance-completed", Kind: kind,
		RequestAt: requestAt, Severity: events.SevRoutine})
	diag.Did("act.hq.maintenance", kind, diag.OK, "HQ completed maintenance",
		"request_at", requestAt)
	return nil
}

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
	LastAt      int64            // unix seconds of the last raised pass (0 = never)
	AgeSec      int64            // since request while pending, otherwise since completion
	Floor       int64            // the cadence floor it is judged against
	State       MaintenanceState // the verdict
	CompletedAt int64            // unix seconds of the latest acknowledged pass
	Pending     bool             // the latest request has no completion receipt
}

// MaintenanceStatus reports the distill + self-check cadences at `now` — the read side of
// "is the periodic ritual actually happening?". It is pure disk reads (marker files), so
// it is safe on any host with no tmux and no live HQ.
func MaintenanceStatus(now int64) (distill, selfCheck MaintenanceRow) {
	dAt, _ := readDistillMark()
	sAt := readSelfCheckAt()
	dCompleted, dRequest := readMaintenanceReceipt("distill")
	sCompleted, sRequest := readMaintenanceReceipt("self-check")
	dPending := dAt > 0 && (dCompleted <= 0 || dRequest != dAt)
	sPending := sAt > 0 && (sCompleted <= 0 || sRequest != sAt)
	dAgeAt, sAgeAt := dAt, sAt
	if !dPending && dCompleted > 0 {
		dAgeAt = dCompleted
	}
	if !sPending && sCompleted > 0 {
		sAgeAt = sCompleted
	}
	return MaintenanceRow{
		LastAt: dAt, AgeSec: now - dAgeAt, Floor: distillWeeklyFloor,
		State:       maintenanceState(now, dAgeAt, distillWeeklyFloor, distillGraceSecs),
		CompletedAt: dCompleted, Pending: dPending,
	}, MaintenanceRow{
		LastAt: sAt, AgeSec: now - sAgeAt, Floor: selfCheckDailyFloor,
		State:       maintenanceState(now, sAgeAt, selfCheckDailyFloor, selfCheckGraceSecs),
		CompletedAt: sCompleted, Pending: sPending,
	}
}
