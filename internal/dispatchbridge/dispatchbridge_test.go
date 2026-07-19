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
