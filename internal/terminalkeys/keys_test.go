package terminalkeys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestEncoding(t *testing.T) {
	for _, tc := range []struct {
		e    Event
		want string
	}{
		{Event{"F2", nil}, "F2"}, {Event{"F2", []string{"shift", "control"}}, "C-S-F2"},
		{Event{"ArrowLeft", []string{"shift"}}, "S-Left"}, {Event{"Tab", []string{"shift"}}, "BTab"},
		{Event{"ArrowLeft", []string{"control", "alt"}}, "C-M-Left"},
		{Event{"a", []string{"shift"}}, "A"}, {Event{"b", []string{"alt", "shift"}}, "M-B"},
		{Event{"C", []string{"control"}}, "C-c"}, {Event{"PageUp", nil}, "PPage"},
	} {
		got, err := Encode(tc.e)
		if err != nil || got != tc.want {
			t.Errorf("%+v: %q %v, want %q", tc.e, got, err, tc.want)
		}
	}
}

func TestInvalidAndAmbiguousEvents(t *testing.T) {
	for _, e := range []Event{
		{"NotAKey", nil}, {"F13", nil}, {"C-c", nil}, {"\x1b", nil}, {"rm -rf", nil}, {"中", nil},
		{"F2", []string{"control", "control"}}, {"F2", []string{"meta"}},
		{"a", []string{"control", "shift"}}, {"1", []string{"control"}}, {"Enter", []string{"shift"}},
	} {
		if got, err := Encode(e); err == nil {
			t.Errorf("unsafe/ambiguous event %+v encoded as %q", e, got)
		}
	}
}

func TestCatalogIsDerivedFromEncoder(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Catalog() {
		if seen[k.Key] {
			t.Fatalf("duplicate %s", k.Key)
		}
		seen[k.Key] = true
		for _, mods := range k.ModifierSets {
			if _, err := Encode(Event{k.Key, mods}); err != nil {
				t.Fatalf("advertised %+v: %v", k, err)
			}
		}
	}
	for i := 1; i <= 12; i++ {
		if !seen[fmt.Sprintf("F%d", i)] {
			t.Errorf("missing F%d", i)
		}
	}
	if !seen["F12"] || !seen["ArrowLeft"] || !seen["a"] {
		t.Fatal("missing key family")
	}
}

func TestDemoCatalogTracksCore(t *testing.T) {
	b, err := os.ReadFile("../../mobileapp/src/api/terminalKeysCatalog.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(struct {
		Keys   []Key            `json:"keys"`
		Legacy map[string]Event `json:"legacy_events"`
	}{Catalog(), LegacyEvents()})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(b), want) {
		t.Fatal("demo catalog drifted; run go generate ./internal/terminalkeys")
	}
}
