package hqfeed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
)

func writeConfig(t *testing.T, body string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveThresholdDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_SURFACE_TIER", "")
	t.Setenv("GTMUX_QUIET", "")
	if got := ResolveThreshold(); got != SurfaceNormal {
		t.Fatalf("default threshold = %q, want normal", got)
	}
	if QuietOn() {
		t.Error("default should not be quiet")
	}
}

func TestResolveThresholdPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// config quiet:true → critical-only.
	writeConfig(t, `{"quiet":true}`)
	t.Setenv("GTMUX_SURFACE_TIER", "")
	t.Setenv("GTMUX_QUIET", "")
	if got := ResolveThreshold(); got != SurfaceCritical {
		t.Fatalf("config quiet → %q, want critical", got)
	}
	if !QuietOn() {
		t.Error("QuietOn should be true when quiet:true")
	}

	// config surfaceTier wins over the default (but not over quiet).
	writeConfig(t, `{"surfaceTier":"quiet"}`)
	if got := ResolveThreshold(); got != SurfaceQuiet {
		t.Fatalf("config surfaceTier=quiet → %q", got)
	}

	// env GTMUX_QUIET overrides config.
	writeConfig(t, `{"surfaceTier":"quiet"}`)
	t.Setenv("GTMUX_QUIET", "1")
	if got := ResolveThreshold(); got != SurfaceCritical {
		t.Fatalf("env GTMUX_QUIET → %q, want critical", got)
	}

	// env GTMUX_SURFACE_TIER wins over everything.
	t.Setenv("GTMUX_SURFACE_TIER", "normal")
	if got := ResolveThreshold(); got != SurfaceNormal {
		t.Fatalf("env GTMUX_SURFACE_TIER=normal → %q, want normal", got)
	}
	// An invalid env tier is ignored (falls through to GTMUX_QUIET=1 → critical).
	t.Setenv("GTMUX_SURFACE_TIER", "bogus")
	if got := ResolveThreshold(); got != SurfaceCritical {
		t.Fatalf("invalid env tier should fall through, got %q", got)
	}
}

func TestShouldSurface(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_SURFACE_TIER", "")

	// Default (normal): notable+important surface, routine does not.
	t.Setenv("GTMUX_QUIET", "")
	if !ShouldSurface(events.SevImportant) || !ShouldSurface(events.SevNotable) {
		t.Error("normal threshold should surface important + notable")
	}
	if ShouldSurface(events.SevRoutine) {
		t.Error("normal threshold should NOT surface routine")
	}

	// Quiet on (critical-only): only important surfaces; notable goes to the ledger.
	t.Setenv("GTMUX_QUIET", "1")
	if !ShouldSurface(events.SevImportant) {
		t.Error("critical threshold must still surface important")
	}
	if ShouldSurface(events.SevNotable) {
		t.Error("critical threshold should NOT surface notable")
	}
}

// TestDegradationNeverSuppressed pins §4's guarantee: a feed degradation is severity
// `important` → maps to CRITICAL → surfaces at EVERY threshold, including quiet-on.
func TestDegradationNeverSuppressed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_SURFACE_TIER", "")
	// Even at the quietest bar, a degradation (important) surfaces.
	t.Setenv("GTMUX_QUIET", "1")
	if SurfaceTier(events.SevImportant) != SurfaceCritical {
		t.Fatal("a degradation must map to CRITICAL")
	}
	if !ShouldSurface(events.SevImportant) {
		t.Fatal("a CRITICAL degradation must surface even under quiet mode")
	}
}
