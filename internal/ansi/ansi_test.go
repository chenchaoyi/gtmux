package ansi

import "testing"

const esc = "\x1b"

func TestStripDroppingFaint(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain passthrough", "just plain text", "just plain text"},
		{"drops a faint span", "keep " + esc + "[2mghost" + esc + "[0m end", "keep  end"},
		{"faint reset by 22", esc + "[2mghost" + esc + "[22mreal", "real"},
		{"bright color kept", esc + "[38;5;246mdim-color-but-not-faint" + esc + "[39m", "dim-color-but-not-faint"},
		{"faint in a combined SGR", esc + "[1;2mfaint" + esc + "[0mbright", "bright"},
		{"strips OSC hyperlink chrome, keeps label", esc + "]8;;http://x" + esc + "\\label" + esc + "]8;;" + esc + "\\", "label"},
	}
	for _, c := range cases {
		if got := StripDroppingFaint(c.in); got != c.want {
			t.Errorf("%s: StripDroppingFaint(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestStrip_KeepsTextRemovesEscapes(t *testing.T) {
	// Strip (unlike StripDroppingFaint) keeps the faint TEXT — it only removes escapes.
	cases := []struct{ name, in, want string }{
		{"plain passthrough", "hello", "hello"},
		{"removes CSI SGR, keeps faint text", "keep " + esc + "[2mghost" + esc + "[0m end", "keep ghost end"},
		{"removes a color code", esc + "[38;5;153mcolored" + esc + "[0m", "colored"},
		{"strips OSC hyperlink, keeps label", esc + "]8;;http://x" + esc + "\\label" + esc + "]8;;" + esc + "\\", "label"},
	}
	for _, c := range cases {
		if got := Strip(c.in); got != c.want {
			t.Errorf("%s: Strip(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
