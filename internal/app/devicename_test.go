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
		"GTMUX • iPhone · iOS 18.5": "iPhone",
		// a bare generic kind is title-cased so it doesn't read as a lowercase word
		"browser":  "Browser",
		"terminal": "Terminal",
		// untouched
		"Lin · iPad":    "Lin · iPad",
		"dev-mbp.local": "dev-mbp.local",
		"ccy":           "ccy",
		// a device legitimately named after the tool keeps something to show
		"gtmux": "gtmux",
		"":      "",
	} {
		if got := deviceDisplayName(raw); got != want {
			t.Errorf("deviceDisplayName(%q) = %q; want %q", raw, got, want)
		}
	}
}

// The roster prints the version a device reports on every request beside its name, and
// keeps it current. A name carrying its own copy said it twice and froze it at pairing:
// one roster read "iPad · iOS 26.6.1" over "iOS 26.6.1 · 127.0.0.1 · last seen 19h ago"
// (2026-09-20).
func TestDeviceDisplayNameDropsAnEchoedOSVersion(t *testing.T) {
	cases := map[string]string{
		"iPad · iOS 26.6.1":         "iPad",
		"iPhone · iOS 18.5":         "iPhone",
		"gtmux • iPhone · iOS 18.5": "iPhone",
		"Lin · iPad":                "Lin · iPad", // a name of their own keeps its parts
		"Android 34":                "Android 34", // not a "· version" tail
		"iOS 26.6.1":                "iOS 26.6.1", // nothing else left: keep it
	}
	for in, want := range cases {
		if got := deviceDisplayName(in); got != want {
			t.Errorf("deviceDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}
