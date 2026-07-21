package connect

import "testing"

func TestParsePaneChoice(t *testing.T) {
	const n = 3
	cases := []struct {
		in         string
		wantIdx    int
		wantCancel bool
		wantOK     bool
	}{
		{"", 0, false, true},     // Enter → default row 0
		{"\n", 0, false, true},   // bare newline → default
		{"1", 0, false, true},    // first
		{"3", 2, false, true},    // last
		{"  2 ", 1, false, true}, // whitespace tolerated
		{"q", 0, true, true},     // cancel
		{"Q", 0, true, true},     // cancel (upper)
		{"quit", 0, true, true},  // cancel (word)
		{"\x1b", 0, true, true},  // leading ESC → cancel
		{"0", 0, false, false},   // out of range (low)
		{"4", 0, false, false},   // out of range (high)
		{"x", 0, false, false},   // non-numeric
		{"1x", 0, false, false},  // trailing garbage
	}
	for _, c := range cases {
		idx, cancel, ok := parsePaneChoice(c.in, n)
		if idx != c.wantIdx || cancel != c.wantCancel || ok != c.wantOK {
			t.Errorf("parsePaneChoice(%q,%d) = (%d,%v,%v); want (%d,%v,%v)",
				c.in, n, idx, cancel, ok, c.wantIdx, c.wantCancel, c.wantOK)
		}
	}
}

func TestFormatPaneChoice(t *testing.T) {
	cases := []struct {
		a    Agent
		want string
	}{
		{Agent{Session: "work", Agent: "claude", Status: "waiting", Task: "run tests"},
			"work · claude · waiting  run tests"},
		{Agent{Session: "work", Agent: "claude", Status: "idle"},
			"work · claude · idle"},
		{Agent{PaneID: "%7"}, "%7"}, // no descriptive fields → pane id
		{Agent{Session: "s", Task: "x"}, "s  x"},
	}
	for _, c := range cases {
		if got := formatPaneChoice(c.a); got != c.want {
			t.Errorf("formatPaneChoice(%+v) = %q; want %q", c.a, got, c.want)
		}
	}
}

func TestFormatPaneChoiceTruncatesLongTask(t *testing.T) {
	long := ""
	for i := 0; i < 80; i++ {
		long += "x"
	}
	got := formatPaneChoice(Agent{Session: "s", Task: long})
	// 60-rune cap: 57 runes + ellipsis, prefixed by "s  ".
	if r := []rune(got); len(r) != len("s  ")+58 { // 57 + "…"
		t.Errorf("truncated length = %d runes; want %d", len(r), len("s  ")+58)
	}
	if got[len(got)-3:] != "…" {
		t.Errorf("want trailing ellipsis, got %q", got)
	}
}
