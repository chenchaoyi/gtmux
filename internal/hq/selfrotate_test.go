package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// testRotatePane is a pane id tmux can never assign, so the tests walk the real delivery
// path without any chance of typing into a live pane on the developer's own machine.
const testRotatePane = "%selfrotate-test"

// rotateKnocks returns the self-rotate lines queued for delivery, and empties the queue —
// standing in for the 3 s fast tick that types a queued batch into the pane.
func rotateKnocks(t *testing.T) []string {
	t.Helper()
	var out []string
	dir := filepath.Join(state.Dir(), "hq-nudges")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "gtmux·"+hqwake.ClassSelfRotate) {
			out = append(out, string(b))
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// writeRotateCfg installs a hqWake config for the test's HOME.
func writeRotateCfg(t *testing.T, body string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRotateBreaches pins the threshold rule, including the property the whole knob set
// rests on: a NON-POSITIVE threshold disables that criterion ALONE. The three measure
// different kinds of wear, so a user who distrusts one must be able to silence it without
// losing the other two — in particular without losing the context knock, which is the one
// the 2026-08-03 incident actually needed.
func TestRotateBreaches(t *testing.T) {
	cfg := hqwake.Defaults() // ctx 0.75 · 12h · 300 turns

	cases := []struct {
		name    string
		ctx     float64
		age     int64
		turns   int
		cfg     hqwake.Config
		wantLen int
		wantHas string
	}{
		{"all clear", 0.40, 3 * 3600, 88, cfg, 0, ""},
		{"just under every line", 0.74, 12*3600 - 1, 299, cfg, 0, ""},
		{"ctx at the line breaches", 0.75, 3600, 10, cfg, 1, "ctx"},
		{"age at the line breaches", 0.10, 12 * 3600, 10, cfg, 1, "age"},
		{"turns at the line breaches", 0.10, 3600, 300, cfg, 1, "turns"},
		{"all three at once", 0.90, 40 * 3600, 500, cfg, 3, "ctx"},
		{"a disabled criterion cannot fire", 0.10, 3600, 5000,
			withTurns(cfg, 0), 0, ""},
		{"disabling one leaves the others watching", 0.90, 3600, 5000,
			withTurns(cfg, 0), 1, "ctx"},
	}
	for _, c := range cases {
		got := rotateBreaches(c.ctx, c.age, c.turns, c.cfg)
		if len(got) != c.wantLen {
			t.Errorf("%s: %d breaches %v, want %d", c.name, len(got), got, c.wantLen)
			continue
		}
		if c.wantHas != "" && got[0].What != c.wantHas {
			t.Errorf("%s: first breach = %q, want %q", c.name, got[0].What, c.wantHas)
		}
	}
}

func withTurns(c hqwake.Config, n int) hqwake.Config { c.SelfRotateTurns = n; return c }

// TestSelfRotateDecide pins the cadence, and the invariant that makes the mechanism a
// guarantee rather than a hint: NOTHING here clears the breach. Only a new session id does,
// in the caller — the same shape as the consumption watermark, applied to a different debt.
func TestSelfRotateDecide(t *testing.T) {
	const now, repeat, floor = 10_000_000, 1800, 12 * 3600
	age := []rotateBreach{{What: "age", Text: "age 13h ≥ 12h"}}
	ctxAge := []rotateBreach{{What: "ctx"}, {What: "age"}}

	cases := []struct {
		name     string
		breaches []rotateBreach
		st       rotateState
		want     rotateVerdict
	}{
		{"healthy says nothing", nil, rotateState{}, rotateHealthy},
		{"a fresh breach knocks", age, rotateState{}, rotateKnock},
		{"inside the repeat interval it holds", age,
			rotateState{KnockedAt: now - repeat + 1, Breach: "age"}, rotateHold},

		// THE CHANGE. Past the repeat interval, an unchanged breach against an unchanged
		// world no longer re-knocks — it has nothing to add. This case previously asserted
		// the opposite, and that assertion is what produced 17 knocks in one 10-hour night.
		{"a standing breach with nothing changed now HOLDS", age,
			rotateState{KnockedAt: now - repeat, Breach: "age"}, rotateHold},

		// Re-arming, which matters more than the suppression: any drift speaks again.
		{"the breach set GREW → knock", ctxAge,
			rotateState{KnockedAt: now - repeat, Breach: "age"}, rotateKnock},
		{"the fleet moved → knock", age,
			rotateState{KnockedAt: now - repeat, Breach: "age", Moved: 7, KnockMoved: 3}, rotateKnock},
		{"the safety floor fires even with nothing changed", age,
			rotateState{KnockedAt: now - floor, Breach: "age"}, rotateKnock},
		{"but the floor cannot outrun the minimum spacing", age,
			rotateState{KnockedAt: now - repeat + 1, Breach: "age"}, rotateHold},
	}
	for _, c := range cases {
		got, next := selfRotateDecide(now, c.breaches, c.st, repeat, floor)
		if got != c.want {
			t.Errorf("%s: verdict = %v, want %v", c.name, got, c.want)
		}
		if got == rotateKnock {
			if next.KnockedAt != now {
				t.Errorf("%s: a delivered knock must stamp KnockedAt, got %+v", c.name, next)
			}
			if next.Breach != breachSet(c.breaches) {
				t.Errorf("%s: a delivered knock must record its breach set, got %q", c.name, next.Breach)
			}
		}
	}
	// A session that came back under the line earns a fresh FIRST knock, not a held one.
	if _, next := selfRotateDecide(now, nil, rotateState{KnockedAt: now - 1, Breach: "age"}, repeat, floor); next.KnockedAt != 0 || next.Breach != "" {
		t.Errorf("recovering must clear the knock fingerprint, got %+v", next)
	}
}

// The measured night, replayed (ledger C17): 10 hours, an age-only breach that can never
// recover, a completely static fleet. It produced 17 knocks. The floor is what it now
// costs — one reminder, not seventeen.
func TestSelfRotateDecide_TheSeventeenKnockNight(t *testing.T) {
	const repeat, floor = int64(1800), int64(12 * 3600)
	age := []rotateBreach{{What: "age", Text: "age grows every single tick"}}
	st := rotateState{}
	knocks := 0
	// Start at a realistic unix time: KnockedAt==0 is the "never knocked" sentinel, so a
	// literal t=0 would read as never-knocked on the following tick.
	const t0 = int64(10_000_000)
	for now := t0; now <= t0+10*3600; now += 300 { // the real 5-minute check cadence
		var v rotateVerdict
		v, st = selfRotateDecide(now, age, st, repeat, floor)
		if v == rotateKnock {
			knocks++
		}
	}
	if knocks != 1 {
		t.Fatalf("the 17-knock night still knocks %d times — want exactly 1", knocks)
	}

	// And the direction that must survive: the moment the fleet actually moves, it speaks.
	st.Moved++
	if v, _ := selfRotateDecide(t0+10*3600+300, age, st, repeat, floor); v != rotateKnock {
		t.Fatal("a fleet event after a long silence must re-arm the knock")
	}
}

// The closing observation of C17: the harness auto-compacted ctx 98% → 21%, the triggering
// condition disappeared — and the knocking continued, now hanging on age alone. Age can
// never recover, so without this rule that session is nagged forever.
func TestSelfRotateDecide_CtxRecoveredButAgeStands(t *testing.T) {
	const repeat, floor = int64(1800), int64(12 * 3600)
	ctxAge := []rotateBreach{{What: "ctx"}, {What: "age"}}
	age := []rotateBreach{{What: "age"}}

	const t0 = int64(10_000_000)
	var st rotateState
	v, st := selfRotateDecide(t0, ctxAge, st, repeat, floor)
	if v != rotateKnock {
		t.Fatal("the first breach must knock")
	}
	// Auto-compact. ctx drops under its line; age cannot. The breach set SHRANK, which is
	// a change — so one knock reports the new, smaller truth...
	if v, st = selfRotateDecide(t0+repeat, age, st, repeat, floor); v != rotateKnock {
		t.Fatal("a shrinking breach set is still a change and should be reported once")
	}
	// ...and then it goes quiet, instead of re-announcing an age that only ever grows.
	for now := t0 + repeat*2; now < t0+11*3600; now += 300 {
		if v, st = selfRotateDecide(now, age, st, repeat, floor); v == rotateKnock {
			t.Fatalf("age alone re-knocked at t=%ds against a static world", now)
		}
	}
}

// Direction 1: at the threshold, HQ actually gets knocked — end to end, through the real
// delivery queue. This is the requirement the commander stated: the judgment that a session
// is worn out must not be his to make.
func TestSelfRotateKnocksAtThreshold(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	selfRotateSensorFor(testRotatePane, "sess-A", 0.82, now-14*3600, now)

	k := rotateKnocks(t)
	if len(k) != 1 {
		t.Fatalf("a session at 82%% ctx / 14h must knock exactly once, got %d: %v", len(k), k)
	}
	// The FIGURES lead the line: HQ is being asked to act on its own state, so it has to be
	// able to check the claim rather than take the verdict on faith.
	for _, want := range []string{"ctx 82%", "14h", "gtmux hq --rotate"} {
		if !strings.Contains(k[0], want) {
			t.Errorf("wake line %q is missing %q", k[0], want)
		}
	}
}

// Direction 2: below every threshold, silence. A knock the user does not believe is worse
// than no knock, so a healthy session must cost nothing and say nothing.
func TestSelfRotateSilentBelowThresholds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	selfRotateSensorFor(testRotatePane, "sess-A", 0.40, now-3*3600, now)

	if k := rotateKnocks(t); len(k) != 0 {
		t.Fatalf("a healthy session must stay silent, got %v", k)
	}
	if recs := events.Read(0, now+1); len(recs) != 0 {
		t.Fatalf("a healthy session must write nothing to the stream, got %d", len(recs))
	}
}

// Direction 3, mirroring TestSensorsNoOpWithoutASupervisor: with no supervisor resolvable
// the sensor is a COMPLETE no-op — no knock, and no state written. A health window recorded
// with no HQ would describe a session that does not exist, and the HQ that came online next
// would be judged against it.
func TestSelfRotateNoOpWithoutASupervisor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	// A temp HOME makes state.HQHome() unmatchable by any real pane, so Find() returns "".
	selfRotateSensor(now)
	selfRotateSensor(now + 100_000)

	if k := rotateKnocks(t); len(k) != 0 {
		t.Errorf("queued %d wakes with no HQ to wake: %v", len(k), k)
	}
	if st := readRotateState(); st != (rotateState{}) {
		t.Errorf("health window written with no HQ: %+v", st)
	}
	// And the doctor row must not read as a failure either — an absent supervisor is not a
	// degraded one, and a row that cried wolf on every HQ-less machine would be ignored on
	// the day it was right.
	if got := SessionHealthStatus(now); got.State != MaintenanceNever {
		t.Errorf("no live HQ must be informational, got state %v", got.State)
	}
}

// The debt clears ONLY by the act. Knocking does not clear it (the breach still stands), and
// a NEW session id does (the rotation happened, so the window it described is gone).
func TestSelfRotateDebtClearsOnlyOnRotation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)
	cfg := hqwake.Defaults()

	selfRotateSensorFor(testRotatePane, "sess-A", 0.90, now-20*3600, now)
	if len(rotateKnocks(t)) != 1 {
		t.Fatal("setup: the first breach must knock")
	}
	// Still breached, still the same session — but nothing has CHANGED, so the repeat is
	// suppressed (standing-wake-backoff). The debt is not cleared; it is simply not
	// re-announced to a consumer who already knows. Silence here is the fix, not a bug:
	// this assertion used to demand the re-knock and that demand cost 17 knocks a night.
	at := now + cfg.SelfRotateRepeatSec
	selfRotateSensorFor(testRotatePane, "sess-A", 0.90, now-20*3600, at)
	if k := rotateKnocks(t); len(k) != 0 {
		t.Fatalf("an unchanged breach must stay quiet, got %v", k)
	}
	// The debt is still owed, and the safety floor proves it: past that, it speaks again
	// even though nothing changed at all.
	at = now + cfg.SelfRotateFloorSec
	selfRotateSensorFor(testRotatePane, "sess-A", 0.90, now-20*3600, at)
	if k := rotateKnocks(t); len(k) != 1 {
		t.Fatalf("the safety floor must eventually restate an unrotated breach, got %d", len(k))
	}
	// Rotation: a NEW session id. The window is discarded whole — age and turns restart from
	// the new conversation, so the fresh session is immediately healthy.
	at += cfg.SelfRotateCheckSec
	selfRotateSensorFor(testRotatePane, "sess-B", 0.05, at-60, at)
	if k := rotateKnocks(t); len(k) != 0 {
		t.Fatalf("a rotated session must be silent, got %v", k)
	}
	if st := readRotateState(); st.Session != "sess-B" || st.Turns != 0 || st.KnockedAt != 0 {
		t.Fatalf("rotation must restart the window, got %+v", st)
	}
}

// Turn counting accrues from the event stream, counts only the HQ pane's own submissions,
// and restarts with the session. A wake HQ answered IS a turn — unlike the unread sensor,
// which must exclude HQ's own records, here they are exactly what is being measured.
func TestSelfRotateCountsHQTurns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)
	// Only the turn count may fire, so the assertion cannot be satisfied by ctx or age.
	writeRotateCfg(t, `{"hqWake":{"selfRotateCtx":0,"selfRotateHours":0,"selfRotateTurns":3}}`)

	selfRotateSensorFor(testRotatePane, "sess-A", 0, now, now) // opens the window
	if k := rotateKnocks(t); len(k) != 0 {
		t.Fatal("setup: a brand-new session must not knock")
	}
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: testRotatePane})
	events.Append(events.Record{Ts: now, Event: "Stop", Pane: testRotatePane})   // HQ's own reply — not a turn in
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%9"}) // another pane
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: testRotatePane})

	at := now + hqwake.Defaults().SelfRotateCheckSec
	selfRotateSensorFor(testRotatePane, "sess-A", 0, now, at)
	if st := readRotateState(); st.Turns != 2 {
		t.Fatalf("counted %d turns, want 2 (HQ-pane prompt submissions only)", st.Turns)
	}
	if k := rotateKnocks(t); len(k) != 0 {
		t.Fatalf("2 turns is under the threshold of 3, got %v", k)
	}
	events.Append(events.Record{Ts: at, Event: "UserPromptSubmit", Pane: testRotatePane})
	at += hqwake.Defaults().SelfRotateCheckSec
	selfRotateSensorFor(testRotatePane, "sess-A", 0, now, at)
	if k := rotateKnocks(t); len(k) != 1 {
		t.Fatalf("3 turns must knock, got %v", k)
	}
}

// The pacing gate is what keeps the sensor free on a 20 s tick: everything expensive (the
// usage snapshot, the log-head read, the journal scan) sits behind it.
func TestSelfRotateEvaluationIsPaced(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	selfRotateSensorFor(testRotatePane, "sess-A", 0.90, now-20*3600, now)
	if len(rotateKnocks(t)) != 1 {
		t.Fatal("setup: the first breach must knock")
	}
	first := readRotateState()
	// A tick one second later must not even evaluate.
	selfRotateSensorFor(testRotatePane, "sess-A", 0.90, now-20*3600, now+1)
	if got := readRotateState(); got != first {
		t.Errorf("an in-window tick must not touch the state: %+v → %+v", first, got)
	}
}

// A pane whose agent has not recorded a session yet is not a judgment gtmux can make — and,
// critically, not a rotation either: the window must not be reset by the absence of data.
func TestSelfRotateHoldsWhenNoSessionResolves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(10_000_000)

	selfRotateSensorFor(testRotatePane, "sess-A", 0.90, now-20*3600, now)
	rotateKnocks(t)
	before := readRotateState()

	at := now + hqwake.Defaults().SelfRotateCheckSec
	selfRotateSensorFor(testRotatePane, "", 0, 0, at)
	got := readRotateState()
	if got.Session != before.Session || got.Turns != before.Turns || got.KnockedAt != before.KnockedAt {
		t.Errorf("an unresolvable session must not disturb the window: %+v → %+v", before, got)
	}
	if k := rotateKnocks(t); len(k) != 0 {
		t.Errorf("nothing to judge must say nothing, got %v", k)
	}
}

// The doctor row's verdict, including the property that keeps it honest: a session whose
// health window has not been opened yet reports its turn count as UNKNOWN and omits it,
// rather than asserting a confident "0 turns" beside a two-day-old session.
func TestSessionHealthRow(t *testing.T) {
	cfg := hqwake.Defaults()

	ok := sessionHealthRow(0.41, 3*3600, 88, cfg)
	if ok.State != MaintenanceOK || ok.Over != "" {
		t.Errorf("a healthy session must read OK with no breach: %+v", ok)
	}
	if got := ok.Figures(); got != "ctx 41% · 3h · 88 turns" {
		t.Errorf("figures = %q", got)
	}

	bad := sessionHealthRow(0.82, 14*3600, 380, cfg)
	if bad.State != MaintenanceSlipped {
		t.Errorf("a breached session must be flagged: %+v", bad)
	}
	if !strings.Contains(bad.Over, "ctx") || !strings.Contains(bad.Over, "age") {
		t.Errorf("the note must name which lines were crossed, got %q", bad.Over)
	}

	unknown := sessionHealthRow(0.41, 2*24*3600, -1, cfg)
	if strings.Contains(unknown.Figures(), "turns") {
		t.Errorf("an uncounted session must omit turns, got %q", unknown.Figures())
	}
	if unknown.State != MaintenanceSlipped || strings.Contains(unknown.Over, "turns") {
		t.Errorf("an unknown turn count must not breach, but age must: %+v", unknown)
	}
}

// The state round-trips, including the empty-session sentinel — the format both halves of
// the sensor and the doctor row share.
func TestRotateStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := rotateState{Session: "abc-123", StartedAt: 1_700_000_000, Turns: 42,
		Cursor: 7210, CheckedAt: 1_700_000_100, KnockedAt: 1_700_000_200}
	writeRotateState(want)
	if got := readRotateState(); got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
	writeRotateState(rotateState{})
	if got := readRotateState(); got != (rotateState{}) {
		t.Fatalf("empty round trip = %+v, want zero", got)
	}
}

// A non-Claude HQ must not be told `ctx 0%`. The usage parser reads Claude's log shape, so
// ctxFracFor returns 0 for codex/opencode — and the figure was printed unconditionally,
// which put "plenty of room" on the very wake built to catch a judge that cannot
// self-assess. Absence must read as absence, the same rule `turns` already followed.
func TestRotateFiguresOmitsAbsentCtx(t *testing.T) {
	// codex/opencode HQ: no usage data, a real age and a real turn count.
	got := rotateFigures(0, 14*3600, 380)
	if strings.Contains(got, "ctx") {
		t.Errorf("no usage data must print no ctx figure, got %q", got)
	}
	if !strings.Contains(got, "380") {
		t.Errorf("the figures we DO have must still show: %q", got)
	}
	// Claude HQ: the figure is real and still leads.
	if got := rotateFigures(0.82, 14*3600, 380); !strings.HasPrefix(got, "ctx 82%") {
		t.Errorf("a real ctx must still lead the figures, got %q", got)
	}
	// Nothing sensed at all → an empty head rather than a fabricated one. (A wake only
	// fires on a breach, and every breach implies at least one live figure, so this is the
	// doctor-row path, not the wake path.)
	if got := rotateFigures(0, 0, -1); got != "" {
		t.Errorf("nothing sensed must render nothing, got %q", got)
	}
}
