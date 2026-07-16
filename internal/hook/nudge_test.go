package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// nudgeGoalChanged dedups per pane on the head, and is a silent no-op with no HQ.
func TestNudgeGoalChanged_DedupAndNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No HQ pane → no-op, and no marker is written (we only record a nudge that fired).
	nudgeGoalChanged("%7", "first prompt")
	if state.ReadMarker(goalChangedMarker("%7")) != "" {
		t.Errorf("no-HQ nudge must not write a dedup marker")
	}
	// A pre-seeded marker equal to the head short-circuits before any tmux work.
	_ = state.WriteMarker(goalChangedMarker("%7"), "same head")
	nudgeGoalChanged("%7", "same head") // must not panic; returns at the dedup check
	// Distinct panes have distinct markers.
	if goalChangedMarker("%7") == goalChangedMarker("%8") {
		t.Errorf("goal-changed marker must be per-pane")
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

// doneGoal falls back to the pane's last user-direct prompt head.
func TestDoneGoalFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if g := doneGoal("%6"); g != "" {
		t.Fatalf("no ledger, no marker → empty goal, got %q", g)
	}
	_ = state.WriteMarker(goalChangedMarker("%6"), "build the parser")
	if g := doneGoal("%6"); g != "build the parser" {
		t.Fatalf("goal fallback = %q", g)
	}
}
