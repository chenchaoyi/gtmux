package app

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
