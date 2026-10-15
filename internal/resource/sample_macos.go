package resource

import (
	"os/exec"
	"strconv"
	"strings"
)

// sampleMachine gathers the whole-machine numbers (best-effort; a missing source
// leaves its field zero/"" so the rest still works).
func sampleDarwinMachine() Machine {
	m := Machine{NCPU: ncpu()}
	var diskOK bool
	m.DiskFreeGB, m.DiskUsePct, diskOK = diskFree(diskPath())
	m.MemFreePct = memFreePct()
	m.MemTier = memPressureTier()
	la, loadOK := loadavg1()
	loadOK = loadOK && m.NCPU > 0
	if loadOK {
		m.LoadRatio = la / float64(m.NCPU)
	}
	m.Battery = sampleBattery()
	m.Read = Readings{Disk: diskOK, Memory: m.MemTier != "", Load: loadOK, Battery: m.Battery != nil}
	return m
}

// sampleBattery reads `pmset -g batt` (macOS). nil on Linux / no pmset (a battery-less
// host reads as "no power concern"). Pure parsing is split into parseBattery for tests.
func sampleBattery() *Battery {
	out, err := exec.Command("pmset", "-g", "batt").Output()
	if err != nil {
		return nil
	}
	b, ok := parseBattery(string(out))
	if !ok {
		return nil // an answer with nothing readable in it is no answer
	}
	return b
}

// diskPath is the volume to sample for disk pressure. On macOS the writable user
// data lives on the DATA volume (/System/Volumes/Data); the root `/` is the tiny
// read-only system volume, so `df /`'s Capacity% reflects that near-empty volume
// (24% here) and badly understates real usage while the Data volume sits at 92%.
// (The Available GB is shared across the APFS container, so it's the same either
// way — only Capacity% was lying.) Fall back to `/` where the data volume is absent
// (Linux, older macOS).
func diskPath() string {
	if _, err := exec.Command("df", "-g", "/System/Volumes/Data").Output(); err == nil {
		return "/System/Volumes/Data"
	}
	return "/"
}

// diskFree parses `df -g <path>`: the Available (GB) + Capacity (%). ok is false on
// error, with 0, 0.
func diskFree(path string) (freeGB, usePct int, ok bool) {
	out, err := exec.Command("df", "-g", path).Output()
	if err != nil {
		return 0, 0, false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, 0, false
	}
	f := strings.Fields(lines[len(lines)-1])
	// Filesystem 1G-blocks Used Available Capacity … → Available=f[3], Capacity=f[4]
	if len(f) < 5 {
		return 0, 0, false
	}
	freeGB, err1 := strconv.Atoi(f[3])
	usePct, err2 := strconv.Atoi(strings.TrimSuffix(f[4], "%"))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return freeGB, usePct, true
}

// memPressureTier reads the kernel memory-pressure level (macOS): sysctl
// kern.memorystatus_vm_pressure_level → 1 normal, 2 warn, 4 critical. "" when
// unavailable.
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

// loadavg1 is the 1-minute load average (sysctl vm.loadavg → "{ 5.97 6.35 6.33 }"). ok
// is false when it could not be read; a real 0.00 is a reading.
func loadavg1() (float64, bool) {
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0, false
	}
	f := strings.Fields(strings.Trim(strings.TrimSpace(string(out)), "{} "))
	if len(f) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
