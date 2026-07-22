package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// buildShareStatus is the `gtmux share status --json` mapper. It must map the
// guest roster to the token-free {id,label,enrolled_at} shape, preserve the
// allowlist, carry the base, and NEVER emit a token field.
func TestBuildShareStatus(t *testing.T) {
	st := shareStateJSON{Enabled: true, Panes: []string{"%37", "%5"}, ViewPanes: []string{"%37", "%5", "%9"}}
	guests := []deviceListEntry{
		{ID: "g1", Name: "alice", EnrolledAt: 1783956522, Scope: "guest"},
		{ID: "g2", Name: "bob", EnrolledAt: 1783934731, Scope: "guest"},
	}
	out := buildShareStatus(st, guests, "https://gtmux-x.ccy.dev", "")

	if !out.Enabled || len(out.Panes) != 2 || out.Panes[0] != "%37" {
		t.Fatalf("state/allowlist not preserved: %+v", out)
	}
	// The VIEW allowlist is carried separately (here it's a superset of input — %9
	// is view-only), so a consumer can render see-vs-type independently.
	if len(out.ViewPanes) != 3 || out.ViewPanes[2] != "%9" {
		t.Fatalf("view_panes not preserved: %+v", out.ViewPanes)
	}
	if out.Base != "https://gtmux-x.ccy.dev" {
		t.Errorf("base = %q", out.Base)
	}
	if len(out.Guests) != 2 || out.Guests[0].ID != "g1" ||
		out.Guests[0].Label != "alice" || out.Guests[0].EnrolledAt != 1783956522 {
		t.Fatalf("guest mapping wrong: %+v", out.Guests)
	}

	// The marshaled JSON must NOT carry a token (the guest roster's secret).
	b, _ := json.Marshal(out)
	if strings.Contains(strings.ToLower(string(b)), "token") {
		t.Errorf("status --json must not contain a token: %s", b)
	}
	// A nil allowlist marshals as [] (a stable array for the consumer), not null.
	empty := buildShareStatus(shareStateJSON{Enabled: false, Panes: nil}, nil, "", "")
	eb, _ := json.Marshal(empty)
	if !strings.Contains(string(eb), `"panes":[]`) || !strings.Contains(string(eb), `"guests":[]`) ||
		!strings.Contains(string(eb), `"view_panes":[]`) {
		t.Errorf("empty panes/view_panes/guests should be [] not null: %s", eb)
	}
}

// buildShareNew is the `gtmux share new --json` link assembler: the URL carries
// the token via the `#g=` guest fragment; no bare token field is emitted.
func TestBuildShareNew(t *testing.T) {
	out := buildShareNew("g9", "carol", "SECRET_TOKEN", "https://gtmux-x.ccy.dev")
	want := "https://gtmux-x.ccy.dev/#g=SECRET_TOKEN"
	if out.URL != want {
		t.Errorf("url = %q, want %q", out.URL, want)
	}
	if out.ID != "g9" || out.Label != "carol" {
		t.Errorf("id/label wrong: %+v", out)
	}
	// The token appears ONLY inside the URL fragment, never as its own field.
	b, _ := json.Marshal(out)
	if strings.Contains(strings.ToLower(string(b)), `"token"`) {
		t.Errorf("new --json must not have a bare token field: %s", b)
	}
}

// parseExpires: durations, never, not-given, and garbage.
func TestParseExpires(t *testing.T) {
	cases := []struct {
		in    string
		secs  int64
		clear bool
		ok    bool
	}{
		{"", 0, false, true},
		{"never", 0, true, true},
		{"45m", 45 * 60, false, true},
		{"24h", 24 * 3600, false, true},
		{"7d", 7 * 86400, false, true},
		{"bogus", 0, false, false},
		{"-3h", 0, false, false},
		{"h", 0, false, false},
	}
	for _, c := range cases {
		secs, clear, ok := parseExpires(c.in)
		if secs != c.secs || clear != c.clear || ok != c.ok {
			t.Errorf("parseExpires(%q) = (%d,%v,%v), want (%d,%v,%v)", c.in, secs, clear, ok, c.secs, c.clear, c.ok)
		}
	}
}

// The status --json mapper carries each guest's per-link scope (pair-share-model),
// defaulting nils to empty arrays, and never a token.
func TestBuildShareStatus_PerLinkScope(t *testing.T) {
	st := shareStateJSON{Enabled: true, Panes: []string{"%1"}, ViewPanes: []string{"%1", "%2"}}
	guests := []deviceListEntry{
		{ID: "a", Name: "Alice", EnrolledAt: 100, ViewPanes: []string{"%1"}, InputPanes: []string{"%1"}, ExpiresAt: 999},
		{ID: "b", Name: "Bob", EnrolledAt: 200}, // legacy nils → empty arrays
	}
	out := buildShareStatus(st, guests, "https://x", "")
	if len(out.Guests) != 2 {
		t.Fatalf("guests = %d", len(out.Guests))
	}
	a := out.Guests[0]
	if len(a.ViewPanes) != 1 || len(a.Panes) != 1 || a.ExpiresAt != 999 {
		t.Fatalf("alice = %+v", a)
	}
	b := out.Guests[1]
	if b.ViewPanes == nil || b.Panes == nil || len(b.ViewPanes) != 0 {
		t.Fatalf("bob's nil scope must map to empty arrays: %+v", b)
	}
}

// guestScopeSummary renders counts + expiry.
func TestGuestScopeSummary(t *testing.T) {
	g := deviceListEntry{ViewPanes: []string{"%1", "%2"}, InputPanes: []string{"%1"}}
	if s := guestScopeSummary(g); s != "2 view · 1 type" {
		t.Fatalf("summary = %q", s)
	}
	g.ExpiresAt = 1 // long past
	if s := guestScopeSummary(g); !strings.Contains(s, "expired") && !strings.Contains(s, "已过期") {
		t.Fatalf("expired summary = %q", s)
	}
}

// share-grant-epoch: pane grants stamped against a DIFFERENT tmux server are stale, so
// `gtmux share status` can tell the owner to re-grant instead of sharing silently
// breaking (or worse, pointing at the wrong pane).
func TestBuildShareStatusFlagsStaleGrants(t *testing.T) {
	st := shareStateJSON{Enabled: true, Panes: []string{"%17"}, ViewPanes: []string{"%17"}, PaneEpoch: "111@old"}
	if out := buildShareStatus(st, nil, "", "222@new"); !out.Stale {
		t.Error("grants from a previous tmux server must be reported stale")
	}
	if out := buildShareStatus(st, nil, "", "111@old"); out.Stale {
		t.Error("grants from the SAME tmux server must not be stale")
	}
	// No tmux server running → nothing to compare, never cry stale.
	if out := buildShareStatus(st, nil, "", ""); out.Stale {
		t.Error("with no tmux identity, grants must not be reported stale")
	}
}
