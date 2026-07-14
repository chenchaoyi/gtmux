package i18n

import "testing"

func TestTrAndSetLang(t *testing.T) {
	defer SetLang("en")

	SetLang("en")
	if got := Tr("hello", "你好"); got != "hello" {
		t.Errorf("en Tr = %q, want %q", got, "hello")
	}
	SetLang("zh")
	if got := Tr("hello", "你好"); got != "你好" {
		t.Errorf("zh Tr = %q, want %q", got, "你好")
	}
	// Unknown values are ignored — language stays put.
	SetLang("fr")
	if Lang() != "zh" {
		t.Errorf("after SetLang(fr) Lang = %q, want zh (unchanged)", Lang())
	}
}

func TestDispWidth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"你好", 4},      // two CJK runes, width 2 each
		{"a你b", 4},     // 1 + 2 + 1
		{"✳ idle", 6},  // ✳ is narrow here (not in a wide block)
		{"日本語テスト", 12}, // 6 wide runes
	}
	for _, c := range cases {
		if got := DispWidth(c.in); got != c.want {
			t.Errorf("DispWidth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestPadRight(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"ab", 5, "ab   "},      // pad to 5 columns
		{"你好", 5, "你好 "},        // display width 4 → one trailing space
		{"abcdef", 3, "abcdef"}, // already wider than the field → unchanged
		{"", 2, "  "},
	}
	for _, c := range cases {
		got := PadRight(c.in, c.width)
		if got != c.want {
			t.Errorf("PadRight(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
		// Padded result must reach the requested display width (when padding applies).
		if DispWidth(c.in) < c.width && DispWidth(got) != c.width {
			t.Errorf("PadRight(%q, %d) display width = %d, want %d", c.in, c.width, DispWidth(got), c.width)
		}
	}
}

func TestPl(t *testing.T) {
	defer SetLang("en")

	SetLang("en")
	if got := Pl(1, "window"); got != "1 window" {
		t.Errorf("en Pl(1) = %q, want %q", got, "1 window")
	}
	if got := Pl(3, "window"); got != "3 windows" {
		t.Errorf("en Pl(3) = %q, want %q", got, "3 windows")
	}
	SetLang("zh")
	if got := Pl(3, "window"); got != "3 window" {
		t.Errorf("zh Pl(3) = %q, want %q (no pluralization)", got, "3 window")
	}
}

func TestPadLeft(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"ab", 5, "   ab"},
		{"你好", 5, " 你好"},
		{"abcdef", 3, "abcdef"}, // already wider than the field → unchanged
		{"", 2, "  "},
	}
	for _, c := range cases {
		got := PadLeft(c.in, c.width)
		if got != c.want {
			t.Errorf("PadLeft(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
		if DispWidth(c.in) < c.width && DispWidth(got) != c.width {
			t.Errorf("PadLeft(%q, %d) display width = %d, want %d", c.in, c.width, DispWidth(got), c.width)
		}
	}
}

func TestTruncDisp(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"", 5, ""},
		{"abc", 5, "abc"},      // fits, unchanged
		{"abcdef", 5, "abcd…"}, // cut + ellipsis, total width 5
		{"你好世界", 5, "你好…"},     // wide runes: 2+2 fits in 4, +… = 5
		{"abcdef", 0, ""},      // no room at all
	}
	for _, c := range cases {
		got := TruncDisp(c.in, c.width)
		if got != c.want {
			t.Errorf("TruncDisp(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
		if got != c.in && DispWidth(got) > c.width {
			t.Errorf("TruncDisp(%q, %d) display width = %d, exceeds %d", c.in, c.width, DispWidth(got), c.width)
		}
	}
}
