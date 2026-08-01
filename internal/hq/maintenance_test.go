package hq

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqfeed"
)

func TestMaintenanceState(t *testing.T) {
	const now = 10_000_000
	const floor, grace = 7 * 24 * 3600, 2 * 24 * 3600

	cases := []struct {
		name   string
		lastAt int64
		want   MaintenanceState
	}{
		{"never run", 0, MaintenanceNever},
		{"just ran", now - 60, MaintenanceOK},
		{"exactly at the floor is still OK", now - floor, MaintenanceOK},
		{"one second past the floor is due, not slipped", now - floor - 1, MaintenanceDue},
		{"inside the grace window is due", now - floor - grace, MaintenanceDue},
		{"past floor+grace has SLIPPED", now - floor - grace - 1, MaintenanceSlipped},
		{"weeks late has slipped", now - 30*24*3600, MaintenanceSlipped},
	}
	for _, c := range cases {
		if got := maintenanceState(now, c.lastAt, floor, grace); got != c.want {
			t.Errorf("%s: maintenanceState = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestMaintenanceStatusReadsMarkers pins the read side doctor + `gtmux capture --list`
// depend on: the verdict comes from the sensors' own marker files, so a stalled cadence is
// visible on any host without a live HQ or tmux.
func TestMaintenanceStatusReadsMarkers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const now = 10_000_000

	// Nothing written yet → both never-run (a fresh install is not a failure).
	d, s := MaintenanceStatus(now)
	if d.State != MaintenanceNever || s.State != MaintenanceNever {
		t.Fatalf("fresh install: got distill=%v selfcheck=%v, want never/never", d.State, s.State)
	}

	writeDistillMark(now-3*24*3600, 42) // 3 days ago → inside the weekly floor
	writeSelfCheckAt(now - 40*3600)     // 40h ago → past daily+12h grace
	d, s = MaintenanceStatus(now)
	if d.State != MaintenanceOK {
		t.Errorf("distill 3d ago: got %v, want OK", d.State)
	}
	if d.AgeSec != 3*24*3600 {
		t.Errorf("distill age = %d, want %d", d.AgeSec, 3*24*3600)
	}
	if s.State != MaintenanceSlipped {
		t.Errorf("self-check 40h ago: got %v, want slipped", s.State)
	}
}

func TestHumanAgeShort(t *testing.T) {
	for _, c := range []struct {
		secs int64
		want string
	}{{0, "just now"}, {59, "just now"}, {60, "1m"}, {3599, "59m"}, {3600, "1h"},
		// Hours run to 48 so a self-check flagged at 40h doesn't read as an on-time "1d".
		{25 * 3600, "25h"}, {40 * 3600, "40h"}, {48 * 3600, "2d"}, {8 * 24 * 3600, "8d"}} {
		if got := HumanAgeShort(c.secs); got != c.want {
			t.Errorf("HumanAgeShort(%d) = %q, want %q", c.secs, got, c.want)
		}
	}
}

// TestRaiseMaintenanceIsAuditable is the regression for the whole bug: a raised trigger
// must land in the session-event JOURNAL, because that is the only stream `gtmux events`
// reads. For 13 days these records existed solely in the feed spool, so the periodic
// ritual looked like it had never run — and, with nothing knocking either, it hadn't.
func TestRaiseMaintenanceIsAuditable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const now = 10_000_000

	// pane "" makes the wake half a no-op (hqnudge.Deliver returns early), so the AUDIT
	// half is exercised on its own, without a terminal. Production always passes a
	// resolved pane — both sensors gate on hqpane.Find() first.
	raiseMaintenance("", "distill", hqfeed.ControlDistill, "due (weekly)",
		"distil the period into the KB", "then: gtmux capture --list", events.SevNotable, now)

	recs := events.Read(0, now+1)
	if len(recs) != 1 {
		t.Fatalf("want exactly 1 journal record, got %d", len(recs))
	}
	r := recs[0]
	if r.Event != hqfeed.ControlDistill {
		t.Errorf("event = %q, want %q", r.Event, hqfeed.ControlDistill)
	}
	if !events.IsControl(r) {
		t.Error("a maintenance record must classify as a control record")
	}
	if r.Seq == 0 {
		t.Error("record must carry a sequence so an HQ --since-seq delta picks it up")
	}
	if !strings.Contains(r.Summary, "weekly") {
		t.Errorf("summary %q should carry the reason", r.Summary)
	}
	// And it must be legible in `gtmux events` — the point of the audit trail.
	if line := events.Format(r); !strings.Contains(line, "CONTROL") ||
		!strings.Contains(line, hqfeed.ControlDistill) {
		t.Errorf("formatted line %q must name the control record", line)
	}

	// The daemon spools the journal on its own tail, so raise must NOT also hand-write a
	// spool copy — that would double every trigger.
	if got := len(hqfeed.ReadSpool(0, now+1)); got != 0 {
		t.Errorf("raise wrote %d spool records directly; the daemon's tail owns that", got)
	}
}

// TestControlRecordsDoNotFeedTheSensors pins the self-feed guard. Both sensors measure a
// delta that now contains their OWN records; counting them would make distill's
// zero-change gate permanently satisfied and would make every self-check suppress the
// next one's idle trigger.
func TestControlRecordsDoNotFeedTheSensors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const now = 10_000_000

	// A quiet fleet whose ONLY journal traffic is gtmux's own maintenance records.
	events.Append(events.Record{Ts: now - 600, Event: hqfeed.ControlSelfCheck,
		Summary: "self-check due (idle)", Severity: events.SevNotable})
	events.Append(events.Record{Ts: now - 300, Event: hqfeed.ControlDistill,
		Summary: "distill due (weekly)", Severity: events.SevNotable})

	if recentAttentionEvent(now) {
		t.Error("gtmux's own control records must not read as recent user attention")
	}

	// A real fleet event DOES count — the guard must not blind the sensor entirely.
	events.Append(events.Record{Ts: now - 60, Event: "Stop", State: "idle", Pane: "%1",
		Severity: events.SevNotable})
	if !recentAttentionEvent(now) {
		t.Error("a genuine notable fleet event must still count as recent attention")
	}
}

// TestCaptureListHeaderReportsDrainAge pins the observability line: an empty queue alone
// cannot distinguish "drained yesterday" from "never drained".
func TestCaptureListHeaderReportsDrainAge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const now = 10_000_000

	if got := captureListHeader(now); !strings.Contains(got, "never") {
		t.Errorf("no marker yet: header = %q, want a never-run note", got)
	}
	writeDistillMark(now-2*24*3600, 1)
	if got := captureListHeader(now); !strings.Contains(got, "2d") || strings.Contains(got, "SLIPPED") {
		t.Errorf("2d ago: header = %q, want a plain 2d age", got)
	}
	writeDistillMark(now-30*24*3600, 1)
	if got := captureListHeader(now); !strings.Contains(got, "SLIPPED") {
		t.Errorf("30d ago: header = %q, want a SLIPPED verdict", got)
	}
}

// The sensors are gated on a live supervisor: with none resolvable they must be a
// COMPLETE no-op — no journal record and, critically, no advanced watermark. A sensor that
// burned its watermark with no HQ to tell would skip the pass entirely once one appeared.
// (A temp HOME makes state.HQHome() unmatchable by any real pane, so this holds whether or
// not the host is running tmux.)
func TestSensorsNoOpWithoutASupervisor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	distillSensor(now)   // long past every floor (no marker = never distilled)
	selfCheckSensor(now) // likewise

	if at, seq := readDistillMark(); at != 0 || seq != 0 {
		t.Errorf("distill watermark advanced with no HQ: (%d,%d)", at, seq)
	}
	if at := readSelfCheckAt(); at != 0 {
		t.Errorf("self-check marker advanced with no HQ: %d", at)
	}
	if recs := events.Read(0, now+1); len(recs) != 0 {
		t.Errorf("raised %d records with no HQ to raise them to", len(recs))
	}
}

// TestDistillMarkRoundTrip guards the watermark format both halves of the sensor share.
func TestDistillMarkRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeDistillMark(1_700_000_000, 4321)
	at, seq := readDistillMark()
	if at != 1_700_000_000 || seq != 4321 {
		t.Fatalf("readDistillMark = (%d,%d), want (1700000000,4321)", at, seq)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".local", "share", "gtmux", "hq-feed", "last-distill"))
	if err != nil {
		t.Fatal(err)
	}
	if fields := strings.Fields(string(raw)); len(fields) != 2 ||
		fields[1] != strconv.Itoa(4321) {
		t.Errorf("marker on disk = %q, want '<at> <seq>'", string(raw))
	}
}
