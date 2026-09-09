package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/terminal"
)

// installFakeTerminal makes `terminalInstalled(bundle)` true for a throwaway HOME, by
// putting an app bundle where it looks. Only the redirected home is touched.
func installFakeTerminal(t *testing.T, home, bundle string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, "Applications", bundle), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The "other terminals" row means terminals BESIDES the one you are in. It listed the
// one you are in whenever that terminal has no driver — kitty, WezTerm and Apple
// Terminal today — because the skip-the-host test read a field that doubled as "is it
// supported", and blank meant both "no driver" and, accidentally, "not the host".
func TestOtherTerminalsExcludesTheHostEvenWithoutADriver(t *testing.T) {
	for _, host := range []string{"kitty", "wezterm", "appleterminal"} {
		t.Run(host, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("GTMUX_TERMINAL", host)
			if terminal.HasDriver(host) {
				t.Skipf("%s has a driver now — pick another driverless terminal for this case", host)
			}
			var bundle, name string
			for _, k := range knownTerminals {
				if k.key == host {
					bundle, name = k.bundle, k.name
				}
			}
			if bundle == "" {
				t.Fatalf("%q is not in knownTerminals — detect.go resolves it, so doctor cannot name it", host)
			}
			installFakeTerminal(t, home, bundle)

			got := rowOtherTerminals()
			if strings.Contains(got.note, name) {
				t.Errorf("running in %s, the OTHER-terminals row lists %s: %q", host, name, got.note)
			}
		})
	}
}

// Support is the registry's answer, not a copy of it.
//
// Asking only about today's registry would not have caught the defect: a hard-coded copy
// that happens to be accurate reads the same. So this presents a registry gtmux does not
// have — a kitty driver — and requires the row to follow it.
func TestOtherTerminalsAsksTheRegistryForSupport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_TERMINAL", "ghostty") // host, so it is skipped and cannot confuse the read
	for _, k := range knownTerminals {
		if k.key != "ghostty" {
			installFakeTerminal(t, home, k.bundle)
		}
	}
	detail := rowOtherTerminals().note

	for _, k := range knownTerminals {
		if k.key == "ghostty" {
			continue
		}
		i := strings.Index(detail, k.name)
		if i < 0 {
			t.Errorf("%s is installed but missing from the row: %q", k.name, detail)
			continue
		}
		sensed := strings.Contains(detail[i:i+len(k.name)+14], "sensed") ||
			strings.Contains(detail[i:i+len(k.name)+14], "仅感知")
		if terminal.HasDriver(k.key) == sensed {
			t.Errorf("%s: HasDriver=%v but the row says sensed=%v — the row is not asking the registry\n  %q",
				k.name, terminal.HasDriver(k.key), sensed, detail)
		}
	}
}

// Every key here must be one internal/terminal can actually resolve a host to, or the
// skip-the-host test above silently never fires for it.
func TestKnownTerminalKeysAreResolvableNames(t *testing.T) {
	for _, k := range knownTerminals {
		t.Setenv("GTMUX_TERMINAL", k.key)
		if got := terminal.DetectedName(); got != k.key {
			t.Errorf("knownTerminals key %q does not survive resolution (got %q)", k.key, got)
		}
	}
}

// The row must follow the registry into a terminal gtmux cannot drive yet. Registering a
// kitty driver has to flip this row on its own; the field it used to read would still
// have said "sensed" forever.
func TestOtherTerminalsFollowsANewlyRegisteredDriver(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_TERMINAL", "ghostty")
	installFakeTerminal(t, home, "kitty.app")

	old := terminalHasDriver
	t.Cleanup(func() { terminalHasDriver = old })
	terminalHasDriver = func(key string) bool { return key == "kitty" || old(key) }

	got := rowOtherTerminals().note
	if strings.Contains(got, "kitty (sensed)") || strings.Contains(got, "kitty（仅感知）") {
		t.Errorf("a kitty driver is registered and the row still calls it sensed-only: %q", got)
	}
	if !strings.Contains(got, "kitty") {
		t.Fatalf("kitty is installed but absent from the row: %q", got)
	}
}
