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
		pending  int
		want     bool
		reason   string
	}{
		// --- does NOT fire ------------------------------------------------------
		{"rate-limited even with a big volume", now - 3600, 10, 6000, 0, false, ""},
		{"rate-limited even with a full spool", now - 3600, 10, 50, 99, false, ""},
		{"zero-change gate beats volume", now - 8*day, 0, 6000, 0, false, ""},
		{"zero-change gate beats weekly", now - week, 0, 50, 0, false, ""},
		{"just under volume, under weekly → wait", now - 2*day, 3, distillVolumeFloor - 1, 0, false, ""},
		{"just under the weekly clock → wait", now - week + 1, 1, 100, 0, false, ""},
		{"just under the spool floor → wait", now - 2*day, 3, 100, distillSpoolFloor - 1, false, ""},

		// --- DOES fire ----------------------------------------------------------
		{"volume floor fires (busy fleet)", now - 2*day, 3, distillVolumeFloor, 0, true, "volume"},
		{"weekly floor fires (quiet fleet)", now - week, 1, 100, 0, true, "weekly"},
		{"spool floor fires before the week is up", now - 2*day, 3, 100, distillSpoolFloor, true, "spool"},
		{"volume takes precedence over weekly", now - 8*day, 5, 6000, 0, true, "volume"},
		{"spool takes precedence over volume", now - 8*day, 5, 6000, distillSpoolFloor, true, "spool"},

		// A captured lesson IS a change: a quiet period with a full spool must still
		// distil, or `gtmux capture` would write into a queue nothing ever drains.
		{"a full spool satisfies the zero-change gate", now - 2*day, 0, 0, distillSpoolFloor, true, "spool"},
	}
	for _, c := range cases {
		got, reason := shouldDistill(now, c.lastAt, c.notable, c.newCount, c.pending)
		if got != c.want || reason != c.reason {
			t.Errorf("%s: shouldDistill = (%v,%q), want (%v,%q)", c.name, got, reason, c.want, c.reason)
		}
	}
}
