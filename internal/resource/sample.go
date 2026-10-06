package resource

import (
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// sampleMachine gathers the whole-machine numbers (best-effort; a missing source
// leaves its field zero/"" so the rest still works).
func sampleMachine() Machine {
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

// parseBattery turns `pmset -g batt` text into a Battery. Shape:
//
//	Now drawing from 'AC Power'
//	 -InternalBattery-0 (id=…)\t100%; charged; 0:00 remaining present: true
//
// Returns Present=false (but OnAC from line 1) when there is no internal-battery line
// (a desktop). ok is false when the text is not a reading at all: no "drawing from"
// line, or a battery line whose charge cannot be read. Either used to come back as
// "no battery", and a queued battery alarm was dropped on it (%12, 2026-10-06).
func parseBattery(text string) (b *Battery, ok bool) {
	b = &Battery{OnAC: strings.Contains(text, "'AC Power'")}
	if !strings.Contains(text, "drawing from") {
		return nil, false
	}
	batteryLine := false
	for _, ln := range strings.Split(text, "\n") {
		if !strings.Contains(ln, "InternalBattery") {
			continue
		}
		batteryLine = true
		pctIdx := strings.IndexByte(ln, '%')
		if pctIdx < 0 {
			continue
		}
		// the run of digits ending right before '%' is the charge
		j := pctIdx
		for j > 0 && ln[j-1] >= '0' && ln[j-1] <= '9' {
			j--
		}
		pct, err := strconv.Atoi(ln[j:pctIdx])
		if err != nil {
			continue
		}
		b.Present = true
		b.Percent = pct
		// after "NN%": "; <state>; <H:MM> remaining present: true"
		fields := strings.Split(ln[pctIdx+1:], ";")
		if len(fields) >= 2 {
			b.State = strings.TrimSpace(fields[1])
		}
		if len(fields) >= 3 {
			if tf := strings.Fields(strings.TrimSpace(fields[2])); len(tf) > 0 && strings.Contains(tf[0], ":") && tf[0] != "0:00" {
				b.TimeLeft = tf[0]
			}
		}
		break
	}
	if batteryLine && !b.Present {
		return nil, false
	}
	return b, true
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

// proc is one row of the ps snapshot.
type proc struct {
	pid, ppid, rssKB int
	cpu              float64
	comm             string
}

// boundedPSOutput runs `ps <args>` with a hard timeout and truly ABANDONS it if it
// overruns. A process wedged in uninterruptible kernel sleep (a corp VPN/EDR agent during
// a network switch) makes `ps` unkillable and unreapable, so CommandContext+Cmd.Wait
// would block forever in wait4. We read+Wait in a goroutine and select against a timer:
// on timeout we best-effort Kill and return nil, leaving the goroutine parked (buffered
// send) until the child finally dies. nil on any error or timeout.
func boundedPSOutput(timeout time.Duration, args ...string) []byte {
	cmd := exec.Command("ps", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}
	done := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(stdout)
		_ = cmd.Wait()
		done <- b
	}()
	select {
	case b := <-done:
		return b
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return nil
	}
}

// sampleProcs takes one `ps -axo pid,ppid,rss,pcpu,comm` snapshot, bounded so a wedged
// process can't hang the serve's resource tick (degrades to a nil sample instead).
func sampleProcs() []proc {
	out := boundedPSOutput(4*time.Second, "-axo", "pid=,ppid=,rss=,pcpu=,comm=")
	if out == nil {
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
