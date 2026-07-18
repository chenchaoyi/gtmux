package hq

import "testing"

func TestShouldDistill(t *testing.T) {
	const now = 2_000_000
	day := int64(distillMinInterval)  // 1 day
	week := int64(distillWeeklyFloor) // 7 days

	cases := []struct {
		name     string
		lastAt   int64
		notable  int
		newCount int
		want     bool
		reason   string
	}{
		{"rate-limited even with a big volume", now - 3600, 10, 6000, false, ""},
		{"zero-change gate beats volume", now - 8*day, 0, 6000, false, ""},
		{"zero-change gate beats weekly", now - week, 0, 50, false, ""},
		{"volume floor fires (busy fleet)", now - 2*day, 3, distillVolumeFloor, true, "volume"},
		{"just under volume, under weekly → wait", now - 2*day, 3, distillVolumeFloor - 1, false, ""},
		{"weekly floor fires (quiet fleet)", now - week, 1, 100, true, "weekly"},
		{"just under the weekly clock → wait", now - week + 1, 1, 100, false, ""},
		{"volume takes precedence over weekly", now - 8*day, 5, 6000, true, "volume"},
	}
	for _, c := range cases {
		got, reason := shouldDistill(now, c.lastAt, c.notable, c.newCount)
		if got != c.want || reason != c.reason {
			t.Errorf("%s: shouldDistill = (%v,%q), want (%v,%q)", c.name, got, reason, c.want, c.reason)
		}
	}
}
