package ghostty

import "testing"

// A tab title can carry a leading glyph from TWO sources, and matching must survive both:
//   - the TERMINAL's own (Ghostty prefixes a background tab that rang the bell)
//   - GTMUX's own (`tab-alert` marks a session with an agent waiting)
//
// The second one shipped in v0.51.0 while the focus path still compared the raw title, so
// clicking a waiting session in the menu bar did nothing — and tab-alert marks ONLY the
// waiting ones, so the failure landed exactly on the rows a user clicks.
func TestTitleMatchesSessionToleratesDecoration(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		session string
		want    bool
	}{
		{"exact", "MP", "MP", true},
		{"with window", "MP — multipilot", "MP", true},
		{"gtmux tab-alert marker", "● MP — multipilot", "MP", true},
		{"marker, no window", "● MP", "MP", true},
		{"terminal bell glyph", "🔔 dev-workspace — vim", "dev-workspace", true},
		{"leading spaces", "   MP — multipilot", "MP", true},

		// Tolerance must not become looseness: a decorated title still has to name THIS
		// session, or the click lands on someone else's tab.
		{"different session", "● Pica — sat-monitor", "MP", false},
		{"session is a prefix of another", "● MPX — other", "MP", false},
		{"substring is not a match", "● XMP — other", "MP", false},
		{"empty session", "● MP — multipilot", "", false},
	}
	for _, c := range cases {
		if got := TitleMatchesSession(c.title, c.session); got != c.want {
			t.Errorf("%s: TitleMatchesSession(%q, %q) = %v, want %v", c.name, c.title, c.session, got, c.want)
		}
	}
}

func TestStripDecoration(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"● MP — multipilot", "MP — multipilot"},
		{"🔔 dev-workspace", "dev-workspace"},
		{"MP", "MP"},
		{"  ● ● MP", "MP"}, // repeated decoration is still decoration
		{"", ""},
		{"●", ""}, // nothing but decoration leaves nothing to match
	} {
		if got := StripDecoration(c.in); got != c.want {
			t.Errorf("StripDecoration(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The session name itself may contain punctuation — only the LEADING decoration goes.
func TestStripDecorationKeepsInnerPunctuation(t *testing.T) {
	if got := StripDecoration("● ccy.dev — web"); got != "ccy.dev — web" {
		t.Errorf("got %q", got)
	}
	if got := StripDecoration("2fa-service — api"); got != "2fa-service — api" {
		t.Errorf("a name starting with a digit must be untouched, got %q", got)
	}
}
