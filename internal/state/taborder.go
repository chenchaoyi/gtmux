package state

import (
	"os"
	"path/filepath"
	"strings"
)

// Tab order: the always-on menu-bar app periodically records the live terminal
// tab → session order here (one session name per line, in tab order). `gtmux
// restore` replays it so restored tabs come back in YOUR arrangement instead of
// tmux's alphabetical `list-sessions` order. Plain text, never a screenshot.

// TabOrderPath is where the live tab→session order is recorded.
func TabOrderPath() string { return filepath.Join(Dir(), "tab-order") }

// SaveTabOrder writes the ordered session names (one per line). Empty input is
// ignored (don't clobber a good record with a transient empty read).
func SaveTabOrder(sessions []string) error {
	if len(sessions) == 0 {
		return nil
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(TabOrderPath(), []byte(strings.Join(sessions, "\n")+"\n"), 0o644)
}

// LoadTabOrder returns the recorded session order ("" entries dropped), or nil.
func LoadTabOrder() []string {
	b, err := os.ReadFile(TabOrderPath())
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	return out
}
