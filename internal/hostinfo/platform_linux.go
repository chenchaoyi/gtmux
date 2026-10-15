package hostinfo

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func gatherPlatform(in *Info) {
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
}
