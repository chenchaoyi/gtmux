package hq

import (
	"encoding/json"
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

// The supervisor's pull shows exactly the DEBT (hq-unread-noise task 6.1). Measured, 68.7 %
// of what a knock sent HQ to read was its own echo — the count excluded those records, the
// read returned them, and HQ spent a turn per knock digging one new fact out of its own
// trail. Now the two sets are the same set.
func TestSupervisorPullHidesItsOwnEcho(t *testing.T) {
	asHQ(t)
	t.Setenv("TMUX_PANE", "%4") // HQ reads from its own pane; no tmux round-trip needed
	now := time.Now().Unix()
	hqwake.Consume(0)

	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%4",
		Summary: "#a3f1c2"}) // the wake echoed back
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%4"}) // HQ's reply
	events.Append(events.Record{Ts: now, Event: "SessionStart", Agent: "Claude Code"})
	events.Append(events.Record{Ts: now, Event: "SessionEnd", Agent: "Claude Code"}) // a blink
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%21", Loc: "web:1.0"})
	latest := events.CurrentSeq()

	out, errs := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0"}) })
	if strings.Contains(out, "#a3f1c2") {
		t.Errorf("the pull showed HQ its own echo:\n%s", out)
	}
	if !strings.Contains(out, "web:1.0") {
		t.Errorf("the pull must still show the actual debt:\n%s", out)
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("want exactly the 1 debt record, got:\n%s", out)
	}
	// Hiding must never be silent — that was B9's failure mode in a new place.
	if !strings.Contains(errs, "4") {
		t.Errorf("stderr must say how many records were withheld, got %q", errs)
	}
	// And it is NOT a filtered read: it showed precisely what HQ owed, so it consumes.
	if got := hqwake.Consumed(); got != latest {
		t.Errorf("watermark = %d, want the pull still counted as consumption (%d)", got, latest)
	}
}

// --all is the escape hatch for when HQ needs its own trail back, and it consumes too —
// it is a superset of the debt, not a subset.
func TestPullAllShowsEverythingAndStillConsumes(t *testing.T) {
	asHQ(t)
	t.Setenv("TMUX_PANE", "%4")
	now := time.Now().Unix()
	hqwake.Consume(0)
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%4", Loc: "hq:0.0"})
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%21", Loc: "web:1.0"})
	latest := events.CurrentSeq()

	out, errs := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0", "--all"}) })
	if !strings.Contains(out, "hq:0.0") || !strings.Contains(out, "web:1.0") {
		t.Errorf("--all must show everything past the cursor:\n%s", out)
	}
	if strings.TrimSpace(errs) != "" {
		t.Errorf("--all hides nothing, so it must say nothing; got %q", errs)
	}
	if got := hqwake.Consumed(); got != latest {
		t.Errorf("watermark = %d, want %d", got, latest)
	}
}

// A non-supervisor read is untouched: the pull view is the SUPERVISOR's view of its own
// debt, and a worker tailing the stream in a repo has no debt and no echo to hide.
func TestNonSupervisorPullIsUnfiltered(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	t.Setenv("TMUX_PANE", "%4")
	now := time.Now().Unix()
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%4", Loc: "hq:0.0"})

	out, errs := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0"}) })
	if !strings.Contains(out, "hq:0.0") {
		t.Errorf("a bystander's read must not be reshaped by HQ's debt rules:\n%s", out)
	}
	if strings.TrimSpace(errs) != "" {
		t.Errorf("…and must stay silent; got %q", errs)
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

// The supervisor's pull hides the audit trail exactly as the count excludes it
// (hq-action-journal): the two sets must not drift, or either the silent-loss hole or
// the echo cost re-opens. --all restores it; a bystander is untouched.
func TestSupervisorPullHidesAuditTrail(t *testing.T) {
	asHQ(t)
	t.Setenv("TMUX_PANE", "%4")
	now := time.Now().Unix()
	hqwake.Consume(0)

	events.AuditWakeDelivered("%4", "» gtmux·done  %21 │ finished · #b2c3d4", now)
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%21", Loc: "web:1.0"})
	latest := events.CurrentSeq()

	out, errs := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0"}) })
	if strings.Contains(out, "#b2c3d4") {
		t.Errorf("the default pull showed the audit trail:\n%s", out)
	}
	if !strings.Contains(out, "web:1.0") {
		t.Errorf("the pull must still show the actual debt:\n%s", out)
	}
	if !strings.Contains(errs, "1") {
		t.Errorf("stderr must count the withheld audit record, got %q", errs)
	}
	if got := hqwake.Consumed(); got != latest {
		t.Errorf("watermark = %d, want %d — hiding trail must not stop consumption", got, latest)
	}

	// --all restores the trail (and a maintenance trigger was never hidden at all).
	hqwake.Consume(0)
	out, _ = captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0", "--all"}) })
	if !strings.Contains(out, "#b2c3d4") {
		t.Errorf("--all must restore the audit trail:\n%s", out)
	}
}

// A worker tailing the stream sees the audit trail: the trail is for auditors, and only
// the supervisor's debt view reshapes a read.
func TestNonSupervisorPullShowsAuditTrail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	now := time.Now().Unix()
	events.AuditReap("t-9", "%21", "worktree removed", now)

	out, errs := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0"}) })
	if !strings.Contains(out, "t-9") {
		t.Errorf("a bystander's read must include the audit trail:\n%s", out)
	}
	if strings.TrimSpace(errs) != "" {
		t.Errorf("…and stay silent; got %q", errs)
	}
}

// The count set and the pull set are the SAME set (the hq-unread-noise invariant, now
// with three exclusion classes): over one mixed delta, the records the supervisor's
// default pull shows are exactly the records the unread tally counts. If these two ever
// disagree, either real debt hides (the silent-loss hole) or echo returns (the 68.7 %).
func TestCountAndPullExcludeTheSameSet(t *testing.T) {
	asHQ(t)
	t.Setenv("TMUX_PANE", "%4")
	now := time.Now().Unix()

	// One of everything: echo, blink pair, audit, a maintenance trigger, worker records.
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%4", Summary: "#e1f2a3"})
	events.Append(events.Record{Ts: now, Event: "SessionStart", Agent: "Claude Code"})
	events.Append(events.Record{Ts: now, Event: "SessionEnd", Agent: "Claude Code"})
	events.AuditWakeDelivered("%4", "» gtmux·done  %21 · #e1f2a3", now)
	events.AuditSend("%9", "landed", "go", now)
	events.Append(events.Record{Ts: now, Event: "gtmux:distill", Summary: "due"})
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%21", Loc: "web:1.0"})
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%9", Origin: events.OriginInstruction})

	tally := unreadScan(0, "%4")
	delta, _ := events.ReadSince(0)
	shown, hidden := pullView(delta, true)
	if len(shown) != tally.N {
		t.Fatalf("pull shows %d records but the tally counts %d — the two sets drifted", len(shown), tally.N)
	}
	if len(shown)+hidden != len(delta) {
		t.Fatalf("shown %d + hidden %d ≠ delta %d", len(shown), hidden, len(delta))
	}
	// And the composition is the expected one: trigger + two worker records.
	if tally.N != 3 {
		t.Fatalf("counted %d, want 3 (the trigger and the two worker records)", tally.N)
	}
}

// The read-time gap warning (retire-perception-spool): a hole in the pulled
// sequence must warn the reader on stderr at the moment of the read, while
// stdout stays record-only so --json consumers keep parsing.
func TestEventsSinceSeqWarnsOnGap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	for _, loc := range []string{"a:0.0", "b:0.0", "c:0.0", "d:0.0"} {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: loc})
	}
	// Punch a hole: drop seq 2 from the journal, as a retention overrun would.
	b, err := os.ReadFile(events.Path())
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if !strings.Contains(line, `"seq":2`) {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile(events.Path(), []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "1"}) })
	if !strings.Contains(stderr, "gap") {
		t.Errorf("a holed pull must warn on stderr:\nstderr=%q", stderr)
	}
	if !strings.Contains(stdout, "c:0.0") || !strings.Contains(stdout, "d:0.0") {
		t.Errorf("the retained tail must still print:\n%s", stdout)
	}

	// --json: every stdout line stays parseable; the warning rides stderr only.
	stdout, stderr = captureBoth(t, func() { CmdEvents([]string{"--since-seq", "1", "--json"}) })
	if !strings.Contains(stderr, "gap") {
		t.Errorf("--json must still warn on stderr:\nstderr=%q", stderr)
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		var r events.Record
		if json.Unmarshal([]byte(line), &r) != nil {
			t.Errorf("--json stdout line not parseable: %q", line)
		}
	}
}

// A contiguous tail stays clean: no warning, byte-for-byte the old output.
func TestEventsSinceSeqContiguousStaysQuiet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	for _, loc := range []string{"a:0.0", "b:0.0", "c:0.0"} {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: loc})
	}
	_, stderr := captureBoth(t, func() { CmdEvents([]string{"--since-seq", "1"}) })
	if strings.Contains(stderr, "gap") {
		t.Errorf("a contiguous pull must not warn:\nstderr=%q", stderr)
	}
	// Cursor 0 = "everything retained": no prior position, never a leading gap.
	_, stderr = captureBoth(t, func() { CmdEvents([]string{"--since-seq", "0"}) })
	if strings.Contains(stderr, "gap") {
		t.Errorf("cursor 0 must never report a leading gap:\nstderr=%q", stderr)
	}
}
