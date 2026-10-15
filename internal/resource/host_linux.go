package resource

import (
	"github.com/chenchaoyi/gtmux/internal/state"
	"os"
	"path/filepath"
	"runtime"
)

func sampleMachine() Machine {
	memory, _ := os.ReadFile("/proc/meminfo")
	load, _ := os.ReadFile("/proc/loadavg")
	path := state.Dir()
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return linuxMachine(path, memory, load, runtime.NumCPU())
}
