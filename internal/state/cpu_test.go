package state

import "testing"

func TestCPUWorking(t *testing.T) {
	cases := []struct {
		name                      string
		now, prevPoll, prevActive int64
		prevCPU, curCPU           float64
		wantActive                int64
		wantWorking               bool
	}{
		{"no baseline", 100, 0, 0, -1, 5, 0, false},
		{"stale baseline", 100, 50, 0, 1.0, 9.0, 0, false},
		{"busy: ~1 core over 2s", 102, 100, 0, 10.0, 12.0, 102, true},
		{"idle: no CPU rise", 102, 100, 0, 10.0, 10.0, 0, false},
		{"below threshold (0.1 core)", 102, 100, 0, 10.0, 10.2, 0, false},
		{"smoothed: active 3s ago, no rise now", 103, 102, 100, 10.0, 10.0, 100, true},
		{"settled: active 5s ago → idle", 105, 104, 100, 10.0, 10.0, 100, false},
		{"cpu reset (process restarted) → not busy", 102, 100, 0, 50.0, 1.0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			active, working := cpuWorking(c.now, c.prevPoll, c.prevActive, c.prevCPU, c.curCPU)
			if active != c.wantActive || working != c.wantWorking {
				t.Fatalf("got (active=%d, working=%v), want (active=%d, working=%v)",
					active, working, c.wantActive, c.wantWorking)
			}
		})
	}
}
