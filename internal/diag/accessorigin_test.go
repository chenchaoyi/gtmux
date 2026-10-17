package diag

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHistoricalAccessOriginNeedsPositiveEvidenceAndDoesNotRewrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, tc := range []struct{ name, device, ip, origin string }{
		{"paired proxy", "full-device", "127.0.0.1", ""},
		{"remote credential", "", "203.0.113.8", ""},
		{"loopback owner", "", "127.0.0.1", ""},
		{"no evidence", "", "", ""},
		{"explicit unknown", "full-device", "203.0.113.8", AccessUnknown},
		{"invalid origin", "full-device", "203.0.113.8", "invented"},
	} {
		attrs := map[string]any{"operation": "sessions", "device_name": tc.name, "device_id": tc.device, "source_ip": tc.ip, "scope": "master"}
		if tc.origin != "" {
			attrs["origin"] = tc.origin
		}
		write(Entry{Component: "serve", Kind: KindAct, Level: "info", Event: "access.request", Actor: "owner", Outcome: OK, Attrs: attrs})
	}
	file := ActiveSegment(state.LogsDir(), time.Now().Format("2006-01-02"))
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	page, err := ReadAccess(AccessQuery{Origin: AccessAll})
	if err != nil || len(page.Entries) != 6 {
		t.Fatal(page, err)
	}
	for _, e := range page.Entries {
		want := AccessUnknown
		if e.Name == "paired proxy" || e.Name == "remote credential" {
			want = AccessExternal
		}
		if e.Origin != want {
			t.Fatalf("%s: %+v", e.Name, e)
		}
	}
	visible, err := ReadAccess(AccessQuery{})
	if err != nil || len(visible.Entries) != 2 || visible.Query.Origin != AccessExternal {
		t.Fatal(visible, err)
	}
	after, _ := os.ReadFile(file)
	if !bytes.Equal(before, after) {
		t.Fatal("reading rewrote historical records")
	}
}

func TestOriginFilteringPrecedesPagingAndSummariesAndBindsCursor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for i := 0; i < 3; i++ {
		For("serve").Access(AccessEntry{Origin: AccessExternal, DeviceID: "full-device", Actor: "phone:full", Name: "Phone", Operation: "send", Outcome: Refused})
		For("serve").Access(AccessEntry{Origin: AccessLocal, Actor: "menubar", Operation: "sessions", Outcome: OK})
		For("serve").Access(AccessEntry{Actor: "owner", Operation: "sessions", Outcome: OK})
	}
	first, err := ReadAccess(AccessQuery{Limit: 1})
	if err != nil || len(first.Entries) != 1 || first.Entries[0].Origin != AccessExternal || first.Next == "" {
		t.Fatal(first, err)
	}
	next, err := ReadAccess(AccessQuery{Before: first.Next})
	if err != nil || len(next.Entries) != 2 || next.Groups[0].Count != 2 || next.Groups[0].Refused != 2 {
		t.Fatal(next, err)
	}
	groups, err := ReadAccess(AccessQuery{GroupBy: "device"})
	if err != nil || len(groups.Groups) != 1 || groups.Groups[0].Count != 3 || groups.Groups[0].Origin != AccessExternal {
		t.Fatal(groups, err)
	}
	for _, origin := range []string{AccessAll, AccessLocal, AccessUnknown} {
		if _, err := ReadAccess(AccessQuery{Origin: origin, Before: first.Next}); err != ErrAccessCursor {
			t.Fatal("cursor crossed origin", origin, err)
		}
		p, err := ReadAccess(AccessQuery{Origin: origin})
		want := 3
		if origin == AccessAll {
			want = 9
		}
		if err != nil || len(p.Entries) != want || p.Query.Origin != origin {
			t.Fatal(origin, p, err)
		}
	}
	if _, err := ReadAccess(AccessQuery{Origin: "invalid"}); err != ErrAccessFilter {
		t.Fatal(err)
	}
	file := ActiveSegment(state.LogsDir(), time.Now().Format("2006-01-02"))
	oldCursor := encodeAccessCursor(filepath.Base(file), 0)
	if _, err := ReadAccess(AccessQuery{Before: oldCursor}); err != ErrAccessCursor {
		t.Fatal("legacy unbound cursor accepted", err)
	}
}

func TestExplicitMalformedOriginIsNotLegacyEvidence(t *testing.T) {
	for _, origin := range []any{"", 42, nil} {
		e, ok := accessFromEntry(Entry{Component: "serve", Event: "access.request", Kind: KindAct, Attrs: map[string]any{"origin": origin, "device_id": "verified-in-older-record", "source_ip": "203.0.113.8"}})
		if !ok || e.Origin != AccessUnknown {
			t.Fatal("malformed origin upgraded", origin, e)
		}
	}
}
