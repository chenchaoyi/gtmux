package hq

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
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
