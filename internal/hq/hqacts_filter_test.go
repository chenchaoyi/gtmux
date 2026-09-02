package hq

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// ?acts=1 must actually drop the fleet's own records, not merely be accepted.
func TestEventsJSONActsOnlyFilters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Ts must be set: the ledger read is windowed, and a zero timestamp falls outside
	// every window — a record written without one is invisible rather than old.
	now := time.Now().Unix()
	events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%1"})
	events.Append(events.Record{Ts: now, Event: events.AuditPrefix + "send", Pane: "%2", Summary: "landed: go"})
	events.Append(events.Record{Ts: now, Event: events.AuditEventWakeDelivered, Pane: "%3"})
	b, err := EventsJSON("", 50, true)
	if err != nil {
		t.Fatal(err)
	}
	var got []events.Record
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Event != events.AuditPrefix+"send" {
		t.Fatalf("acts-only returned %d record(s): %s — want just the dispatch", len(got), b)
	}
}
