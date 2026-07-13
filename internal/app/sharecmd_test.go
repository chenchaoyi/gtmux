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
	st := shareStateJSON{Enabled: true, Panes: []string{"%37", "%5"}}
	guests := []deviceListEntry{
		{ID: "g1", Name: "alice", EnrolledAt: 1783956522, Scope: "guest"},
		{ID: "g2", Name: "bob", EnrolledAt: 1783934731, Scope: "guest"},
	}
	out := buildShareStatus(st, guests, "https://gtmux-x.ccy.dev")

	if !out.Enabled || len(out.Panes) != 2 || out.Panes[0] != "%37" {
		t.Fatalf("state/allowlist not preserved: %+v", out)
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
	empty := buildShareStatus(shareStateJSON{Enabled: false, Panes: nil}, nil, "")
	eb, _ := json.Marshal(empty)
	if !strings.Contains(string(eb), `"panes":[]`) || !strings.Contains(string(eb), `"guests":[]`) {
		t.Errorf("empty panes/guests should be [] not null: %s", eb)
	}
}

// buildShareNew is the `gtmux share new --json` link assembler: the URL carries
// the token via the `#t=` fragment; no bare token field is emitted.
func TestBuildShareNew(t *testing.T) {
	out := buildShareNew("g9", "carol", "SECRET_TOKEN", "https://gtmux-x.ccy.dev")
	want := "https://gtmux-x.ccy.dev/#t=SECRET_TOKEN"
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
