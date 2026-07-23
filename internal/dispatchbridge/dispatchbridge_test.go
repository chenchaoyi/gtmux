package dispatchbridge

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
)

func TestHookEquipped(t *testing.T) {
	for _, ok := range []string{"claude", "claude --model opus", "codex", "/usr/bin/gemini"} {
		if !hookEquipped(ok) {
			t.Errorf("%q should be hook-equipped", ok)
		}
	}
	for _, no := range []string{"aider", "vim", "", "some-random-agent"} {
		if hookEquipped(no) {
			t.Errorf("%q should NOT be hook-equipped", no)
		}
	}
}

func TestEventsForPane_MapsAndFilters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()

	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%1", Summary: "do the thing"})
	events.Append(events.Record{Ts: now, Event: "Stop", Pane: "%1", Summary: "done"})
	events.Append(events.Record{Ts: now, Event: "PreCompact", Pane: "%1"})
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%2", Summary: "other pane"})
	events.Append(events.Record{Ts: now - 100, Event: "UserPromptSubmit", Pane: "%1", Summary: "too old"})

	got := eventsForPane("%1", now)
	// Expect the 3 recent %1 events (submit/stop/compact), not %2's, not the old one.
	kinds := map[string]string{}
	for _, e := range got {
		kinds[e.Kind] = e.Head
	}
	if kinds[dispatch.EvSubmit] != "do the thing" {
		t.Fatalf("submit head = %q (all: %+v)", kinds[dispatch.EvSubmit], got)
	}
	if _, ok := kinds[dispatch.EvStop]; !ok {
		t.Fatalf("stop event missing: %+v", got)
	}
	if _, ok := kinds[dispatch.EvCompact]; !ok {
		t.Fatalf("compact event missing: %+v", got)
	}
	for _, e := range got {
		if e.Head == "other pane" || e.Head == "too old" {
			t.Fatalf("leaked an event that should be filtered: %+v", e)
		}
	}
}

// readyGate is the settle state machine WaitAgentReady drives: it must not report
// ready until the agent has launched AND two consecutive identical ready captures
// arrive (guarding against a paste into a still-repainting boot screen).
func TestReadyGate(t *testing.T) {
	const ready = "prior output\n\n❯ "
	const banner = "2 MCP servers need authentication\n❯ "

	// A capture sequence fed one per step. cmd stays a shell until `launchAt`.
	type sample struct {
		cmd string
		cap string
	}
	steps := []sample{
		{"bash", banner},   // still a shell → not launched, capture ignored
		{"claude", banner}, // launched, but a boot banner → not ready
		{"claude", ready},  // ready once, but prev != cur → not yet settled
		{"claude", ready},  // ready AND identical to prev → settled
	}
	g := readyGate{agent: "claude"}
	got := -1
	for i, s := range steps {
		if g.step(s.cmd, func() string { return s.cap }) {
			got = i
			break
		}
	}
	if got != 3 {
		t.Fatalf("ready at step %d, want 3 (two identical ready captures)", got)
	}

	// A capture that keeps CHANGING never settles, even while each frame is "ready".
	g2 := readyGate{agent: "claude"}
	for i := 0; i < 5; i++ {
		frame := ready + "\n" + string(rune('a'+i)) // different every step
		if g2.step("claude", func() string { return frame }) {
			t.Fatalf("a changing capture settled at step %d — it must not", i)
		}
	}

	// Not launched → never ready, and the capture func is never called.
	g3 := readyGate{agent: "claude"}
	if g3.step("bash", func() string { t.Fatal("capture called before launch"); return "" }) {
		t.Fatal("a bare shell must not be ready")
	}
}

// The driver's session-start signal SHORT-CIRCUITS the settle wait: a boot whose
// screen keeps churning (MCP noise — every frame different) normally never
// settles, but once the event proves the session is up, the FIRST input-ready
// capture is enough. The gate/banner checks still apply to that capture.
func TestReadyGate_SessionStartShortCircuits(t *testing.T) {
	const ready = "prior output\n\n❯ "
	const banner = "2 MCP servers need authentication\n❯ "

	// Event fired, but a banner still on screen → NOT ready (the one-capture
	// check keeps its gate/banner teeth).
	g := readyGate{agent: "claude", sessionUp: func() bool { return true }}
	if g.step("claude", func() string { return banner }) {
		t.Fatal("a boot banner must hold the gate even with the session-start event")
	}
	// First ready capture → ready, no second identical frame needed.
	if !g.step("claude", func() string { return ready + "\nchurn-1" }) {
		t.Fatal("session-start + one ready capture must be READY")
	}

	// Regression (the slow-boot timeout): churning ready frames + NO event →
	// never settles, exactly the pre-driver behavior.
	g2 := readyGate{agent: "claude", sessionUp: func() bool { return false }}
	for i := 0; i < 5; i++ {
		frame := ready + "\nchurn-" + string(rune('a'+i))
		if g2.step("claude", func() string { return frame }) {
			t.Fatalf("no event: a changing capture must not settle (step %d)", i)
		}
	}

	// …and WITH the event the same churn is ready on its first ready frame.
	g3 := readyGate{agent: "claude", sessionUp: func() bool { return true }}
	if !g3.step("claude", func() string { return ready + "\nchurn-x" }) {
		t.Fatal("the same churn with the event must be ready at once")
	}
}

// I2: the signal's absence changes nothing — a nil sessionUp gate behaves
// byte-identically to the pre-driver gate (two identical ready captures), and a
// missing event never fails the spawn (the gate simply keeps polling).
func TestReadyGate_NoEvent_TwoFrameUnchanged(t *testing.T) {
	const ready = "prior output\n\n❯ "
	g := readyGate{agent: "claude"} // nil sessionUp
	if g.step("claude", func() string { return ready }) {
		t.Fatal("first ready capture alone must not settle without the event")
	}
	if !g.step("claude", func() string { return ready }) {
		t.Fatal("two identical ready captures must settle, event or not")
	}
}

// The signal is consulted only on a ready-but-unsettled frame — a banner frame
// (composer not ready) never pays the event-file scan.
func TestReadyGate_NoPollWhileNotComposerReady(t *testing.T) {
	const banner = "2 MCP servers need authentication\n❯ "
	polls := 0
	g := readyGate{agent: "claude", sessionUp: func() bool { polls++; return true }}
	_ = g.step("claude", func() string { return banner })
	_ = g.step("claude", func() string { return banner })
	if polls != 0 {
		t.Fatalf("a not-ready frame must not poll the signal; polled %d times", polls)
	}
}
