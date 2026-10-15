package resource

import "testing"

func TestLinuxResourcesDistinguishMissingSourcesAndNoBattery(t *testing.T) {
	for _, tc := range []struct {
		name, mem, load, tier string
		complete              bool
	}{
		{"healthy", "MemTotal: 1000 kB\nMemAvailable: 700 kB\n", "0.00 1 2", "normal", true},
		{"critical zero is real", "MemTotal: 1000 kB\nMemAvailable: 0 kB\n", "4.00 1 2", "critical", true},
		{"warning", "MemTotal: 1000 kB\nMemAvailable: 70 kB\n", "1.00", "warn", true},
		{"missing available", "MemTotal: 1000 kB\nMemFree: 700 kB\n", "0.00", "", false},
		{"corrupt memory", "MemTotal: nope kB\nMemAvailable: 0 kB\n", "0.00", "", false},
		{"impossible memory", "MemTotal: 1000 kB\nMemAvailable: 2000 kB\n", "0.00", "", false},
		{"missing load", "MemTotal: 1000 kB\nMemAvailable: 700 kB\n", "", "normal", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := linuxMachine(t.TempDir(), []byte(tc.mem), []byte(tc.load), 4)
			if m.MemTier != tc.tier || m.Complete() != tc.complete || !m.Read.BatteryNotApplicable || m.Read.Battery || m.Battery != nil {
				t.Fatalf("unexpected Linux sample: %+v", m)
			}
		})
	}
	m := linuxMachine("/definitely/missing/gtmux-volume", []byte("MemTotal: 1000 kB\nMemAvailable: 700 kB\n"), []byte("0.00"), 4)
	if m.Read.Disk || m.Complete() {
		t.Fatal("failed filesystem read cleared an alarm")
	}
}
