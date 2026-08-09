package hqwake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// withTempState points the state dir at a temp HOME so marker/tally files are
// isolated per test.
func withTempState(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// ── signal line format (pinned fixtures — the visual language contract) ──────

func TestLineFormatFixtures(t *testing.T) {
	// The GRADE leads, in a fixed position after the sigil (hq-signal-ergonomics): `done`
	// is attention-grade, so the line opens `» ▸ gtmux·done`.
	got := Line(ClassDone, "%14 gtmux:1.2", "3m", `goal:"重构 auth"`, `tail:"tests pass, PR #12"`)
	want := `» ▸ gtmux·done  %14 gtmux:1.2 │ 3m │ goal:"重构 auth" │ tail:"tests pass, PR #12"`
	if got != want {
		t.Fatalf("done line:\n got %q\nwant %q", got, want)
	}
	// Empty fields are skipped; a headless line carries no double-space gap.
	if got := Line(ClassTick, "seq 341-352", "2 done · 1 gone", ""); got != "» · gtmux·tick  seq 341-352 │ 2 done · 1 gone" {
		t.Fatalf("tick line = %q", got)
	}
	if got := Line(ClassNewSession, ""); got != "» ▸ gtmux·new-session" {
		t.Fatalf("bare line = %q", got)
	}
}

// The sigil must stay Latin-1 (U+00BB) — the hostile-locale robustness rule — and
// every line must be valid UTF-8.
func TestLineEncodingRobustness(t *testing.T) {
	if Sigil != "»" {
		t.Fatalf("sigil drifted from U+00BB: %q", Sigil)
	}
	l := Line(ClassWaiting, "%7 api:0.0", "permission", `title:"Run tests?"`)
	if !utf8.ValidString(l) {
		t.Fatalf("line is not valid UTF-8: %q", l)
	}
	if !strings.HasPrefix(l, "» ◆ gtmux·waiting") {
		t.Fatalf("prefix drifted: %q", l)
	}
}

// ── done merge window ─────────────────────────────────────────────────────────

func TestDoneMergeWindow(t *testing.T) {
	withTempState(t)
	now := int64(1_000_000)
	due, at := DoneDue("%14", now, 120)
	if !due || at != now {
		t.Fatalf("first done should be due now: due=%v at=%d", due, at)
	}
	StampDone("%14", now)
	due, at = DoneDue("%14", now+30, 120)
	if due || at != now+120 {
		t.Fatalf("inside window: due=%v at=%d, want deferred to %d", due, at, now+120)
	}
	if due, _ := DoneDue("%14", now+120, 120); !due {
		t.Fatal("window elapsed → due again")
	}
	if due, _ := DoneDue("%99", now+30, 120); !due {
		t.Fatal("another pane is independent")
	}
}

// ── tick gate + consumption ───────────────────────────────────────────────────

func TestTickZeroChangeGate(t *testing.T) {
	withTempState(t)
	cfg := Defaults()
	if TickDue(9_999_999, cfg) {
		t.Fatal("empty tally must never be due (zero-change gate)")
	}
	AddOutcome("done")
	if !TickDue(9_999_999, cfg) {
		t.Fatal("first outcome with no prior tick → due")
	}
}

func TestTickIntervalAndBurst(t *testing.T) {
	withTempState(t)
	cfg := Defaults()
	now := int64(2_000_000)
	AddOutcome("done")
	if line := ConsumeTick(now, 10); line == "" {
		t.Fatal("consume with outcomes must produce a line")
	}
	// Just delivered → one new outcome is NOT due until the interval elapses…
	AddOutcome("done")
	if TickDue(now+60, cfg) {
		t.Fatal("inside the interval with < burst outcomes must not be due")
	}
	if !TickDue(now+cfg.TickMinutes*60, cfg) {
		t.Fatal("interval elapsed → due")
	}
	// …but the burst threshold fires early.
	for i := 0; i < cfg.TickBurst; i++ {
		AddOutcome("gone")
	}
	if !TickDue(now+61, cfg) {
		t.Fatal("burst threshold must fire the tick early")
	}
}

func TestConsumeTickLineAndSeqRange(t *testing.T) {
	withTempState(t)
	now := int64(3_000_000)
	AddOutcome("done")
	AddOutcome("done")
	AddOutcome("gone")
	line := ConsumeTick(now, 352)
	if !strings.HasPrefix(line, "» · gtmux·tick  seq 1-352") {
		t.Fatalf("tick head = %q", line)
	}
	if !strings.Contains(line, "2 done · 1 gone") {
		t.Fatalf("tick counts = %q", line)
	}
	// Consumed: nothing pending, and the next tick covers from 353.
	if TallyCount() != 0 {
		t.Fatal("tally must be consumed")
	}
	AddOutcome("done")
	if next := ConsumeTick(now+700, 360); !strings.Contains(next, "seq 353-360") {
		t.Fatalf("next tick range = %q", next)
	}
	// Empty tally → no line, stamps untouched.
	if line := ConsumeTick(now+800, 400); line != "" {
		t.Fatalf("empty consume must return \"\", got %q", line)
	}
}

// ── pull freshness ────────────────────────────────────────────────────────────

func TestPullHint(t *testing.T) {
	withTempState(t)
	now := int64(4_000_000)
	// No stamp ever → overdue.
	if h := PullHint(now, 340); !strings.Contains(h, "--since-seq 340") {
		t.Fatalf("no-stamp hint = %q", h)
	}
	StampPull() // stamps with wall-clock now
	if h := PullHint(timeNow(), 340); h != "" {
		t.Fatalf("fresh pull must yield no hint, got %q", h)
	}
}

// timeNow mirrors the wall clock used by StampPull for the freshness comparison.
func timeNow() int64 {
	fi, err := os.Stat(pullStampPath())
	if err != nil {
		return 0
	}
	return fi.ModTime().Unix()
}

// ── config ────────────────────────────────────────────────────────────────────

func TestConfigDefaultsAndParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if got := loadFrom(path); got != Defaults() {
		t.Fatalf("missing file must yield defaults: %+v", got)
	}
	_ = os.WriteFile(path, []byte(`{"hqWake":{"done":"tick","paneMinGapSec":30,"tickMinutes":5,"tickBurst":2}}`), 0o644)
	got := loadFrom(path)
	if got.Done != DoneTick || got.PaneMinGapSec != 30 || got.TickMinutes != 5 || got.TickBurst != 2 {
		t.Fatalf("parsed = %+v", got)
	}
	// Unknown mode + invalid numbers fall back per-field.
	_ = os.WriteFile(path, []byte(`{"hqWake":{"done":"bogus","tickMinutes":0}}`), 0o644)
	got = loadFrom(path)
	if got.Done != DoneUnattended || got.TickMinutes != 10 {
		t.Fatalf("fallback = %+v", got)
	}
}

// The self-rotate thresholds break the "ignore a non-positive value" rule ON PURPOSE: 0 is
// meaningful here, switching ONE criterion off while the other two keep watching. The
// cadences keep the rule, because a non-positive repeat would turn a standing hint into the
// per-tick noise it is designed not to be.
func TestConfigSelfRotate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	d := Defaults()
	if d.SelfRotateCtx != 0.75 || d.SelfRotateHours != 12 || d.SelfRotateTurns != 300 ||
		d.SelfRotateRepeatSec != 1800 || d.SelfRotateCheckSec != 300 {
		t.Fatalf("conservative defaults changed: %+v", d)
	}
	_ = os.WriteFile(path, []byte(`{"hqWake":{"selfRotateCtx":0.6,"selfRotateHours":6,`+
		`"selfRotateTurns":150,"selfRotateRepeatSec":600,"selfRotateCheckSec":60}}`), 0o644)
	got := loadFrom(path)
	if got.SelfRotateCtx != 0.6 || got.SelfRotateHours != 6 || got.SelfRotateTurns != 150 ||
		got.SelfRotateRepeatSec != 600 || got.SelfRotateCheckSec != 60 {
		t.Fatalf("parsed = %+v", got)
	}
	// A zeroed THRESHOLD must be honoured (that criterion is off); a zeroed CADENCE must not.
	_ = os.WriteFile(path, []byte(`{"hqWake":{"selfRotateTurns":0,"selfRotateRepeatSec":0}}`), 0o644)
	got = loadFrom(path)
	if got.SelfRotateTurns != 0 {
		t.Errorf("a 0 threshold must disable that criterion, got %d", got.SelfRotateTurns)
	}
	if got.SelfRotateRepeatSec != 1800 || got.SelfRotateCtx != 0.75 {
		t.Errorf("a 0 cadence must fall back, and other criteria must stand: %+v", got)
	}
}

// ── delivery priority (hq-wake-reliability) ──────────────────────────────────

// The queue drains by priority, so every class this package can build must have a
// deliberate one — an omission would silently demote a wake to the default.
func TestPriorityOf(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{Line(ClassWaiting, "main:1.0 (%14)"), PriorityDecision},
		{Line(ClassWaiting+"·permission", "main:1.0 (%14)"), PriorityDecision}, // matches on the stem
		{Line(ClassAsks, "(%14)", `ask:"which one?"`), PriorityDecision},
		{Line(ClassGoalChanged, "(%14)", `goal:"ship it"`), PriorityDecision},
		{Line(ClassCrash, "(%14)"), PriorityDecision},
		{Line(ClassFeedDegraded, ""), PriorityDecision},
		{Line(ClassWakeDegraded, ""), PriorityDecision},
		{Line(ClassDone, "(%14)", "3m"), PriorityOutcome},
		{Line(ClassResolved, "(%14)"), PriorityOutcome},
		{Line(ClassNewSession, "(%14)"), PriorityOutcome},
		{Line(ClassReapSuggest, "(%14)"), PriorityOutcome},
		{Line(ClassTick, ""), PriorityOutcome},
		{Line(ClassResourceWarn, "", "disk 14GB free"), PriorityStanding},
		{Line(ClassLimitsWarn, "", "week (fable) 93%"), PriorityStanding},
		// Periodic maintenance is the archetypal standing condition — it re-fires on its
		// own cadence, so it drains last and is evicted first. It must never sit at the
		// default (outcome) priority, which would let a weekly housekeeping knock queue
		// ahead of an agent that is blocked right now.
		{Line(ClassDistill, "due (weekly)", "distil the period into the KB"), PriorityStanding},
		{Line(ClassSelfCheck, "due (daily)", "review feed/ledger health"), PriorityStanding},
		// Self-rotate is standing too, and deliberately not decision priority despite what
		// it is about: a supervisor too heavy to judge well still has to unblock the agent
		// waiting on a human right now. Its own hygiene goes behind that.
		{Line(ClassSelfRotate, "ctx 82% · 14h · 380 turns", "over: ctx 82% ≥ 75%"), PriorityStanding},
	}
	for _, c := range cases {
		if got := PriorityOf(c.line); got != c.want {
			t.Errorf("PriorityOf(%q) = %d, want %d", c.line, got, c.want)
		}
	}
}

// Anything that is not a recognizable wake line takes the default — never a panic,
// and never priority 0 (an unknown line must not outrank a real decision wake).
func TestPriorityOf_UnknownLines(t *testing.T) {
	for _, line := range []string{"", "   ", "plain text", "» not-gtmux  x", Line("invented-class", "x")} {
		if got := PriorityOf(line); got != PriorityDefault {
			t.Errorf("PriorityOf(%q) = %d, want the default %d", line, got, PriorityDefault)
		}
	}
}

// ── resolved dedup ────────────────────────────────────────────────────────────

// ClaimResolved lets exactly ONE channel (hook fast path / slow-tick backstop) emit a
// resolved for a given clear: the first claim wins within the TTL; a second is refused;
// a different pane is independent; past the TTL a fresh clear claims again.
func TestClaimResolved(t *testing.T) {
	withTempState(t)
	now := int64(2_000_000)
	if !ClaimResolved("%88", now) {
		t.Fatal("first claim should win")
	}
	if ClaimResolved("%88", now+5) {
		t.Fatal("a second claim inside the TTL must be refused (no duplicate resolved)")
	}
	if !ClaimResolved("%99", now+5) {
		t.Fatal("a different pane claims independently")
	}
	if !ClaimResolved("%88", now+ResolvedClaimTTL) {
		t.Fatal("past the TTL a fresh clear claims again")
	}
}

// BatchID extracts a submitted wake batch's trailing id — and ONLY that: it is
// the wake channel's delivery-receipt fingerprint (the hook records it as the
// submission's Summary), so anything user-authored must yield "".
func TestBatchID(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"single wake", `» ▸ gtmux·done  gtmux:0.0 (%14) │ goal:"x" · #a3f1c2`, "#a3f1c2"},
		{"coalesced batch", `» ◆ gtmux·waiting  %1 · » ◆ gtmux·crash  %2 · +3 more queued · #09fe12`, "#09fe12"},
		{"trailing newline", "» ◆ gtmux·asks  %7 · #abc123\n", "#abc123"},
		{"no id", `» ▸ gtmux·done  gtmux:0.0 (%14) │ goal:"x"`, ""},
		{"not a wake line", "please ship the release · #a3f1c2", ""},
		{"id not hex", `» ▸ gtmux·done  %14 · #ZZZZZZ`, ""},
		{"id wrong length", `» ▸ gtmux·done  %14 · #a3f1`, ""},
		{"user text after id", "» ▸ gtmux·done  %14 · #a3f1c2\nand my own note", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := BatchID(c.in); got != c.want {
			t.Errorf("%s: BatchID(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// ── attention grade (hq-signal-ergonomics) ───────────────────────────────────

// Every declared class must have a grade. A class added without one would silently
// read as attention, which is the safe default but not a decision anyone made — this is
// the conformance check that makes the projection total.
func TestEveryClassHasAGrade(t *testing.T) {
	for _, c := range []string{
		ClassWaiting, ClassResolved, ClassAsks, ClassDone, ClassCrash, ClassGoalChanged,
		ClassNewSession, ClassReapSuggest, ClassTick, ClassStuckWaiting, ClassResourceWarn,
		ClassLimitsWarn, ClassUsageWarn, ClassFeedDegraded, ClassWakeDegraded,
		ClassDistill, ClassSelfCheck, ClassSelfRotate, ClassUnread,
	} {
		if _, ok := classGrade[c]; !ok {
			t.Errorf("class %q has no grade — add it to classGrade", c)
		}
	}
}

// A compound class grades by its stem, the same rule PriorityOf follows, so
// `waiting·permission` cannot drift away from `waiting`.
func TestGradeOfStemAndUnknown(t *testing.T) {
	if got := GradeOf(ClassWaiting + "·permission"); got != GradeDecision {
		t.Errorf("waiting·permission graded %v, want decision", got.Name())
	}
	// An unknown class reads as attention: a signal gtmux chose to deliver is at least
	// worth knowing, and grading it as bookkeeping would hide it.
	if got := GradeOf("something-new"); got != GradeAttention {
		t.Errorf("unknown class graded %v, want attention", got.Name())
	}
}

// The glyphs must clear the same encoding bar as the sigil: no emoji (variation-selector
// and presentation baggage), valid UTF-8, and distinct from each other. The `✳`-rendered-
// as-`_` incident under a locale-less serve is why this bar exists.
func TestGradeGlyphsAreEncodingSafe(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range []Grade{GradeDecision, GradeAttention, GradeLedger} {
		gl := g.Glyph()
		if !utf8.ValidString(gl) {
			t.Errorf("%s glyph is not valid UTF-8", g.Name())
		}
		if seen[gl] {
			t.Errorf("%s reuses a glyph: %q", g.Name(), gl)
		}
		seen[gl] = true
		for _, r := range gl {
			// Emoji presentation / variation selectors live well above these blocks; the
			// grammar's own characters (U+00BB, U+2502) are the precedent for staying low.
			if r > 0x2BFF {
				t.Errorf("%s glyph %q (U+%04X) is outside the safe blocks the grammar uses", g.Name(), gl, r)
			}
		}
	}
}

// The grade rides in a FIXED position — immediately after the sigil, before the class —
// so a reader (and PriorityOf) can find both without parsing the whole line.
func TestGradeLeadsTheLine(t *testing.T) {
	if got := Line(ClassCrash, "%9 api:0.0"); !strings.HasPrefix(got, "» ◆ gtmux·crash") {
		t.Errorf("decision line = %q", got)
	}
	if got := Line(ClassUnread, "3 unconsumed"); !strings.HasPrefix(got, "» · gtmux·unread") {
		t.Errorf("ledger line = %q", got)
	}
	// …and the priority parse still finds the class behind the new glyph.
	if got := PriorityOf(Line(ClassWaiting+"·plan", "%7")); got != PriorityDecision {
		t.Errorf("PriorityOf through the grade glyph = %d, want %d", got, PriorityDecision)
	}
}

// The ack must recognise its own batch THROUGH the grade glyph. It matched
// `» gtmux·` literally, so adding the grade silently broke it — and a wake whose id is
// never recognised is a wake re-sent forever, until the channel declares itself degraded.
func TestBatchIDSurvivesTheGradeGlyph(t *testing.T) {
	line := Line(ClassDone, "%14 gtmux:1.2", "3m") + " · #a3f1c2"
	if got := BatchID(line); got != "#a3f1c2" {
		t.Fatalf("BatchID through the grade glyph = %q, want #a3f1c2", got)
	}
}

// Colour is an ADDITION, never a carrier: with it off the bytes must be exactly what
// they were before the grade existed, because a pipe, a file and a --json consumer all
// read this output. The glyph already carries the grade, so a colourless terminal loses
// nothing.
func TestGradePaintIsOptIn(t *testing.T) {
	const line = "12:06:58  idle  gtmux dev  Claude Code (%18)"
	if got := GradeAttention.Paint(line, false); got != line {
		t.Errorf("colour off must be byte-identical:\n got %q\nwant %q", got, line)
	}
	got := GradeDecision.Paint(line, true)
	if !strings.HasPrefix(got, "\033[31m") || !strings.HasSuffix(got, "\033[0m") {
		t.Errorf("colour on must wrap the line: %q", got)
	}
	if !strings.Contains(got, line) {
		t.Errorf("colour must not alter the text: %q", got)
	}
	// An empty string stays empty rather than becoming a bare escape pair.
	if got := GradeLedger.Paint("", true); got != "" {
		t.Errorf("empty stays empty, got %q", got)
	}
}

// Severity and class project onto the SAME scale, so an event read in `gtmux events` and
// the knock that announced it read alike.
func TestGradeOfSeverity(t *testing.T) {
	for sev, want := range map[string]Grade{
		"important": GradeDecision,
		"notable":   GradeAttention,
		"routine":   GradeLedger,
		"":          GradeLedger, // a legacy record with no severity
	} {
		if got := GradeOfSeverity(sev); got != want {
			t.Errorf("severity %q graded %s, want %s", sev, got.Name(), want.Name())
		}
	}
}
