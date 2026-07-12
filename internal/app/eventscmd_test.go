package app

import "testing"

func TestParseSince(t *testing.T) {
	cases := map[string]int64{"": 0, "90s": 90, "10m": 600, "2h": 7200, "45": 45, "bad": 0, "-5m": 0}
	for in, want := range cases {
		if got := parseSince(in); got != want {
			t.Errorf("parseSince(%q) = %d, want %d", in, got, want)
		}
	}
}
