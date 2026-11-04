package agents

import "strings"

// OpenCodeInteractiveArgs keeps management, headless and explicit server recipes
// native. It is shared by launch, restore evidence and the command wrapper.
func OpenCodeInteractiveArgs(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "run", "serve", "service", "api", "acp", "auth", "plugin", "upgrade", "update", "uninstall", "session", "models", "stats", "debug", "reload", "pair":
		return false
	}
	for _, a := range args {
		if a == "--server" || strings.HasPrefix(a, "--server=") || a == "--help" || a == "-h" || a == "--version" || a == "-v" {
			return false
		}
	}
	return true
}
