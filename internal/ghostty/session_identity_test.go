package ghostty

import "testing"

// FOCUS-001: decoration is separate from the known session identity. In
// particular, a legal leading quote must not be removed from the real name.
func TestTitleMatchesSessionPreservesLeadingSymbols(t *testing.T) {
	for _, session := range []string{"“synthetic“", `"synthetic"`, "--worker", "🧪 sample", "●"} {
		for _, title := range []string{session, session + " — shell ", "● " + session + " — shell", " 🔔 ● " + session, session + "  "} {
			t.Run(session+"/"+title, func(t *testing.T) {
				if !TitleMatchesSession(title, session) {
					t.Fatalf("complete session %q did not match title %q", session, title)
				}
			})
		}
	}
	for _, tc := range []struct{ title, session string }{
		{"● synthetic — shell", "“synthetic“"},
		{"“synthetic” — shell", "“synthetic“"},
		{"● “synthetic“X — shell", "“synthetic“"},
		{"● X“synthetic“ — shell", "“synthetic“"},
		{"“synthetic“ - shell", "“synthetic“"},
		{"● --worker-extra — shell", "--worker"},
		{"other — shell", "worker"},
		{"● worker — shell", ""},
		{"", ""},
		{"", "worker"},
	} {
		t.Run("reject/"+tc.title+"/"+tc.session, func(t *testing.T) {
			if TitleMatchesSession(tc.title, tc.session) {
				t.Fatalf("different or empty identity %q matched %q", tc.session, tc.title)
			}
		})
	}
}

// FOCUS-002: resolving a known session must not prefer a tolerant lookalike or
// normalize the title before AppleScript performs its exact string comparison.
func TestSessionTabPrefersLiteralIdentity(t *testing.T) {
	literal := tabTitle{win: 2, tab: 3, name: "worker — shell  \t"}
	quoted := tabTitle{win: 3, tab: 1, name: "“synthetic“ — shell "}
	decorated := tabTitle{win: 1, tab: 2, name: "🔔 “synthetic“ — shell  \t"}
	for _, tc := range []struct {
		name, session string
		titles        []tabTitle
		want          tabTitle
		found         bool
	}{
		{"literal before tolerant", "worker", []tabTitle{{name: "● worker — other"}, literal}, literal, true},
		{"quoted literal before tolerant", "“synthetic“", []tabTitle{decorated, quoted}, quoted, true},
		{"decorated fallback preserves raw bytes", "“synthetic“", []tabTitle{{name: "other — shell"}, decorated}, decorated, true},
		{"bare title", "worker", []tabTitle{{name: "worker"}}, tabTitle{name: "worker"}, true},
		{"first literal retained", "worker", []tabTitle{literal, {name: "worker — duplicate"}}, literal, true},
		{"unrelated", "worker", []tabTitle{{name: "worker-extra — shell"}}, tabTitle{}, false},
		{"empty list", "worker", nil, tabTitle{}, false},
		{"empty session", "", []tabTitle{{name: ""}, literal}, tabTitle{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, found := sessionTab(tc.titles, tc.session)
			if found != tc.found || got != tc.want {
				t.Fatalf("sessionTab = (%#v, %v), want (%#v, %v)", got, found, tc.want, tc.found)
			}
		})
	}
}
