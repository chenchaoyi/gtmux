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
