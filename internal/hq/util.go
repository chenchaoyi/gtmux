package hq

import (
	"os"
	"strings"
)

// fileExists reports whether a path exists (a small local copy of the same
// os.Stat helper app uses — a leaf-level primitive, duplicated rather than shared
// to keep the hq package from importing the command layer).
func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// isShellCommand reports whether name is a bare login/interactive shell (so a
// pane running only this has no agent yet).
func isShellCommand(name string) bool {
	switch strings.TrimPrefix(name, "-") {
	case "bash", "zsh", "fish", "sh", "dash", "tcsh", "ksh":
		return true
	}
	return false
}

// newSessionArgs builds the tmux new-session argv for a detached, named session.
func newSessionArgs(name string) []string {
	args := []string{"new-session", "-d", "-P", "-F", "#{session_name}"}
	if name != "" {
		args = append(args, "-s", name)
	}
	return args
}
