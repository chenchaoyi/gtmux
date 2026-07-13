package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resource"
)

// markerChanged is the by-TIER dedup core: a value that jitters within the same tier
// must not re-nudge; only a tier change (or a clear-then-recross) does.
func TestMarkerChanged_ByTier(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if !markerChanged("resourcewarn", "amber") {
		t.Fatal("first crossing into amber should nudge")
	}
	if markerChanged("resourcewarn", "amber") {
		t.Fatal("same tier (intra-tier jitter) must NOT re-nudge")
	}
	if !markerChanged("resourcewarn", "red") {
		t.Fatal("escalation amber→red should nudge")
	}
	if markerChanged("resourcewarn", "") {
		t.Fatal("clearing to normal must not nudge")
	}
	if !markerChanged("resourcewarn", "amber") {
		t.Fatal("re-crossing into amber after a clear should nudge again")
	}
}

// limitsTierKey collapses a limits warn to its window identity so a climbing % within
// the same window doesn't re-nudge.
func TestLimitsTierKey(t *testing.T) {
	for _, c := range [][2]string{
		{"week (fable) 93%", "week (fable)"},
		{"week (fable) 94%", "week (fable)"}, // same key despite the % jitter
		{"week (all models) 88%", "week (all models)"},
		{"  ", ""},
		{"", ""},
	} {
		if got := limitsTierKey(c[0]); got != c[1] {
			t.Errorf("limitsTierKey(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestResourceTierKey(t *testing.T) {
	if got := resourceTierKey(resource.Machine{}); got != "" {
		t.Errorf("no warn → empty key, got %q", got)
	}
	m := resource.Machine{Warn: "disk 40Gi", DiskFreeGB: 40}
	if got := resourceTierKey(m); got == "" || got != resource.MachineTier(m).String() {
		t.Errorf("warn set → tier key %q (MachineTier=%q)", got, resource.MachineTier(m).String())
	}
}
