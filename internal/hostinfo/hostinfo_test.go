package hostinfo

import (
	"runtime"
	"testing"
)

func TestParsers(t *testing.T) {
	if got := parseBootTime("{ sec = 1759700000, usec = 123456 } Mon Oct  6 03:00:00 2026"); got != 1759700000 {
		t.Errorf("boot time %d", got)
	}
	if parseBootTime("garbage") != 0 {
		t.Error("garbage boot time")
	}
	if got := parseOSRelease("NAME=\"Debian GNU/Linux\"\nPRETTY_NAME=\"Debian GNU/Linux 12 (bookworm)\"\n"); got != "Debian GNU/Linux 12 (bookworm)" {
		t.Errorf("os-release %q", got)
	}
	if got := parseCPUInfo("processor\t: 0\nmodel name\t: AMD EPYC 7B13\n"); got != "AMD EPYC 7B13" {
		t.Errorf("cpuinfo %q", got)
	}
	if got := parseMemTotal("MemTotal:       16316412 kB\nMemFree: 1 kB\n"); got != 16316412*1024 {
		t.Errorf("meminfo %d", got)
	}
}

// gather fills what this platform offers, from stubbed commands; nothing real is run.
func TestGatherUsesTheCommandsOutput(t *testing.T) {
	prev := run
	t.Cleanup(func() { run = prev })
	run = func(name string, args ...string) string {
		key := name
		if len(args) > 0 {
			key += " " + args[len(args)-1]
		}
		return map[string]string{
			"scutil ComputerName":             "Studio",
			"sw_vers -productVersion":         "26.1",
			"sw_vers -buildVersion":           "25B78",
			"sysctl machdep.cpu.brand_string": "Apple M4 Max",
			"sysctl hw.memsize":               "68719476736",
			"sysctl kern.boottime":            "{ sec = 1759700000, usec = 0 } x",
			"/usr/bin/tmux -V":                "tmux 3.5a",
		}[key]
	}
	in := gather("/usr/bin/tmux")
	if in.Hostname == "" || in.Arch != runtime.GOARCH || in.Cores != runtime.NumCPU() || in.Tmux != "tmux 3.5a" {
		t.Fatalf("common fields: %+v", in)
	}
	if runtime.GOOS == "darwin" {
		want := Info{Hostname: in.Hostname, ComputerName: "Studio", OS: "macOS", OSVersion: "26.1", OSBuild: "25B78",
			Arch: in.Arch, CPU: "Apple M4 Max", Cores: in.Cores, MemoryBytes: 68719476736, BootTime: 1759700000, Tmux: "tmux 3.5a"}
		if in != want {
			t.Fatalf("macOS:\n got %+v\nwant %+v", in, want)
		}
	}
}

// os-release values are shell words: escapes are undone, nothing is expanded (%12's
// review of #1429 found `\"` and `\$` passed through with their backslashes).
func TestOSReleaseIsParsedAsData(t *testing.T) {
	for in, want := range map[string]string{
		`PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"`: "Debian GNU/Linux 12 (bookworm)",
		`PRETTY_NAME="Audit \"Blue\" GNU/Linux"`:       `Audit "Blue" GNU/Linux`,
		`PRETTY_NAME="Audit \$HOME Linux"`:             "Audit $HOME Linux",
		`PRETTY_NAME="Back\\slash \n kept"`:            `Back\slash \n kept`,
		`PRETTY_NAME='Single "quoted" \$ literal'`:     `Single "quoted" \$ literal`,
		`PRETTY_NAME=Plain\ Word`:                      "Plain Word",
		`PRETTY_NAME="Unclosed`:                        "Unclosed",
		"  PRETTY_NAME=\"Indented\"":                   "Indented",
	} {
		if got := parseOSRelease("NAME=x\n" + in + "\nID=y\n"); got != want {
			t.Errorf("%s → %q, want %q", in, got, want)
		}
	}
}
