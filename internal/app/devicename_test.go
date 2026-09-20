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

// A revoke that can only name an id tells the person who typed it nothing, and after the
// roster names stopped carrying a frozen OS version ("iPad · iOS 26.6.1"), two iPads look
// alike. The line names the device with what separates it from its twin.
func TestRevokedWhatNamesTheDeviceNotJustItsID(t *testing.T) {
	devs := []deviceListEntry{
		{ID: "e816142782cee09e", Name: "iPad · iOS 26.6.1", Platform: "iOS 26.6.1", LastIP: "127.0.0.1"},
		{ID: "c3bb6e4c5a4253e5", Name: "gtmux • iPhone", Platform: "iOS 26.6.2"},
		{ID: "32a54fd0878c6e80", Name: "browser"},
	}
	for _, c := range []struct{ id, want string }{
		{"e816142782cee09e", "iPad (iOS 26.6.1 · 127.0.0.1) · e816142782cee09e"},
		{"c3bb6e4c5a4253e5", "iPhone (iOS 26.6.2) · c3bb6e4c5a4253e5"},
		{"32a54fd0878c6e80", "Browser · 32a54fd0878c6e80"},
		{"nosuch", "nosuch"}, // nothing to add: the id alone still tells the truth
	} {
		if got := labelForRevoke(devs, c.id); got != c.want {
			t.Errorf("labelForRevoke(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}
