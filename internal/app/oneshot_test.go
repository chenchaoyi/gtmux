package app

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/driver"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// The env discipline (multiplexer-research ⭐B): a one-shot run must not inherit
// the variables that would make it believe it is nested inside another agent
// session and recursively trigger hooks — while everything else (the proxy env
// the launch wrapped around the runner) passes through untouched.
func TestScrubHookEnv(t *testing.T) {
	in := []string{
		"PATH=/usr/bin",
		"CLAUDECODE=1",
		"CLAUDE_CODE_ENTRYPOINT=cli",
		"CMUX_SURFACE_ID=x",
		"HTTPS_PROXY=http://127.0.0.1:7897",
		"CLAUDE_CONFIG_DIR=/tmp/x", // not in the scrub set — passes through
	}
	got := strings.Join(scrubHookEnv(in), " ")
	for _, banned := range []string{"CLAUDECODE=", "CLAUDE_CODE_", "CMUX_"} {
		if strings.Contains(got, banned) {
			t.Errorf("scrub left %q in %q", banned, got)
		}
	}
	for _, kept := range []string{"PATH=", "HTTPS_PROXY=", "CLAUDE_CONFIG_DIR="} {
		if !strings.Contains(got, kept) {
			t.Errorf("scrub must keep %q; got %q", kept, got)
		}
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("fix the bug"); got != "'fix the bug'" {
		t.Errorf("plain: %q", got)
	}
	if got := shellQuote("don't break"); got != `'don'\''t break'` {
		t.Errorf("embedded quote: %q", got)
	}
}

// finishOneshot mirrors the hook's Stop/StopFailure semantics: markers set the
// pane's radar state, the completion event carries the stream's summary — and
// when the run's own hooks already recorded the completion, nothing is appended.
func TestFinishOneshot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	pane := "%77"
	start := time.Now().Unix() - 2

	finishOneshot(pane, "claude", "goal", driver.HeadlessOutcome{Summary: "shipped it"}, false, start)
	if _, err := os.Stat(state.FinishedPath(pane)); err != nil {
		t.Fatal("a successful one-shot must stamp finished/<pane>")
	}
	if _, err := os.Stat(state.ActivePath(pane)); err == nil {
		t.Fatal("active/<pane> must be cleared on completion")
	}
	recs := events.Read(60, time.Now().Unix())
	if len(recs) != 1 || recs[0].Event != "Stop" || recs[0].State != "idle" || recs[0].Summary != "shipped it" {
		t.Fatalf("completion event wrong: %+v", recs)
	}

	// A failure clears finished (a crash must never read as done) and records crash.
	t.Setenv("HOME", t.TempDir())
	finishOneshot(pane, "claude", "goal", driver.HeadlessOutcome{Summary: "api 500"}, true, start)
	if _, err := os.Stat(state.FinishedPath(pane)); err == nil {
		t.Fatal("a failed one-shot must not read as done")
	}
	recs = events.Read(60, time.Now().Unix())
	if len(recs) != 1 || recs[0].Event != "StopFailure" || recs[0].State != "crash" {
		t.Fatalf("crash event wrong: %+v", recs)
	}

	// Backstop dedup: a completion the run's own hooks already recorded is not
	// appended twice.
	t.Setenv("HOME", t.TempDir())
	events.Append(events.Record{Ts: time.Now().Unix(), Event: "Stop", Pane: pane, State: "idle"})
	finishOneshot(pane, "claude", "goal", driver.HeadlessOutcome{}, false, start)
	if recs = events.Read(60, time.Now().Unix()); len(recs) != 1 {
		t.Fatalf("hook-recorded completion must not be duplicated; got %d records", len(recs))
	}
}

// --oneshot refuses a non-headless-capable agent explicitly — never a silent
// interactive spawn (and the refusal happens before any tmux dependency).
func TestSpawn_OneshotRefusesNonCapableAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if rc := cmdSpawn([]string{"--oneshot", "--agent", "gemini", "do", "the", "thing"}); rc != 2 {
		t.Fatalf("rc = %d, want 2 (explicit refusal)", rc)
	}
	// The kill-switch closes the same door.
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(home+"/.config/gtmux", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(home+"/.config/gtmux/config.json",
		[]byte(`{"driver": {"claude": {"headless": false}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := cmdSpawn([]string{"--oneshot", "do", "the", "thing"}); rc != 2 {
		t.Fatalf("rc = %d, want 2 (switch off refuses too)", rc)
	}
}
