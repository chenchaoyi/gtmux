package resource

import (
	"os/exec"
	"strconv"
	"strings"
)

// sampleMachine gathers the whole-machine numbers (best-effort; a missing source
// leaves its field zero/"" so the rest still works).
func sampleMachine() Machine {
	m := Machine{NCPU: ncpu()}
	m.DiskFreeGB, m.DiskUsePct = diskFree("/")
	m.MemFreePct = memFreePct()
	m.MemTier = memPressureTier()
	if la := loadavg1(); la > 0 && m.NCPU > 0 {
		m.LoadRatio = la / float64(m.NCPU)
	}
	return m
}

// diskFree parses `df -g <path>`: the Available (GB) + Capacity (%). 0,0 on error.
func diskFree(path string) (freeGB, usePct int) {
	out, err := exec.Command("df", "-g", path).Output()
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, 0
	}
	f := strings.Fields(lines[len(lines)-1])
	// Filesystem 1G-blocks Used Available Capacity … → Available=f[3], Capacity=f[4]
	if len(f) < 5 {
		return 0, 0
	}
	freeGB, _ = strconv.Atoi(f[3])
	usePct, _ = strconv.Atoi(strings.TrimSuffix(f[4], "%"))
	return freeGB, usePct
}

// memPressureTier reads the kernel memory-pressure level (macOS): sysctl
// kern.memorystatus_vm_pressure_level → 1 normal, 2 warn, 4 critical. "" when
// unavailable (e.g. Linux — memFreePct still works there).
func memPressureTier() string {
	out, err := exec.Command("sysctl", "-n", "kern.memorystatus_vm_pressure_level").Output()
	if err != nil {
		return ""
	}
	switch strings.TrimSpace(string(out)) {
	case "4":
		return "critical"
	case "2":
		return "warn"
	case "1":
		return "normal"
	default:
		return ""
	}
}

// memFreePct parses `memory_pressure -Q`'s "System-wide memory free percentage: N%".
func memFreePct() int {
	out, err := exec.Command("memory_pressure", "-Q").Output()
	if err != nil {
		return 0
	}
	for _, ln := range strings.Split(string(out), "\n") {
		if i := strings.LastIndex(ln, ": "); i > 0 && strings.Contains(ln, "free percentage") {
			n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(ln[i+2:]), "%"))
			return n
		}
	}
	return 0
}

// ncpu is the core count (sysctl hw.ncpu).
func ncpu() int {
	out, err := exec.Command("sysctl", "-n", "hw.ncpu").Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

// loadavg1 is the 1-minute load average (sysctl vm.loadavg → "{ 5.97 6.35 6.33 }").
func loadavg1() float64 {
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0
	}
	f := strings.Fields(strings.Trim(strings.TrimSpace(string(out)), "{} "))
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(f[0], 64)
	return v
}

// proc is one row of the ps snapshot.
type proc struct {
	pid, ppid, rssKB int
	cpu              float64
	comm             string
}

// sampleProcs takes one `ps -axo pid,ppid,rss,pcpu,comm` snapshot.
func sampleProcs() []proc {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,rss=,pcpu=,comm=").Output()
	if err != nil {
		return nil
	}
	var ps []proc
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) < 5 {
			continue
		}
		pid, _ := strconv.Atoi(f[0])
		ppid, _ := strconv.Atoi(f[1])
		rss, _ := strconv.Atoi(f[2])
		cpu, _ := strconv.ParseFloat(f[3], 64)
		ps = append(ps, proc{pid: pid, ppid: ppid, rssKB: rss, cpu: cpu, comm: strings.Join(f[4:], " ")})
	}
	return ps
}
