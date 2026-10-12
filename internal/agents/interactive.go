package agents

import (
	"path/filepath"
	"strings"
)

// InteractiveCommand preserves terminal history for direct Codex launches made
// by gtmux. It never rewrites global settings or custom wrapper recipes, and an
// explicit screen choice wins. Callers apply it before appending a session ID.
func InteractiveCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 || filepath.Base(strings.Trim(fields[0], `"'`)) != "codex" ||
		strings.Contains(command, "--no-alt-screen") || strings.Contains(command, "tui.alternate_screen") {
		return command
	}
	for _, field := range fields[1:] {
		if field == "exec" || field == "app-server" || field == "mcp-server" {
			return command
		}
	}
	end := strings.Index(command, fields[0]) + len(fields[0])
	return command[:end] + " --no-alt-screen" + command[end:]
}
