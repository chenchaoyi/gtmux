package agents

import "strings"

// BridgeCommand wraps only a bare, direct interactive Codex recipe. Explicit
// executable paths, opaque wrappers, environment recipes and headless modes
// retain their existing launch semantics.
func BridgeCommand(command, executable string) string {
	f := strings.Fields(command)
	if len(f) == 0 || (f[0] != "codex" && f[0] != "'codex'") || executable == "" {
		return command
	}
	for _, field := range f[1:] {
		if field == "exec" || field == "e" || field == "app-server" || field == "mcp-server" {
			return command
		}
	}
	index := strings.Index(command, f[0])
	return command[:index] + "'" + strings.ReplaceAll(executable, "'", `'\''`) + "' codex --" + command[index+len(f[0]):]
}
