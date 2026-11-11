package state

import "testing"

func TestCSR005RepeatedPollPreservesObservedCPUWork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const pane = "%synthetic"
	if PaneCPUWorking(pane, 10, 100) {
		t.Fatal("first observation invented CPU activity")
	}
	if !PaneCPUWorking(pane, 12, 102) {
		t.Fatal("actual one-core rise was lost")
	}
	for i := 0; i < 3; i++ {
		if !PaneCPUWorking(pane, 12, 102) {
			t.Fatalf("same-second reader %d erased observed CPU work", i)
		}
	}
	if !PaneCPUWorking(pane, 11.999999999, 102) {
		t.Fatal("floating-point sum noise looked like a restarted CPU counter")
	}
	if !PaneCPUWorking(pane, 12, 105) {
		t.Fatal("repeated reads changed the original smoothing window")
	}
	if PaneCPUWorking(pane, 12, 106) {
		t.Fatal("same-second readers extended the four-second grace")
	}
}

func TestCSR005RepeatedPollRequiresValidPriorEvidence(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		now, previous, active int64
		before, current       float64
	}{
		{"no baseline", 102, 102, 102, -1, 12},
		{"no previous activity", 102, 102, 0, 12, 12},
		{"backward clock", 101, 102, 102, 12, 12},
		{"restarted CPU counter", 102, 102, 102, 12, 1},
		{"expired activity", 106, 106, 102, 12, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, working := cpuWorking(tc.now, tc.previous, tc.active, tc.before, tc.current); working {
				t.Fatal("unusable or expired evidence granted CPU work")
			}
		})
	}
}
