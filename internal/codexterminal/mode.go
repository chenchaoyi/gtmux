package codexterminal

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// InteractiveMode rejects known headless modes even when someone launched the
// backend itself in a foreground PTY. Arguments are inspected transiently and
// never returned, logged or persisted. Flat ps output can be ambiguous, so a
// headless token inside positional text conservatively defers recovery too.
func InteractiveMode(pid int) bool {
	if pid <= 1 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return interactiveCommand(run(ctx, "ps", "-ww", "-p", strconv.Itoa(pid), "-o", "command="))
}

// ManagedBackend recognizes the shared daemon, not a terminal frontend. Unknown
// commands fail open for callers deciding whether to retain a visible record.
// Arguments remain transient, as in InteractiveMode.
func ManagedBackend(pid int) bool {
	if pid <= 1 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return managedBackendCommand(run(ctx, "ps", "-ww", "-p", strconv.Itoa(pid), "-o", "command="))
}

func managedBackendCommand(raw string) bool {
	if strings.ContainsAny(raw, "\r\n\x00") {
		return false
	}
	args := strings.Fields(raw)
	if len(args) < 3 || filepath.Base(args[0]) != "codex" || args[1] != "app-server" {
		return false
	}
	managed := false
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--managed-daemon":
			managed = true
		case "--analytics-default-enabled":
		case "--listen":
			i++
			if i >= len(args) || !strings.HasPrefix(args[i], "unix://") {
				return false
			}
		default:
			// Flat ps cannot disambiguate quoted configuration or positional text.
			// Admit only the observed daemon flags, never a token inside a value.
			return false
		}
	}
	return managed
}

func interactiveCommand(raw string) bool {
	if strings.ContainsAny(raw, "\r\n\x00") {
		return false
	}
	args := strings.Fields(raw)
	if len(args) == 0 || filepath.Base(args[0]) != "codex" {
		return false
	}
	for _, arg := range args[1:] {
		switch strings.Trim(arg, "\"'") {
		case "app-server", "mcp-server", "exec", "e":
			return false
		}
	}
	return true
}
