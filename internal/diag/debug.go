package diag

import (
	"os"
	"strings"
	"sync"

	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

// debugOn reports whether a component's debug entries are written. One switch names
// components ("serve,tunnel") or says "all": GTMUX_DEBUG for one run, or `debug` in
// config.json for every process, including the launchd ones that no shell variable
// reaches. The three variables that each component grew on its own stay as aliases, so
// a habit or a runbook that uses them keeps working.
func debugOn(component string) bool {
	if names(os.Getenv("GTMUX_DEBUG")).has(component) || names(configDebug()).has(component) {
		return true
	}
	alias := map[string]string{"hook": "GTMUX_HOOK_DEBUG", "tunnel": "GTMUX_TUNNEL_DEBUG", "menubar": "GTMUXBAR_DEBUG"}
	if name, ok := alias[component]; ok && os.Getenv(name) != "" {
		return true
	}
	return false
}

// DebugOn is debugOn for a component that also keeps its own trace (the hook's, the
// tunnel client's), so both follow the same switch.
func DebugOn(component string) bool { return debugOn(component) }

// A switchSet is the components one switch value turns on; "all" and "1" turn on every
// component.
type switchSet map[string]bool

func (s switchSet) has(component string) bool { return s["all"] || s[component] }

func names(v string) switchSet {
	set := switchSet{}
	for _, c := range strings.Split(v, ",") {
		switch c = strings.TrimSpace(c); c {
		case "":
		case "1":
			set["all"] = true
		default:
			set[c] = true
		}
	}
	return set
}

// configDebug reads `debug` from config.json once per process: debug entries are written
// on hot paths, and a value that changes takes effect on the next process, as every
// other setting does.
var configDebug = sync.OnceValue(func() string {
	var c struct {
		Debug string `json:"debug"`
	}
	_ = usercfg.Load(&c)
	return c.Debug
})

// DebugSwitch is the configured `debug` value, for the commands that report what is
// being recorded (`gtmux logs --stats`) and for the menu bar's Diagnostics section.
// Empty means only the ordinary entries are written.
func DebugSwitch() string { return configDebug() }
