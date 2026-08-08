package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestParseSince(t *testing.T) {
	cases := map[string]int64{"": 0, "90s": 90, "10m": 600, "2h": 7200, "45": 45, "bad": 0, "-5m": 0}
	for in, want := range cases {
		if got := parseSince(in); got != want {
			t.Errorf("parseSince(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestValidSeverity(t *testing.T) {
	for _, ok := range []string{events.SevRoutine, events.SevNotable, events.SevImportant} {
		if !validSeverity(ok) {
			t.Errorf("validSeverity(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "bogus", "IMPORTANT", "high"} {
		if validSeverity(bad) {
			t.Errorf("validSeverity(%q) = true, want false", bad)
		}
	}
}

// `gtmux events --severity important` prints only the attention stream (waiting +
// asking turn-ends), not routine chatter; an invalid level falls back to usage.
func TestEventsSeverityFilter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", State: "working", Loc: "routine:0.0"})
	events.Append(events.Record{Ts: now, Event: "Waiting", State: "waiting", Kind: "plan", Loc: "important:0.0"})
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Class: "report", Loc: "notable:0.0"})

	out := captureStdout(t, func() { CmdEvents([]string{"--severity", "important"}) })
	if !strings.Contains(out, "important:0.0") {
		t.Errorf("--severity important dropped the important record:\n%s", out)
	}
	if strings.Contains(out, "routine:0.0") || strings.Contains(out, "notable:0.0") {
		t.Errorf("--severity important leaked lower tiers:\n%s", out)
	}

	// inclusive-and-above: notable shows notable + important, not routine
	out = captureStdout(t, func() { CmdEvents([]string{"--severity", "notable"}) })
	if !strings.Contains(out, "important:0.0") || !strings.Contains(out, "notable:0.0") {
		t.Errorf("--severity notable should include notable+important:\n%s", out)
	}
	if strings.Contains(out, "routine:0.0") {
		t.Errorf("--severity notable leaked routine:\n%s", out)
	}

	// invalid level → usage (return 0, prints usage, no event lines)
	if rc := CmdEvents([]string{"--severity", "bogus"}); rc != 0 {
		t.Errorf("invalid --severity returned %d, want 0 (usage)", rc)
	}
}

// --since-seq: a one-shot delta read of everything strictly after the cursor,
// oldest first — the pull-on-wake primitive (hq-perception-v2).
func TestEventsSinceSeq(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "first:0.0"})
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "second:0.0"})
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "third:0.0"})

	out := captureStdout(t, func() { CmdEvents([]string{"--since-seq", "1"}) })
	if strings.Contains(out, "first:0.0") {
		t.Errorf("--since-seq 1 must exclude seq 1:\n%s", out)
	}
	if !strings.Contains(out, "second:0.0") || !strings.Contains(out, "third:0.0") {
		t.Errorf("--since-seq 1 must include seq 2+:\n%s", out)
	}
	// Oldest first.
	if strings.Index(out, "second:0.0") > strings.Index(out, "third:0.0") {
		t.Errorf("delta must be oldest-first:\n%s", out)
	}
	// Cursor 0 = everything retained; combinable with --json.
	out = captureStdout(t, func() { CmdEvents([]string{"--since-seq", "0", "--json"}) })
	for _, want := range []string{`"seq":1`, `"seq":2`, `"seq":3`} {
		if !strings.Contains(out, want) {
			t.Errorf("--since-seq 0 --json missing %s:\n%s", want, out)
		}
	}
	// A negative / malformed cursor → usage, exit 0.
	if rc := CmdEvents([]string{"--since-seq", "-3"}); rc != 0 {
		t.Errorf("malformed --since-seq returned %d, want 0 (usage)", rc)
	}
}

// asHQ makes this test process look like the supervisor: a temp HOME with an HQ home, and
// cwd inside it (the same cwd-keyed role rule the radar and the pull stamp use).
func asHQ(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(state.HQHome())
}

// The everyday writeback: HQ's own pull-on-wake IS the acknowledgement, so the guarantee
// needs no new habit from HQ — the delta read it already performs on every knock is what
// clears the debt.
func TestUnfilteredDeltaReadConsumes(t *testing.T) {
	asHQ(t)
	now := time.Now().Unix()
	hqwake.Consume(0)
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "web:1.0"})
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "api:0.0"})
	latest := events.CurrentSeq()

	captureStdout(t, func() { CmdEvents([]string{"--since-seq", "0", "--json"}) })
	if got := hqwake.Consumed(); got != latest {
		t.Errorf("watermark after an unfiltered delta read = %d, want %d", got, latest)
	}
}

// A FILTERED read is a triage shortcut, not consumption — the playbook's own rule, made
// mechanical. Treating `--severity important` as a full read would let HQ mark a range
// consumed while having seen only its escalation subset.
func TestFilteredReadDoesNotConsume(t *testing.T) {
	asHQ(t)
	now := time.Now().Unix()
	hqwake.Consume(0)
	events.Append(events.Record{Ts: now, Event: "Waiting", State: "waiting", Kind: "plan"})
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "web:1.0"})

	captureStdout(t, func() {
		CmdEvents([]string{"--since-seq", "0", "--severity", "important"})
	})
	if got := hqwake.Consumed(); got != 0 {
		t.Errorf("a filtered read advanced the watermark to %d", got)
	}
}

// A read that starts AHEAD of the watermark is a peek at the tail: honoring it would
// silently drop the range jumped over, which is the loss this whole mechanism prevents.
func TestSkipAheadReadDoesNotConsume(t *testing.T) {
	asHQ(t)
	now := time.Now().Unix()
	for i := 0; i < 4; i++ {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "web:1.0"})
	}
	hqwake.Consume(1)

	captureStdout(t, func() { CmdEvents([]string{"--since-seq", "3"}) })
	if got := hqwake.Consumed(); got != 1 {
		t.Errorf("a skip-ahead read moved the watermark to %d, want it left at 1", got)
	}
}

// The explicit writeback, and its two guards: only the supervisor may move its own
// watermark, and never past the end of the stream.
func TestEventsAck(t *testing.T) {
	asHQ(t)
	now := time.Now().Unix()
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "web:1.0"})
	latest := events.CurrentSeq()

	if rc := CmdEvents([]string{"--ack", "1"}); rc != 0 {
		t.Fatalf("--ack returned %d", rc)
	}
	if got := hqwake.Consumed(); got != 1 {
		t.Errorf("watermark = %d, want 1", got)
	}
	// Clamped to the stream end: a mistyped 999999 must not blind HQ to everything up to it.
	captureStdout(t, func() { CmdEvents([]string{"--ack=999999"}) })
	if got := hqwake.Consumed(); got != latest {
		t.Errorf("watermark = %d, want it clamped to the stream end %d", got, latest)
	}
	// Monotonic: an ack can never rewind and re-raise events already judged.
	captureStdout(t, func() { CmdEvents([]string{"--ack", "0"}) })
	if got := hqwake.Consumed(); got != latest {
		t.Errorf("an ack rewound the watermark to %d", got)
	}
}

// B9 (hq-unread-noise): the cd-drift that fails LOUDLY now. HQ's Bash cwd persists across
// calls, so after writing the board (`notes/`) or the KB (`knowledge/`) its next pull ran
// from a SUBDIRECTORY, consumed nothing, and the same cursor re-knocked — reproduced five
// times, the fifth in the very turn after HQ wrote the note about it. The read still
// succeeds and still prints; it just stops pretending it counted.
func TestDriftedReadWarnsAndDoesNotConsume(t *testing.T) {
	asHQ(t)
	now := time.Now().Unix()
	hqwake.Consume(0)
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "web:1.0"})

	// The exact failure shape: cwd inside the home rather than at it.
	sub := filepath.Join(state.HQHome(), "knowledge")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	out, errs := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0", "--json"}) })
	if got := hqwake.Consumed(); got != 0 {
		t.Errorf("a drifted read advanced the watermark to %d", got)
	}
	if !strings.Contains(errs, "NOT counted") && !strings.Contains(errs, "未计入") {
		t.Errorf("a drifted read must warn on stderr, got %q", errs)
	}
	if !strings.Contains(errs, state.HQHome()) {
		t.Errorf("the warning must name the home to run from, got %q", errs)
	}
	// stdout is the read's product and must be untouched by the warning.
	if !strings.Contains(out, "web:1.0") {
		t.Errorf("the read itself must still print its delta, got %q", out)
	}
}

// The same read from the home consumes and says nothing: a warning on the happy path would
// train HQ to ignore the channel.
func TestHomeReadConsumesSilently(t *testing.T) {
	asHQ(t)
	now := time.Now().Unix()
	hqwake.Consume(0)
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "web:1.0"})

	_, errs := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0"}) })
	if got := hqwake.Consumed(); got != events.CurrentSeq() {
		t.Errorf("watermark = %d, want the read counted", got)
	}
	if strings.TrimSpace(errs) != "" {
		t.Errorf("the supervisor's own correct read must be silent, got %q", errs)
	}
}

// A bystander owns no watermark and must never be nagged about one. This is why the
// warning keys on "inside the HQ home", not on "did not consume".
func TestUnrelatedCwdReadNeitherWarnsNorConsumes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	now := time.Now().Unix()
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "web:1.0"})

	_, errs := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0"}) })
	if strings.TrimSpace(errs) != "" {
		t.Errorf("a non-supervisor read must stay silent, got %q", errs)
	}
	if got := hqwake.Consumed(); got != hqwake.WatermarkUnset {
		t.Errorf("a non-supervisor read moved the watermark to %d", got)
	}
}

// A worker running `gtmux events` in its own repo must not be able to declare the
// supervisor caught up.
func TestEventsAckRefusedOutsideHQHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	if rc := CmdEvents([]string{"--ack", "5"}); rc == 0 {
		t.Error("--ack outside the HQ home returned 0, want a refusal")
	}
	if got := hqwake.Consumed(); got != hqwake.WatermarkUnset {
		t.Errorf("a non-HQ caller moved the watermark to %d", got)
	}
}
