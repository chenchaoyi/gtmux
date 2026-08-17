package hq

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqsurface"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// testHQPane is a pane id tmux can never assign (ids are `%<int>`), so a test that walks
// the real delivery path can enqueue and inspect a wake without any chance of typing into
// a live pane on the developer's own machine.
const testHQPane = "%unread-test"

// queuedWakes returns the wake lines sitting in the delivery queue, oldest first.
func queuedWakes(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join(state.Dir(), "hq-nudges")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	return out
}

func unreadKnocks(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, w := range queuedWakes(t) {
		if strings.Contains(w, "gtmux·"+hqwake.ClassUnread) {
			out = append(out, w)
		}
	}
	return out
}

// takeUnreadKnocks returns the unread knocks queued since the last call and empties the
// queue — standing in for the 3 s fast tick that types a queued batch into a live pane.
// Draining matters to what is under test: the sensor deliberately holds off while a knock
// is still waiting to land (a second line tells HQ nothing the first one didn't), so a test
// that never drains would be testing that guard rather than the repeat cadence.
func takeUnreadKnocks(t *testing.T) []string {
	t.Helper()
	got := unreadKnocks(t)
	dir := filepath.Join(state.Dir(), "hq-nudges")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".txt") {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	return got
}

// TestUnreadDecide pins the pure tick decision — the aggregation window that keeps a burst
// to one line, and the fact that the debt only ever clears by CONSUMPTION.
func TestUnreadDecide(t *testing.T) {
	const now = 10_000_000
	const debounce, repeat = 120, 300

	cases := []struct {
		name              string
		latest, watermark int64
		st                unreadState
		want              unreadVerdict
	}{
		{"caught up says nothing", 100, 100, unreadState{}, unreadCaughtUp},
		{"watermark ahead of the stream is still caught up", 100, 140, unreadState{}, unreadCaughtUp},
		{"first sighting starts the window, does not knock", 105, 100, unreadState{}, unreadHold},
		{"inside the aggregation window", 105, 100,
			unreadState{Watermark: 100, Since: now - debounce + 1}, unreadHold},
		{"past the window with no knock yet", 105, 100,
			unreadState{Watermark: 100, Since: now - debounce}, unreadEvaluate},
		{"a knocked debt holds until the repeat interval", 105, 100,
			unreadState{Watermark: 100, Since: now - 1000, CheckedAt: now - repeat + 1}, unreadHold},
		{"an unconsumed debt re-knocks at the repeat interval", 105, 100,
			unreadState{Watermark: 100, Since: now - 1000, CheckedAt: now - repeat}, unreadEvaluate},
		// A partial catch-up earns a fresh window rather than an immediate second knock.
		{"a moved watermark restarts the window", 105, 103,
			unreadState{Watermark: 100, Since: now - 1000, CheckedAt: now - 1000}, unreadHold},
	}
	for _, c := range cases {
		got, next := unreadDecide(now, c.latest, c.watermark, c.st, debounce, repeat)
		if got != c.want {
			t.Errorf("%s: verdict = %v, want %v", c.name, got, c.want)
		}
		if got == unreadCaughtUp && (next.Since != 0 || next.Watermark != 0) {
			t.Errorf("%s: caught up must forget the window, got %+v", c.name, next)
		}
		if got == unreadEvaluate && next.CheckedAt != now {
			t.Errorf("%s: an evaluated tick must stamp CheckedAt, got %+v", c.name, next)
		}
	}
}

// TestUnreadKnocksThenStopsOnConsumption is the whole mechanism in one pass, and the
// regression for the 2026-08-01 miss: a turn-end that belonged to no dispatch and asked
// nothing landed in the stream complete — and, because no wake class claimed it, nobody was
// told. Direction 1 (an unconsumed event knocks) and direction 2 (consumption stops it).
func TestUnreadKnocksThenStopsOnConsumption(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	// Bootstrap: an install adopting the watermark must NOT be told it is N events behind.
	events.Append(events.Record{Ts: now - 9999, Event: "Stop", State: "idle", Pane: "%1"})
	unreadSensorFor(testHQPane, now)
	if got := hqwake.Consumed(); got != events.CurrentSeq() {
		t.Fatalf("bootstrap watermark = %d, want the stream end %d", got, events.CurrentSeq())
	}
	if k := unreadKnocks(t); len(k) != 0 {
		t.Fatalf("bootstrap must not knock about history, got %v", k)
	}

	// The event nobody classifies: a completion in a session gtmux never dispatched to.
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%16",
		Loc: "gtmux:3.0", Summary: "installed", Severity: events.SevNotable})

	// Inside the aggregation window: still silent (a burst must not become a burst of lines).
	unreadSensorFor(testHQPane, now+1)
	if k := unreadKnocks(t); len(k) != 0 {
		t.Fatalf("knocked inside the aggregation window: %v", k)
	}

	// Past it: exactly one knock, carrying a count and the cursor to pull from.
	wm := hqwake.Consumed()
	unreadSensorFor(testHQPane, now+1+hqwake.Defaults().UnreadDebounceSec)
	knocks := takeUnreadKnocks(t)
	if len(knocks) != 1 {
		t.Fatalf("want exactly 1 unread knock, got %d: %v", len(knocks), knocks)
	}
	if !strings.Contains(knocks[0], "1") || !strings.Contains(knocks[0], "--since-seq") {
		t.Errorf("knock %q must carry the count and the pull cursor", knocks[0])
	}
	if hqwake.Consumed() != wm {
		t.Errorf("knocking must not advance the watermark — only HQ consuming does")
	}
	// It carries no severity claim: importance is HQ's judgment, made after the pull.
	for _, w := range []string{"important", "notable", "routine"} {
		if strings.Contains(knocks[0], w) {
			t.Errorf("knock %q must not classify importance (%q)", knocks[0], w)
		}
	}

	// CONSUMED: HQ pulls the delta. The debt clears and the knock stops — for good, not
	// just for one interval.
	hqwake.ConsumeRead(wm, events.CurrentSeq())
	for _, at := range []int64{
		now + hqwake.Defaults().UnreadDebounceSec + 1,
		now + hqwake.Defaults().UnreadDebounceSec + hqwake.Defaults().UnreadRepeatSec + 1,
		now + 100_000,
	} {
		unreadSensorFor(testHQPane, at)
	}
	if got := takeUnreadKnocks(t); len(got) != 0 {
		t.Errorf("after consumption the sensor kept knocking: %v", got)
	}
}

// TestUnreadRepeatsWhileUnconsumed is the guarantee's teeth: a knock that was delivered but
// NOT acted on is still a debt. Every previous fix in this area was a one-shot — the event
// got exactly one chance to be seen, and a swallowed or ignored line was the end of it.
func TestUnreadRepeatsWhileUnconsumed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	hqwake.Consume(0) // an initialized, caught-up HQ
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%16",
		Severity: events.SevNotable})

	d, r := hqwake.Defaults().UnreadDebounceSec, hqwake.Defaults().UnreadRepeatSec
	unreadSensorFor(testHQPane, now)   // start the window
	unreadSensorFor(testHQPane, now+d) // first knock
	if got := takeUnreadKnocks(t); len(got) != 1 {
		t.Fatalf("want 1 knock at the end of the aggregation window, got %d", len(got))
	}
	unreadSensorFor(testHQPane, now+d+1) // inside the repeat interval — silent
	if got := takeUnreadKnocks(t); len(got) != 0 {
		t.Fatalf("re-knocked inside the repeat interval: %v", got)
	}
	unreadSensorFor(testHQPane, now+d+r)
	if got := takeUnreadKnocks(t); len(got) != 1 {
		t.Errorf("an unconsumed debt must re-knock at the repeat interval, got %d knocks", len(got))
	}
}

// TestUnreadIgnoresHQsOwnEcho pins the anti-self-feed rule. Every knock is typed into the HQ
// pane, so it lands back in the stream as a submission, and HQ's reply lands as a turn-end.
// Counting those would make the sensor its own event source: knock → two events → debt →
// knock, forever, on a fleet where nothing happened at all.
func TestUnreadIgnoresHQsOwnEcho(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	hqwake.Consume(0)
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: testHQPane,
		Summary: "#a3f1c2"}) // the wake batch echoed back by the hook
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: testHQPane,
		Severity: events.SevNotable}) // HQ's own reply

	d := hqwake.Defaults().UnreadDebounceSec
	unreadSensorFor(testHQPane, now)
	unreadSensorFor(testHQPane, now+d)
	if k := unreadKnocks(t); len(k) != 0 {
		t.Fatalf("HQ's own records must not knock at HQ: %v", k)
	}
	// And the watermark steps over them, so the scan is not repeated forever.
	if got := hqwake.Consumed(); got != events.CurrentSeq() {
		t.Errorf("watermark = %d, want it stepped over HQ's own echo to %d", got, events.CurrentSeq())
	}

	// A real fleet event still knocks — the guard must not blind the sensor.
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%16",
		Severity: events.SevNotable})
	unreadSensorFor(testHQPane, now+d)
	unreadSensorFor(testHQPane, now+2*d)
	if k := unreadKnocks(t); len(k) != 1 {
		t.Errorf("a genuine fleet event must still knock, got %d knocks", len(k))
	}
}

// A control record is gtmux's own bookkeeping but it is NOT HQ's echo: a maintenance pass
// is something HQ owes work on, so it counts. (Its own sensors' self-feed guard is a
// different concern — see TestControlRecordsDoNotFeedTheSensors.)
func TestUnreadCountsControlRecords(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	hqwake.Consume(0)
	events.Append(events.Record{Ts: now, Event: hqsurface.ControlDistill,
		Summary: "due (weekly)", Severity: events.SevNotable})
	if n := unreadScan(0, testHQPane).N; n != 1 {
		t.Errorf("unread count = %d, want the maintenance record to count as owed work", n)
	}
}

// TestUnreadSkipsPaneLessBlinks is C7 (hq-unread-noise), pinned to the measured shape: two
// same-second SessionStart/SessionEnd pairs with no pane (the seq 8472-8475 form), from a
// short-lived subprocess whose hook fired without a pane. HQ can neither act on nor
// attribute them, so they are not a read it owes — but they stay in the stream, because
// every other consumer (the tick summary, a manual read) still has business with them.
func TestUnreadSkipsPaneLessBlinks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	hqwake.Consume(0)
	for i := 0; i < 2; i++ {
		events.Append(events.Record{Ts: now, Event: "SessionStart", Agent: "Claude Code",
			Severity: events.SevNotable})
		events.Append(events.Record{Ts: now, Event: "SessionEnd", Agent: "Claude Code",
			Severity: events.SevNotable})
	}
	if n := unreadScan(0, testHQPane).N; n != 0 {
		t.Errorf("unread count = %d, want 0 — a pane-less process blink is not owed work", n)
	}
	// The exclusion is from the tally ONLY. A record that vanished from the stream would
	// be a silent data loss dressed up as a noise fix.
	recs, _ := events.ReadSince(0)
	if len(recs) != 4 {
		t.Errorf("stream returned %d records, want all 4 blink records still readable", len(recs))
	}

	// And the debt genuinely clears rather than standing forever: with nothing owed, the
	// sensor steps the watermark over the blinks instead of rescanning them every interval.
	d := hqwake.Defaults().UnreadDebounceSec
	unreadSensorFor(testHQPane, now)
	unreadSensorFor(testHQPane, now+d)
	if k := unreadKnocks(t); len(k) != 0 {
		t.Fatalf("a pane-less blink knocked: %v", k)
	}
	if got := hqwake.Consumed(); got != events.CurrentSeq() {
		t.Errorf("watermark = %d, want it stepped over the blinks to %d", got, events.CurrentSeq())
	}
}

// TestUnreadCountsPaneLessWork is the anti-regression for the rule this change ALMOST
// shipped. The ledger's cheap criterion was "an empty pane is not countable debt"; measured
// against the live stream (audit-echo-2026-08-08), an empty pane is not a blink signature
// at all — it is carried by every native (non-tmux) agent's turns, by an unmatched
// SessionStart (a native session coming ONLINE — 13 of 28 of them), and by gtmux's own
// maintenance triggers. Those three would have gone silent, and they can least afford it:
// the class-wake channel fires only for a pane, so this knock is their only channel.
func TestUnreadCountsPaneLessWork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	hqwake.Consume(0)
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Agent: "Codex",
		Origin: events.OriginInstruction, Severity: events.SevNotable}) // a native agent's turn
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Agent: "Codex",
		Severity: events.SevNotable})
	events.Append(events.Record{Ts: now, Event: hqsurface.ControlSelfCheck,
		Severity: events.SevNotable}) // gtmux's own maintenance trigger
	// A pane-less start with no end near it: a native session coming online, not a blink.
	events.Append(events.Record{Ts: now, Event: "SessionStart", Agent: "Claude Code",
		Severity: events.SevNotable})
	// Its end arrives an hour later — far outside the pairing window, so neither is a blink.
	events.Append(events.Record{Ts: now + 3600, Event: "SessionEnd", Agent: "Claude Code",
		Severity: events.SevNotable})

	if got := unreadScan(0, testHQPane).N; got != 5 {
		t.Errorf("unread count = %d, want 5 — pane-less work is still work HQ owes a read", got)
	}
}

// TestUnreadKnockNamesItsComposition pins the diagnosability half. "1 unconsumed" — of
// what, from where? — cost HQ four rounds and a manual stream read to see the shape of a
// suspected feedback loop on 2026-08-04. The line now carries the sources.
func TestUnreadKnockNamesItsComposition(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	hqwake.Consume(0)
	for i := 0; i < 2; i++ {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%21",
			Severity: events.SevNotable})
	}
	events.Append(events.Record{Ts: now, Event: hqsurface.ControlSelfCheck, Severity: events.SevNotable})

	d := hqwake.Defaults().UnreadDebounceSec
	unreadSensorFor(testHQPane, now)
	unreadSensorFor(testHQPane, now+d)
	knocks := takeUnreadKnocks(t)
	if len(knocks) != 1 {
		t.Fatalf("want 1 knock, got %d: %v", len(knocks), knocks)
	}
	// Most numerous source first, singletons unadorned — the exact rendered form.
	if want := "(%21 ×2 · control)"; !strings.Contains(knocks[0], want) {
		t.Errorf("knock %q must carry the composition %q", knocks[0], want)
	}
	// The composition is DATA about gtmux's own stream, never a severity claim.
	for _, w := range []string{"important", "notable", "routine"} {
		if strings.Contains(knocks[0], w) {
			t.Errorf("knock %q must not classify importance (%q)", knocks[0], w)
		}
	}
}

// TestUnreadCompositionIsBounded: one delivery is capped at 800 chars across 8 lines, so a
// wide fleet must not be able to grow this line without limit.
func TestUnreadCompositionIsBounded(t *testing.T) {
	var groups []unreadGroup
	for i := 0; i < 20; i++ {
		groups = append(groups, unreadGroup{Key: "%" + strconv.Itoa(i), N: 1})
	}
	got := unreadTally{N: 20, Groups: groups}.composition()
	if strings.Count(got, "·") != unreadMaxGroups {
		t.Errorf("composition %q must show %d groups then fold the rest", got, unreadMaxGroups)
	}
	if !strings.Contains(got, "+17 more") {
		t.Errorf("composition %q must name how many sources it folded", got)
	}
	if len(got) > 64 {
		t.Errorf("composition %q is %d chars — too much of the batch budget", got, len(got))
	}
}

// Direction 3, mirroring TestSensorsNoOpWithoutASupervisor: with no supervisor resolvable
// the sensor is a COMPLETE no-op. A watermark advanced with no HQ to tell would declare an
// HQ that came online seconds later caught up on events it had never seen — the same silent
// loss in a new place.
func TestUnreadSensorNoOpWithoutASupervisor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%16",
		Severity: events.SevNotable})
	unreadSensor(now) // a temp HOME makes state.HQHome() unmatchable by any real pane
	unreadSensor(now + 100_000)

	if got := hqwake.Consumed(); got != hqwake.WatermarkUnset {
		t.Errorf("watermark advanced with no HQ: %d", got)
	}
	if k := queuedWakes(t); len(k) != 0 {
		t.Errorf("queued %d wakes with no HQ to wake: %v", len(k), k)
	}
	if st := readUnreadState(); st.Since != 0 {
		t.Errorf("sensor state advanced with no HQ: %+v", st)
	}
}

// TestConsumptionStatus pins the doctor verdict — the observability half. Without it a wake
// channel that stops landing is invisible: every knock is silent by design, so "HQ consumed
// nothing for two hours" reads exactly like "nothing happened for two hours".
func TestConsumptionStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	if got := ConsumptionStatus(now); got.State != MaintenanceNever {
		t.Errorf("no watermark yet: state = %v, want never", got.State)
	}

	hqwake.Consume(0)
	if got := ConsumptionStatus(now); got.State != MaintenanceOK || got.Unread != 0 {
		t.Errorf("caught up: got %+v, want OK/0", got)
	}

	// A small, fresh backlog is normal in-flight traffic, not a fault.
	for i := 0; i < 3; i++ {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%16"})
	}
	writeUnreadState(unreadState{Watermark: 0, Since: now - 60})
	if got := ConsumptionStatus(now); got.State != MaintenanceOK || got.Unread != 3 {
		t.Errorf("3 events behind for a minute: got %+v, want OK/3", got)
	}

	// Standing too long is the flag, even for a small backlog: the volume is not the
	// symptom, the not-consuming is.
	writeUnreadState(unreadState{Watermark: 0, Since: now - consumptionLagSecs})
	got := ConsumptionStatus(now)
	if got.State != MaintenanceSlipped {
		t.Errorf("standing %ds: state = %v, want slipped", consumptionLagSecs, got.State)
	}
	if got.StandingSec != consumptionLagSecs {
		t.Errorf("standing = %d, want %d", got.StandingSec, consumptionLagSecs)
	}

	// So is a large one, however recent.
	for i := 0; i < consumptionLagCount; i++ {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%16"})
	}
	writeUnreadState(unreadState{Watermark: 0, Since: now})
	if got := ConsumptionStatus(now); got.State != MaintenanceSlipped {
		t.Errorf("%d events behind: state = %v, want slipped", consumptionLagCount+3, got.State)
	}
}

// TestUnreadExcludesAuditTrailButCountsTriggers is hq-action-journal's exclusion rule,
// tested in BOTH directions. Direction 1: gtmux's own `gtmux:audit:*` records are trail,
// not debt — without the exclusion, every delivered wake would mint fresh debt and the
// knock would feed itself. Direction 2: a NON-audit control record (a maintenance
// trigger) must keep counting — the unread knock is the only channel it has (#647).
func TestUnreadExcludesAuditTrailButCountsTriggers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(20_000_000)

	// Bootstrap the watermark at the stream head.
	events.Append(events.Record{Ts: now - 100, Event: "Stop", State: "idle", Pane: "%1"})
	unreadSensorFor(testHQPane, now)
	wm := hqwake.Consumed()

	// Direction 1: only audit records past the watermark → zero debt, no knock, and the
	// watermark steps over them exactly like pure echo (so the scan is not repeated all day).
	events.AuditWakeDelivered(testHQPane, "» gtmux·done  %21 │ finished · #a1b2c3", now)
	events.AuditSend("%28", "landed", "check the failing test", now)
	unreadSensorFor(testHQPane, now+1+hqwake.Defaults().UnreadDebounceSec)
	unreadSensorFor(testHQPane, now+2+2*hqwake.Defaults().UnreadDebounceSec)
	if k := takeUnreadKnocks(t); len(k) != 0 {
		t.Fatalf("audit records became debt — the trail is feeding the knock: %v", k)
	}
	if got := hqwake.Consumed(); got != events.CurrentSeq() {
		t.Fatalf("watermark = %d, want stepped over the pure-trail scan to %d", got, events.CurrentSeq())
	}
	if got := hqwake.Consumed(); got <= wm {
		t.Fatalf("watermark did not move past the audit records (%d <= %d)", got, wm)
	}

	// Direction 2: a maintenance trigger (control, NOT audit) still counts and knocks.
	wm = hqwake.Consumed()
	events.Append(events.Record{Ts: now + 300, Event: "gtmux:distill",
		Summary: "distill due", Severity: events.SevNotable})
	unreadSensorFor(testHQPane, now+300)
	unreadSensorFor(testHQPane, now+301+hqwake.Defaults().UnreadDebounceSec)
	knocks := takeUnreadKnocks(t)
	if len(knocks) != 1 {
		t.Fatalf("a maintenance trigger must still knock as unread, got %v", knocks)
	}
	if !strings.Contains(knocks[0], "control") {
		t.Errorf("the knock should attribute the debt to `control`, got %q", knocks[0])
	}
	if got := hqwake.Consumed(); got != wm {
		t.Errorf("knocking about the trigger moved the watermark %d → %d", wm, got)
	}
}
