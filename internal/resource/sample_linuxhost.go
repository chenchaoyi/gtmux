package resource

import (
	"math"
	"strconv"
	"strings"
)

// Linux memory pressure is available/total RAM, not macOS kernel pressure.
// Missing MemAvailable (old kernels) is unknown; free RAM alone is misleading.
func linuxMachine(path string, memory, load []byte, cpus int) Machine {
	m := Machine{NCPU: cpus}
	m.DiskFreeGB, m.DiskUsePct, m.Read.Disk = diskFreePortable(path)
	values := map[string]uint64{}
	for _, line := range strings.Split(string(memory), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || f[2] != "kB" {
			continue
		}
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err == nil {
			values[strings.TrimSuffix(f[0], ":")] = n
		}
	}
	total, available := values["MemTotal"], values["MemAvailable"]
	_, hasAvailable := values["MemAvailable"]
	if total > 0 && hasAvailable && available <= total {
		m.MemFreePct = int(float64(available) / float64(total) * 100)
		m.MemTier = "normal"
		if m.MemFreePct < 5 {
			m.MemTier = "critical"
		} else if m.MemFreePct < 10 {
			m.MemTier = "warn"
		}
		m.Read.Memory = true
	}
	if fields := strings.Fields(string(load)); len(fields) > 0 && cpus > 0 {
		if n, err := strconv.ParseFloat(fields[0], 64); err == nil && n >= 0 && !math.IsNaN(n) && !math.IsInf(n, 0) {
			m.LoadRatio, m.Read.Load = n/float64(cpus), true
		}
	}
	m.Read.BatteryNotApplicable = true
	return m
}
