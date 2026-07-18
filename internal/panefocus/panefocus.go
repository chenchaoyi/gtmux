// Package panefocus is the local "jump to a tmux pane" primitive: select the
// pane's window+pane in tmux and bring its terminal tab forward. It depends only
// on leaf packages (tmux, terminal), so the CLI focus command, the watch TUI, the
// remote server, and the HQ supervisor can all jump a pane without a cross-cycle.
package panefocus

import (
	"fmt"
	"regexp"

	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

var paneIDRe = regexp.MustCompile(`^%[0-9]+`)

// JumpPane selects a pane's window+pane in tmux and brings its terminal tab
// forward (no output). Used by the watch TUI on Enter.
func JumpPane(paneID string) {
	if tmux.Bin == "" || tmux.Display(paneID, "#{pane_id}") == "" {
		return
	}
	sess := tmux.Display(paneID, "#{session_name}")
	if win := tmux.Display(paneID, "#{window_id}"); win != "" {
		tmux.OK("select-window", "-t", win)
	}
	tmux.OK("select-pane", "-t", paneID)
	if sess != "" {
		// Resolve the terminal that hosts THIS session (not a global guess), so a
		// session in iTerm2 focuses iTerm2 even when other sessions are in Ghostty.
		terminal.ForSession(sess).FocusTab(sess)
	}
}

// FocusPaneByID selects an exact tmux pane (%N) — window then pane — and brings
// its terminal tab forward, the same local "jump" the watch TUI does on Enter.
// It injects no input (read-only/no RCE); the remote server calls it for
// POST /api/focus ("when you're back at your desk, you're already on this pane").
// Returns an error if id isn't a pane id or the pane no longer exists.
func FocusPaneByID(id string) error {
	if !paneIDRe.MatchString(id) {
		return fmt.Errorf("not a pane id: %q", id)
	}
	if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
		return fmt.Errorf("pane %s no longer exists", id)
	}
	JumpPane(id)
	return nil
}
