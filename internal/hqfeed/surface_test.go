package hqfeed

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
)

func TestSurfaceTier(t *testing.T) {
	cases := map[string]string{
		events.SevImportant: SurfaceCritical,
		events.SevNotable:   SurfaceNormal,
		events.SevRoutine:   SurfaceQuiet,
		"":                  SurfaceQuiet, // legacy / unknown → non-flooding default
		"bogus":             SurfaceQuiet,
	}
	for sev, want := range cases {
		if got := SurfaceTier(sev); got != want {
			t.Errorf("SurfaceTier(%q) = %q, want %q", sev, got, want)
		}
	}
}

func TestSurfaceRank(t *testing.T) {
	if !(SurfaceRank(SurfaceQuiet) < SurfaceRank(SurfaceNormal) &&
		SurfaceRank(SurfaceNormal) < SurfaceRank(SurfaceCritical)) {
		t.Fatalf("ranks not ordered: quiet=%d normal=%d critical=%d",
			SurfaceRank(SurfaceQuiet), SurfaceRank(SurfaceNormal), SurfaceRank(SurfaceCritical))
	}
	if SurfaceRank("bogus") != 0 {
		t.Errorf("unknown tier should rank 0, got %d", SurfaceRank("bogus"))
	}
}
