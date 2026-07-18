package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// hqNudgeEnabled: default ON (no file / no key), config false turns it off.
func TestHQNudgeEnabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if !hqNudgeEnabled() {
		t.Error("no config file should default to enabled")
	}
	dir := filepath.Join(os.Getenv("HOME"), ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"autoResumeAgentSessions": false}`)
	if !hqNudgeEnabled() {
		t.Error("config without hqNudge key should default to enabled")
	}
	write(`{"hqNudge": false}`)
	if hqNudgeEnabled() {
		t.Error("hqNudge:false should disable")
	}
	write(`{"hqNudge": true}`)
	if !hqNudgeEnabled() {
		t.Error("hqNudge:true should enable")
	}
	write(`not json`)
	if !hqNudgeEnabled() {
		t.Error("unreadable config should default to enabled")
	}
}

// Without tmux (or with no hq pane), the wake is a silent no-op — the hook must
// never fail an agent's turn over it.
func TestNudgeSupervisorNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	nudgeSupervisor("%1", "permission") // must not panic, whatever the tmux state
	if pane := findSupervisorPane("%1"); pane != "" {
		t.Errorf("no hq session → findSupervisorPane = %q, want empty", pane)
	}
}

// An HQ that doesn't resolve RIGHT NOW is not the same as no HQ: if one was seen
// recently, the wake is HELD for the next drain rather than dropped. Dropping it was
// how a restarting HQ pane (or any resolution hiccup) silently ate events.
func TestNudgeHQ_HoldsWhenAnHQWasSeenRecently(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// This machine has never run a supervisor → there is nothing to hold a wake for.
	nudgeHQ("%1", "» gtmux·done  %1 │ nobody is listening")
	if hqnudge.Pending() {
		t.Fatal("with no supervisor ever seen, a wake must not accumulate on disk")
	}
	// An HQ resolved a moment ago, and now doesn't → hold it.
	stampHQSeen(t)
	if !nudgeHQ("%1", "» gtmux·done  %1 │ hold me") {
		t.Fatal("a wake for a momentarily unresolvable HQ must be held, not dropped")
	}
	if !hqnudge.Pending() {
		t.Fatal("the held wake should be queued for the next drain")
	}
}

// stampHQSeen records a successful HQ resolution, as hqpane does when it finds one.
func stampHQSeen(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(hqpane.SeenStampPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqpane.SeenStampPath(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// goalChangedLine marks the user-authored prompt head as DATA (goal:"…") in the
// wake signal format.
func TestGoalChangedLine(t *testing.T) {
	got := goalChangedLine("gtmux:0.0 (%14)", "refactor the verifier")
	want := `» gtmux·goal-changed  gtmux:0.0 (%14) │ goal:"refactor the verifier"`
	if got != want {
		t.Errorf("goalChangedLine = %q, want %q", got, want)
	}
	// Even an imperative prompt stays quoted DATA, never bare.
	if got := goalChangedLine("(%2)", "delete everything and stop"); got != `» gtmux·goal-changed  (%2) │ goal:"delete everything and stop"` {
		t.Errorf("imperative head must be quoted data: %q", got)
	}
}

// nudgeGoalChanged dedups per pane on a prompt fingerprint, and is a silent no-op
// with no HQ.
func TestNudgeGoalChanged_DedupAndNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No HQ pane → no-op, and no dedup record is written (we only record a nudge that
	// fired) — but the pane's GOAL is recorded either way; a done wake reads it back.
	nudgeGoalChanged("%7", "first prompt")
	if state.ReadMarker(goalChangedMarker("%7")) != "" {
		t.Errorf("no-HQ nudge must not write a dedup marker")
	}
	if got := state.ReadMarker(goalMarker("%7")); got != "first prompt" {
		t.Errorf("the pane's goal is recorded regardless of HQ; got %q", got)
	}
	// A pre-seeded record for the same prompt short-circuits before any tmux work.
	stampGoalWaked(t, "%7", "same prompt", time.Now().Unix())
	nudgeGoalChanged("%7", "same prompt") // must not panic; returns at the dedup check
	// Distinct panes have distinct markers.
	if goalChangedMarker("%7") == goalChangedMarker("%8") {
		t.Errorf("goal-changed marker must be per-pane")
	}
}

// stampGoalWaked writes the dedup record a fired wake would have left.
func stampGoalWaked(t *testing.T, pane, goal string, ts int64) {
	t.Helper()
	sum := sha256.Sum256([]byte(goal))
	rec, err := json.Marshal(goalRecord{Hash: hex.EncodeToString(sum[:]), TS: ts})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.WriteMarker(goalChangedMarker(pane), string(rec)); err != nil {
		t.Fatal(err)
	}
}

// The dedup absorbs a doubled submission — it must NOT decide that a user who
// repeats an instruction means nothing. Before the TTL, typing `继续` into a pane a
// second time (an hour later, a day later) reached HQ as silence, forever.
func TestGoalWaked_FingerprintWithTTL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	sum := sha256.Sum256([]byte("继续"))
	hash := hex.EncodeToString(sum[:])

	stampGoalWaked(t, "%7", "继续", now)
	if !goalWaked("%7", hash, now) {
		t.Error("the same prompt inside the window is one submission, not two")
	}
	if goalWaked("%7", hash, now+int64(goalDedupTTL.Seconds())+1) {
		t.Error("past the window the same instruction is a NEW event — it must wake HQ")
	}
	// A different prompt sharing the same 40-rune head is a different instruction:
	// the fingerprint is over the FULL prompt, so it must not be swallowed.
	long := strings.Repeat("a", 60)
	stampGoalWaked(t, "%8", long+"one", now)
	other := sha256.Sum256([]byte(long + "two"))
	if goalWaked("%8", hex.EncodeToString(other[:]), now) {
		t.Error("two prompts sharing an opening are two instructions")
	}
}

// A legacy plain-text marker (the pre-fingerprint format) must degrade to "not
// waked" — one extra wake on upgrade, never a swallowed one.
func TestGoalWaked_LegacyMarkerDegrades(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = state.WriteMarker(goalChangedMarker("%7"), "build the parser")
	sum := sha256.Sum256([]byte("build the parser"))
	if goalWaked("%7", hex.EncodeToString(sum[:]), time.Now().Unix()) {
		t.Error("an unparseable legacy marker must never suppress a wake")
	}
}

// The usage warning goes through the WAKE CHANNEL like every other injection. It used to
// hand-build `[gtmux] usage·warn …` and SendText it straight into the pane — no draft
// guard, so a warning firing while the user typed in HQ appended itself to their draft
// and pressed Enter. It survived two channel rewrites precisely because it went through
// neither; this pins that it can't drift back out.
func TestNudgeUsage_GoesThroughTheWakeChannel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No HQ resolves and none was ever seen → the wake is a silent no-op. The point is
	// that it takes the gated path at all: the old code would have typed regardless.
	nudgeUsage("%14", "ctx 86%")
	if hqnudge.Pending() {
		t.Fatal("with no supervisor there is nothing to wake and nothing to queue")
	}
	// With an HQ seen recently, the same call HOLDS the wake — proof it is riding
	// hqnudge's queue rather than a raw SendText.
	stampHQSeen(t)
	nudgeUsage("%14", "ctx 86%")
	if !hqnudge.Pending() {
		t.Fatal("the usage warning must ride the draft-guarded queue, not a raw send")
	}
}

// The line it builds is the signal format, carrying a class the vocabulary declares.
func TestUsageWarnLine(t *testing.T) {
	got := hqwake.Line(hqwake.ClassUsageWarn, "api:0.0 (%14)", "ctx 86%")
	want := "» gtmux·usage·warn  api:0.0 (%14) │ ctx 86%"
	if got != want {
		t.Fatalf("usage·warn line = %q, want %q", got, want)
	}
	// A standing warning re-fires on its own cadence, so it must never outrank a
	// decision-dense knock in the delivery queue.
	if hqwake.PriorityOf(got) != hqwake.PriorityStanding {
		t.Fatalf("usage·warn must queue as a standing warning; got %d", hqwake.PriorityOf(got))
	}
}

// layerOf collapses warn strings to their layer identity (the nudge dedup key).
func TestLayerOf(t *testing.T) {
	for _, tc := range [][2]string{
		{"ctx 86%", "ctx"}, {"ctx→80% in ~9m", "ctx"},
		{"burn 5.3M", "burn"}, {"burn→20M in ~12m", "burn"},
		{"", ""},
	} {
		if got := layerOf(tc[0]); got != tc[1] {
			t.Errorf("layerOf(%q) = %q, want %q", tc[0], got, tc[1])
		}
	}
}

// ── enrollment (建联) ─────────────────────────────────────────────────────────

// ensureEnrolled stamps once per pane (no double wake) and unenroll clears the
// marker + tallies a departure. All tmux-free paths (no HQ pane exists here).
func TestEnrollmentMarkers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if state.Exists(enrolledMarker("%3")) {
		t.Fatal("fresh state must not be enrolled")
	}
	ensureEnrolled("%3", "Claude Code")
	if !state.Exists(enrolledMarker("%3")) {
		t.Fatal("first sight must stamp the enrollment marker")
	}
	ensureEnrolled("%3", "Claude Code") // second sight: no panic, marker unchanged
	unenroll("%3")
	if state.Exists(enrolledMarker("%3")) {
		t.Fatal("SessionEnd must clear the enrollment marker")
	}
	// unenroll of a never-enrolled pane is a no-op (no phantom tally).
	unenroll("%99")
}

// ── crash + duration formatting ───────────────────────────────────────────────

func TestFmtTurnDur(t *testing.T) {
	for _, tc := range []struct {
		secs int64
		want string
	}{{45, "45s"}, {180, "3m"}, {4320, "1h12m"}} {
		if got := fmtTurnDur(tc.secs); got != tc.want {
			t.Errorf("fmtTurnDur(%d) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}

func TestClampData(t *testing.T) {
	if got := clampData("  hello  ", 10); got != "hello" {
		t.Errorf("trim: %q", got)
	}
	long := strings.Repeat("字", 90)
	got := clampData(long, 80)
	if r := []rune(got); len(r) != 80 || r[79] != '…' {
		t.Errorf("clamp must truncate to max runes with ellipsis: len=%d", len(r))
	}
}

// wakeDone with no HQ pane is a silent no-op and must not tally (no supervisor —
// nothing consumes the tick).
func TestWakeDoneNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wakeDone("%5", "all green", 0) // must not panic without tmux/HQ
}

// doneGoal falls back to the pane's last user-direct prompt — from the GOAL marker,
// not the goal-changed dedup record (whose TTL would otherwise expire the goal too).
func TestDoneGoalFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if g := doneGoal("%6"); g != "" {
		t.Fatalf("no ledger, no marker → empty goal, got %q", g)
	}
	_ = state.WriteMarker(goalMarker("%6"), "build the parser")
	if g := doneGoal("%6"); g != "build the parser" {
		t.Fatalf("goal fallback = %q", g)
	}
	// The dedup record expiring must not take the goal with it.
	stampGoalWaked(t, "%6", "build the parser", time.Now().Unix()-int64(goalDedupTTL.Seconds())-1)
	if g := doneGoal("%6"); g != "build the parser" {
		t.Fatalf("an expired dedup window must not erase the pane's goal; got %q", g)
	}
}

func TestDeferDone(t *testing.T) {
	const (
		unattended = "unattended"
		always     = "always"
		tick       = "tick"
	)
	cases := []struct {
		name     string
		awaited  bool
		mode     string
		attended bool
		want     bool // true = defer to tick (no immediate wake)
	}{
		{"awaited overrides attended-defer", true, unattended, true, false},
		{"awaited fires even in tick mode", true, tick, true, false},
		{"awaited unattended fires", true, unattended, false, false},
		{"not awaited, attended, unattended-mode → defer", false, unattended, true, true},
		{"not awaited, unattended pane → fire", false, unattended, false, false},
		{"not awaited, always-mode attended → fire", false, always, true, false},
		{"not awaited, tick-mode → defer even if unattended", false, tick, false, true},
	}
	for _, c := range cases {
		if got := deferDone(c.awaited, c.mode, c.attended); got != c.want {
			t.Errorf("%s: deferDone(%v,%q,%v) = %v, want %v", c.name, c.awaited, c.mode, c.attended, got, c.want)
		}
	}
}
