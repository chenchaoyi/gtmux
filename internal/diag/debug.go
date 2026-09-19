package diag

import (
	"os"
	"strings"
)

// debugOn reports whether a component's debug entries are written. One switch,
// GTMUX_DEBUG, names components ("serve,tunnel") or says "all"; the three variables that
// each component grew on its own stay as aliases, so a habit or a runbook that uses them
// keeps working.
func debugOn(component string) bool {
	if v := os.Getenv("GTMUX_DEBUG"); v != "" {
		for _, c := range strings.Split(v, ",") {
			c = strings.TrimSpace(c)
			if c == "all" || c == "1" || c == component {
				return true
			}
		}
	}
	alias := map[string]string{"hook": "GTMUX_HOOK_DEBUG", "tunnel": "GTMUX_TUNNEL_DEBUG", "menubar": "GTMUXBAR_DEBUG"}
	if name, ok := alias[component]; ok && os.Getenv(name) != "" {
		return true
	}
	return false
}
