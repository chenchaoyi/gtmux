package resource

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeTools puts stand-ins for the sampling commands alone on PATH, and returns their
// directory. vm.loadavg and pmset run whatever $FAKE_LOADAVG and $FAKE_PMSET say, so one
// set of scripts serves every case (macOS scans each new executable on its first run,
// which costs a fraction of a second apiece). sysctl gets the OID as $2.
func fakeTools(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	tools := map[string]string{
		"df": `echo "Filesystem 1G-blocks Used Available Capacity iused ifree %iused Mounted on"
echo "/dev/disk3s5 460 420 40 92% 1 1 1% /System/Volumes/Data"`,
		"sysctl": `case "$2" in
hw.ncpu) echo 4 ;;
kern.memorystatus_vm_pressure_level) echo 1 ;;
vm.loadavg) eval "$FAKE_LOADAVG" ;;
*) exit 1 ;;
esac`,
		"memory_pressure": `echo "System-wide memory free percentage: 40%"`,
		"pmset":           `eval "$FAKE_PMSET"`,
	}
	for name, body := range tools {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return dir
}

func fakeAnswers(t *testing.T, loadavg, pmset string) {
	t.Setenv("FAKE_LOADAVG", loadavg)
	t.Setenv("FAKE_PMSET", pmset)
}

const (
	loadOK     = `echo "{ 0.00 0.10 0.20 }"`
	pmsetDesk  = `printf "Now drawing from 'AC Power'\n"`
	pmsetBatt8 = `printf "Now drawing from 'Battery Power'\n -InternalBattery-0 (id=7)\t8%%; discharging; 0:23 remaining present: true\n"`
)

// Which sources gave a reading is recorded per source, so a failed or unreadable answer
// is never taken for a measured zero (%12, 2026-10-06): a core count does not make a load
// average, and pmset exiting 0 does not make its answer a reading. A real 0.00 load and a
// desktop with no battery line are readings.
func TestSampleRecordsWhichSourcesGaveAReading(t *testing.T) {
	fakeTools(t)
	for _, tc := range []struct {
		name           string
		loadavg, pmset string
		load, battery  bool
	}{
		{"all read; a real 0.00 load; a desktop", loadOK, pmsetDesk, true, true},
		{"all read; a laptop at 8%", loadOK, pmsetBatt8, true, true},
		{"vm.loadavg fails", `exit 1`, pmsetDesk, false, true},
		{"vm.loadavg prints no number", `echo not-a-load`, pmsetDesk, false, true},
		{"pmset answers nothing", loadOK, `exit 0`, true, false},
		{"pmset's charge is unreadable", loadOK,
			`printf "Now drawing from 'Battery Power'\n -InternalBattery-0 (id=7)\t??%%; discharging\n"`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeAnswers(t, tc.loadavg, tc.pmset)
			m := sampleMachine()
			if !m.Read.Disk || !m.Read.Memory {
				t.Fatalf("disk/memory should read from the stand-ins: %+v", m.Read)
			}
			if m.Read.Load != tc.load || m.Read.Battery != tc.battery {
				t.Fatalf("Read = %+v, want load %v battery %v", m.Read, tc.load, tc.battery)
			}
			if m.Complete() != (tc.load && tc.battery) {
				t.Fatalf("Complete() = %v with %+v", m.Complete(), m.Read)
			}
			if !tc.battery && m.Battery != nil {
				t.Errorf("an unreadable pmset answer should omit the battery, got %+v", m.Battery)
			}
		})
	}
}

func TestSampleWithAFailedDfHasNoDiskReading(t *testing.T) {
	dir := fakeTools(t)
	fakeAnswers(t, loadOK, pmsetDesk)
	if err := os.WriteFile(filepath.Join(dir, "df"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if m := sampleMachine(); m.Read.Disk || m.Complete() {
		t.Fatalf("a failed df must not read as a disk reading: %+v", m.Read)
	}
}
