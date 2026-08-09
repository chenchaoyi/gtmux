package hook

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/native"
)

func TestIsHelperPrompt(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   bool
	}{
		{"ambient-suggestions generator", "# Overview Generate 0 to 3 hyperpersonal suggestions for the user", true},
		{"safety classifier", "You are an expert at upholding safety and preventing harm. Classify:", true},
		{"newlines normalize like the event summary", "# Overview\nGenerate 0 to 3   hyperpersonal\nsuggestions", true},
		{"real user prompt", "fix the bug in foo.go", false},
		{"real prompt that merely mentions safety", "You are asked to review the safety doc", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isHelperPrompt(c.prompt); got != c.want {
				t.Errorf("isHelperPrompt(%q) = %v, want %v", c.prompt, got, c.want)
			}
		})
	}
}

func TestHelperMarkerLifecycle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if isHelperSession("s1") {
		t.Fatal("unmarked session must not read as helper")
	}
	markHelperSession("s1")
	if !isHelperSession("s1") {
		t.Fatal("marked session must read as helper")
	}
	unmarkHelperSession("s1")
	if isHelperSession("s1") {
		t.Fatal("unmark must clear the marker")
	}
	// Expiry: a marker whose session never signals an end is pruned by TTL, not kept
	// forever (the observed helper Stops carry no session id, so no event clears it).
	markHelperSession("s2")
	old := time.Now().Add(-helperMarkerTTL - time.Hour)
	if err := os.Chtimes(helperMarkerFor("s2"), old, old); err != nil {
		t.Fatal(err)
	}
	pruneHelperMarkers(time.Now())
	if isHelperSession("s2") {
		t.Fatal("an expired marker must be pruned")
	}
}

// The ghost-row regression (2026-08-09, twice): Codex's internal helper calls — the
// ambient-suggestions generator, the auto-mode safety classifier — fire a full
// pane-less SessionStart/UserPromptSubmit/Stop lifecycle through ~/.codex/hooks.json
// with a session id that is nobody's conversation. Recorded as native sessions they
// became phantom radar rows the commander tried to kill (nothing to kill: no process,
// no pane), one stuck "working" forever. A pane-less UserPromptSubmit whose prompt is
// a known helper system prompt must instead ERASE the session: record dropped, later
// events swallowed, and the already-streamed SessionStart paired with a SessionEnd so
// the unread-debt blink exclusion covers it.
func TestRunFiltersHelperSession(t *testing.T) {
	hermeticEnv(t)

	// SessionStart arrives first; nothing identifies a helper yet, so it records.
	Run(strings.NewReader(`{"session_id":"h1","cwd":"/"}`),
		[]string{"--agent", "codex", "--detached", "SessionStart"})
	if _, ok := native.Load("h1"); !ok {
		t.Fatal("precondition: SessionStart should have recorded a native session")
	}

	// The helper prompt lands → the session is unmasked and erased.
	Run(strings.NewReader(`{"session_id":"h1","cwd":"/","prompt":"# Overview\nGenerate 0 to 3 hyperpersonal suggestions"}`),
		[]string{"--agent", "codex", "--detached", "UserPromptSubmit"})
	if _, ok := native.Load("h1"); ok {
		t.Fatal("helper UserPromptSubmit must remove the native record its SessionStart created")
	}
	if !isHelperSession("h1") {
		t.Fatal("the session must be marked so its later events are swallowed")
	}

	// The streamed lifecycle must read as a pane-less BLINK: the SessionStart got its
	// pairing SessionEnd (same agent, no pane), and the helper's UserPromptSubmit
	// itself never streamed.
	recs, _ := events.ReadSince(0)
	var starts, ends, prompts int
	for _, r := range recs {
		if r.Pane != "" {
			continue
		}
		switch r.Event {
		case "SessionStart":
			starts++
		case "SessionEnd":
			ends++
		case "UserPromptSubmit":
			prompts++
		}
	}
	if starts != 1 || ends != 1 {
		t.Fatalf("stream = %d SessionStart / %d SessionEnd, want a 1/1 blink pair", starts, ends)
	}
	if prompts != 0 {
		t.Fatal("the helper's UserPromptSubmit must not reach the event stream")
	}

	// Every later event of the marked session is a no-op: no record, no stream growth.
	before := len(recs)
	Run(strings.NewReader(`{"session_id":"h1"}`), []string{"--agent", "codex", "--detached", "Stop"})
	if _, ok := native.Load("h1"); ok {
		t.Fatal("a marked helper session's Stop must not re-create a record")
	}
	if after, _ := events.ReadSince(0); len(after) != before {
		t.Fatal("a marked helper session's Stop must not stream an event")
	}

	// Its SessionEnd clears the marker (nothing left to swallow).
	Run(strings.NewReader(`{"session_id":"h1"}`), []string{"--agent", "codex", "--detached", "SessionEnd"})
	if isHelperSession("h1") {
		t.Fatal("SessionEnd must clear the helper marker")
	}
}

// The other direction must keep working: a REAL native session — an agent running
// outside tmux with a human's prompt — is pane-less too, and pane-lessness alone must
// never be the filter (that would blind the radar to every native session).
func TestRunStillTracksRealNativeSession(t *testing.T) {
	hermeticEnv(t)

	Run(strings.NewReader(`{"session_id":"real1","cwd":"/tmp/proj"}`),
		[]string{"--agent", "codex", "--detached", "SessionStart"})
	Run(strings.NewReader(`{"session_id":"real1","cwd":"/tmp/proj","prompt":"fix the flaky test in foo_test.go"}`),
		[]string{"--agent", "codex", "--detached", "UserPromptSubmit"})

	rec, ok := native.Load("real1")
	if !ok {
		t.Fatal("a real native session must stay recorded")
	}
	if rec.State != "working" {
		t.Fatalf("state = %q, want working", rec.State)
	}
	if isHelperSession("real1") {
		t.Fatal("a real prompt must not mark the session as a helper")
	}
}
