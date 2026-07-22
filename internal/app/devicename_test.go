package app

import "testing"

// The pair roster's job is letting you tell YOUR devices apart well enough to revoke the
// right one, and every entry read "gtmux • iPhone" — a product prefix inside gtmux's own
// roster (device-roster-naming).
func TestDeviceDisplayNameDropsTheLegacyPrefix(t *testing.T) {
	for raw, want := range map[string]string{
		"gtmux • iPhone":            "iPhone",
		"gtmux · iPad":              "iPad",
		"gtmux iPhone":              "iPhone",
		"GTMUX • iPhone · iOS 18.5": "iPhone · iOS 18.5",
		// untouched
		"iPhone · iOS 18.5": "iPhone · iOS 18.5",
		"ccy-mbp.local":     "ccy-mbp.local",
		// a device legitimately named after the tool keeps something to show
		"gtmux": "gtmux",
		"":      "",
	} {
		if got := deviceDisplayName(raw); got != want {
			t.Errorf("deviceDisplayName(%q) = %q; want %q", raw, got, want)
		}
	}
}
