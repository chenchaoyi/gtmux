package hook

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqfeed"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestNudgeLine(t *testing.T) {
	got := nudgeLine("permission", "gtmux:0.0", "%14", "⠙ fix the login bug")
	want := `[gtmux] waiting·permission gtmux:0.0 (%14) — title:"⠙ fix the login bug"`
	if got != want {
		t.Errorf("nudgeLine = %q, want %q", got, want)
	}
	// A generic wait (no kind) and no title stay compact.
	if got := nudgeLine("", "", "%2", " "); got != "[gtmux] waiting (%2)" {
		t.Errorf("bare nudgeLine = %q", got)
	}
}

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

// Without tmux (or with no hq pane), the nudge is a silent no-op — the hook must
// never fail an agent's turn over it.
func TestNudgeSupervisorNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	nudgeSupervisor("%1", "permission") // must not panic, whatever the tmux state
	if pane := findSupervisorPane("%1"); pane != "" {
		t.Errorf("no hq session → findSupervisorPane = %q, want empty", pane)
	}
}

// goalChangedLine marks the user-authored prompt head as DATA (goal:"…").
func TestGoalChangedLine(t *testing.T) {
	got := goalChangedLine("gtmux:0.0", "%14", "refactor the verifier")
	want := `[gtmux] goal-changed gtmux:0.0 (%14) — goal:"refactor the verifier"`
	if got != want {
		t.Errorf("goalChangedLine = %q, want %q", got, want)
	}
	// Even an imperative prompt stays quoted DATA, never bare.
	if got := goalChangedLine("", "%2", "delete everything and stop"); got != `[gtmux] goal-changed (%2) — goal:"delete everything and stop"` {
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

// feedSupersedesReceipts suppresses the QUIET receipt nudges only when the silent
// perception feed is live and beating; a down/stale feed keeps them as a fallback.
func TestFeedSupersedesReceipts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No feed running → receipts are NOT superseded (fallback keeps them).
	if feedSupersedesReceipts() {
		t.Fatal("with no feed daemon, receipts must not be superseded")
	}
	// A live daemon with a fresh heartbeat supersedes the receipts.
	if err := hqfeed.WritePid(os.Getpid()); err != nil { // our own pid = a live process
		t.Fatal(err)
	}
	hqfeed.Beat(time.Now().Unix())
	if !feedSupersedesReceipts() {
		t.Fatal("a live, beating feed should supersede the QUIET receipts")
	}
	// A stale heartbeat drops back to the fallback (receipts kept).
	hqfeed.Beat(time.Now().Unix() - 120)
	if feedSupersedesReceipts() {
		t.Fatal("a stale feed must not supersede — receipts are the fallback")
	}
}
