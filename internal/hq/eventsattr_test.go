package hq

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

// A record the hook could not name a pane for is attributable through the session id
// it carries — the whole point of recording that id.
func TestAttributedPaneFromSession(t *testing.T) {
	a := &attributor{m: map[string]string{"sess-1": "%7"}} // pane table already resolved
	a.once.Do(func() {})                                   // …so the lazy build does not overwrite it

	got := a.attributedPane(events.Record{AgentSession: "sess-1"})
	if got != "%7" {
		t.Fatalf("attributed pane = %q, want %%7", got)
	}
}

// A record that HAS a pane is never re-attributed: what the hook observed wins over
// what a binding says now, and the two can legitimately disagree.
func TestRecordedPaneIsNeverOverridden(t *testing.T) {
	a := &attributor{m: map[string]string{"sess-1": "%7"}}
	a.once.Do(func() {})
	if got := a.attributedPane(events.Record{Pane: "%3", AgentSession: "sess-1"}); got != "" {
		t.Fatalf("a recorded pane was re-attributed to %q", got)
	}
}

// The 28 records from 2026-08-18 have no session id — they predate it. Nothing may be
// invented for them.
func TestNoSessionMeansNoAttribution(t *testing.T) {
	a := &attributor{m: map[string]string{"sess-1": "%7"}}
	a.once.Do(func() {})
	if got := a.attributedPane(events.Record{Summary: "219 项断言绿"}); got != "" {
		t.Fatalf("a record with no session id was attributed to %q", got)
	}
}

// An attributed pane renders differently from a recorded one, so a reader can tell an
// observation from a lookup.
func TestAttributedRendersDistinctly(t *testing.T) {
	rec := events.Record{Ts: 1787057160, Event: "Stop", State: "idle", Agent: "Claude Code"}
	line := events.FormatAttributed(rec, "%13")
	if want := "(~%13)"; !contains(line, want) {
		t.Fatalf("attributed line %q should carry %s", line, want)
	}
	recorded := events.Format(events.Record{Ts: 1787057160, Event: "Stop", State: "idle", Agent: "Claude Code", Pane: "%13"})
	if !contains(recorded, "(%13)") || contains(recorded, "~") {
		t.Fatalf("recorded line %q must not look attributed", recorded)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// The pane table feeds the map regardless of how the radar TIERS each pane. The first
// cut filtered to agent-tier panes and attributed nothing on a live probe: this failure
// mode is exactly one where the radar's read of a pane can be wrong, because the
// conversation runs somewhere the pane's own command does not show.
func TestAttributionMapIgnoresTier(t *testing.T) {
	panes := []radar.PaneRow{{PaneID: "%0", Loc: "probe:0.0", Tier: "plain"}}
	load := func(loc string) (resume.Record, bool) {
		if loc == "probe:0.0" {
			return resume.Record{Agent: "claude", SessionID: "sess-1"}, true
		}
		return resume.Record{}, false
	}
	m := attributionMap(panes, load)
	if m["sess-1"] != "%0" {
		t.Fatalf("a plain-tier pane's binding was dropped: %v", m)
	}
}

// Two panes claiming one session is unresolvable; the map must not oscillate between
// them depending on scan order.
func TestAttributionMapKeepsTheFirstClaim(t *testing.T) {
	panes := []radar.PaneRow{
		{PaneID: "%1", Loc: "a:0.0"},
		{PaneID: "%2", Loc: "b:0.0"},
	}
	load := func(string) (resume.Record, bool) { return resume.Record{SessionID: "dup"}, true }
	if got := attributionMap(panes, load)["dup"]; got != "%1" {
		t.Fatalf("duplicate claim resolved to %q, want the first (%%1)", got)
	}
}

// A pane with no binding contributes nothing — there is no session to key on.
func TestAttributionMapSkipsUnboundPanes(t *testing.T) {
	panes := []radar.PaneRow{{PaneID: "%5", Loc: "x:0.0"}}
	load := func(string) (resume.Record, bool) { return resume.Record{}, false }
	if m := attributionMap(panes, load); len(m) != 0 {
		t.Fatalf("unbound pane produced %v", m)
	}
}
