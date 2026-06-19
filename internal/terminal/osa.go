package terminal

import (
	"os/exec"
	"strings"
)

// osa runs an AppleScript and returns trimmed stdout. Shared by the AppleScript
// drivers in this package.
func osa(script string) (string, error) {
	out, err := exec.Command("/usr/bin/osascript", "-e", script).Output()
	return strings.TrimSpace(string(out)), err
}

// aplQuote escapes a string for an AppleScript "..." literal.
func aplQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// shellQuote single-quotes a string for the shell (handles spaces etc.).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
