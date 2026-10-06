// Package hostinfo describes the machine gtmux serve runs on: its names, operating
// system, hardware and tool versions. The phone shows it for each paired Mac (its server
// list and server details), so a Mac the user renamed on the phone, or one of several
// similar Macs, can still be told apart by what it actually is. It is owner-only on the
// wire (GET /api/host).
//
// Everything here is a snapshot taken on first use and cached for the life of the serve
// process: it rarely changes (a renamed host or an OS update shows after serve restarts).
// On macOS each value comes from an external command limited to 2s, run one after another,
// so the first request may wait on them; Linux reads os-release and /proc. Later requests
// read the cache.
package hostinfo

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Info is what the phone shows. OS, Arch and Cores are always set (OS from the platform,
// the other two from the Go runtime); any other field may be empty where the platform,
// or a probe that failed, does not offer it, and the phone leaves an empty field out.
type Info struct {
	Hostname     string `json:"hostname"`
	ComputerName string `json:"computer_name,omitempty"` // macOS "Computer Name" (System Settings)
	OS           string `json:"os"`                      // "macOS", "Linux", else GOOS
	OSVersion    string `json:"os_version,omitempty"`    // "26.1"; Linux: PRETTY_NAME
	OSBuild      string `json:"os_build,omitempty"`      // "25B78"
	Arch         string `json:"arch"`                    // the architecture gtmux runs as: "arm64", "amd64"
	CPU          string `json:"cpu,omitempty"`           // "Apple M4 Max"
	Cores        int    `json:"cores"`                   // logical CPUs this process may use (runtime.NumCPU)
	MemoryBytes  int64  `json:"memory_bytes,omitempty"`
	BootTime     int64  `json:"boot_time,omitempty"` // unix seconds
	Tmux         string `json:"tmux,omitempty"`      // "tmux 3.5a"
}

var (
	once   sync.Once
	cached Info
)

// Get returns the machine's Info, gathered on first use. tmuxBin is the tmux to ask for
// its version ("" skips it).
func Get(tmuxBin string) Info {
	once.Do(func() { cached = gather(tmuxBin) })
	return cached
}

// run is a command's trimmed output, or "" when it fails or takes over two seconds.
var run = func(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gather(tmuxBin string) Info {
	in := Info{Arch: runtime.GOARCH, Cores: runtime.NumCPU()}
	in.Hostname, _ = os.Hostname()
	switch runtime.GOOS {
	case "darwin":
		in.OS = "macOS"
		in.ComputerName = run("scutil", "--get", "ComputerName")
		in.OSVersion = run("sw_vers", "-productVersion")
		in.OSBuild = run("sw_vers", "-buildVersion")
		in.CPU = run("sysctl", "-n", "machdep.cpu.brand_string")
		in.MemoryBytes, _ = strconv.ParseInt(run("sysctl", "-n", "hw.memsize"), 10, 64)
		in.BootTime = parseBootTime(run("sysctl", "-n", "kern.boottime"))
	case "linux":
		in.OS = "Linux"
		// os-release(5): /etc first, /usr/lib when /etc has none.
		for _, p := range []string{"/etc/os-release", "/usr/lib/os-release"} {
			if b, err := os.ReadFile(p); err == nil {
				in.OSVersion = parseOSRelease(string(b))
				break
			}
		}
		if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			in.CPU = parseCPUInfo(string(b))
		}
		if b, err := os.ReadFile("/proc/meminfo"); err == nil {
			in.MemoryBytes = parseMemTotal(string(b))
		}
		if b, err := os.ReadFile("/proc/uptime"); err == nil {
			if f := strings.Fields(string(b)); len(f) > 0 {
				if up, err := strconv.ParseFloat(f[0], 64); err == nil {
					in.BootTime = time.Now().Unix() - int64(up)
				}
			}
		}
	default:
		in.OS = runtime.GOOS
	}
	if tmuxBin != "" {
		in.Tmux = run(tmuxBin, "-V")
	}
	return in
}

var bootSec = regexp.MustCompile(`sec\s*=\s*(\d+)`)

// parseBootTime reads `sysctl -n kern.boottime`: "{ sec = 1759700000, usec = 0 } Mon …".
func parseBootTime(s string) int64 {
	m := bootSec.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	return n
}

// parseOSRelease returns os-release's PRETTY_NAME ("Debian GNU/Linux 12 (bookworm)").
// The file is shell-compatible, so a value may be quoted and escaped the shell's way;
// os-release(5) asks a reader to undo that and never to expand or run anything, so it
// is parsed here as data: no variable is expanded and nothing is sourced.
func parseOSRelease(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "PRETTY_NAME="); ok {
			return shellWord(v)
		}
	}
	return ""
}

// shellWord undoes one shell word's quoting: '…' is literal, "…" drops the backslash
// before " \ $ and `, and outside quotes a backslash escapes any character.
func shellWord(v string) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		switch c := v[i]; c {
		case '\'':
			j := strings.IndexByte(v[i+1:], '\'')
			if j < 0 {
				b.WriteString(v[i+1:])
				return b.String()
			}
			b.WriteString(v[i+1 : i+1+j])
			i += j + 1
		case '"':
			for i++; i < len(v) && v[i] != '"'; i++ {
				if v[i] == '\\' && i+1 < len(v) && strings.IndexByte("\"\\$`", v[i+1]) >= 0 {
					i++
				}
				b.WriteByte(v[i])
			}
		case '\\':
			if i+1 < len(v) {
				i++
				b.WriteByte(v[i])
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// parseCPUInfo returns the first "model name" in /proc/cpuinfo.
func parseCPUInfo(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if ok && strings.TrimSpace(k) == "model name" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// parseMemTotal returns /proc/meminfo's MemTotal in bytes ("MemTotal: 16316412 kB").
func parseMemTotal(s string) int64 {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "MemTotal:"); ok {
			f := strings.Fields(v)
			if len(f) > 0 {
				n, _ := strconv.ParseInt(f[0], 10, 64)
				return n * 1024
			}
		}
	}
	return 0
}
