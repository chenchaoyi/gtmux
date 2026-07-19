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
	got := Line(ClassDone, "%14 gtmux:1.2", "3m", `goal:"重构 auth"`, `tail:"tests pass, PR #12"`)
	want := `» gtmux·done  %14 gtmux:1.2 │ 3m │ goal:"重构 auth" │ tail:"tests pass, PR #12"`
	if got != want {
		t.Fatalf("done line:\n got %q\nwant %q", got, want)
	}
	// Empty fields are skipped; a headless line carries no double-space gap.
	if got := Line(ClassTick, "seq 341-352", "2 done · 1 gone", ""); got != "» gtmux·tick  seq 341-352 │ 2 done · 1 gone" {
		t.Fatalf("tick line = %q", got)
	}
	if got := Line(ClassNewSession, ""); got != "» gtmux·new-session" {
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
	if !strings.HasPrefix(l, "» gtmux·waiting") {
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
	if !strings.HasPrefix(line, "» gtmux·tick  seq 1-352") {
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
