package driver

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
)

func TestEventsReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	needle := dispatch.NormalizeNeedle("continue with P2 exactly as planned in tasks.md")
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%1", Summary: needle})
	events.Append(events.Record{Ts: now, Event: "Stop", Pane: "%1", Summary: needle})
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%2",
		Summary: dispatch.NormalizeNeedle("an unrelated submission on another pane")})

	if v := eventsReceipt("%1", needle, now-5); v != Confirmed {
		t.Errorf("matching submit on the pane: verdict = %v, want Confirmed", v)
	}
	if v := eventsReceipt("%3", needle, now-5); v != NoEvidence {
		t.Errorf("no events for the pane: verdict = %v, want NoEvidence", v)
	}
	if v := eventsReceipt("%1", dispatch.NormalizeNeedle("a completely different payload head"), now-5); v != NoEvidence {
		t.Errorf("non-matching needle: verdict = %v, want NoEvidence (a Stop must not confirm either)", v)
	}
	if v := eventsReceipt("%1", needle, now+5); v != NoEvidence {
		t.Errorf("event before the delivery window: verdict = %v, want NoEvidence", v)
	}
}

// TestFor_ReceiptReadsTheStream pins the end-to-end wiring: the registered
// capability, resolved through For(), reads the real events store — for Codex the
// same as for Claude (the commander's same-batch ruling).
func TestFor_ReceiptReadsTheStream(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	needle := dispatch.NormalizeNeedle("run the migration and report the row counts")
	events.Append(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: "%7", Summary: needle})

	for _, agent := range []string{"claude", "codex"} {
		d := For(agent)
		if d.Receipt == nil {
			t.Fatalf("For(%q).Receipt = nil", agent)
		}
		if v := d.Receipt("%7", needle, now-5); v != Confirmed {
			t.Errorf("%s: verdict = %v, want Confirmed", agent, v)
		}
	}
}
