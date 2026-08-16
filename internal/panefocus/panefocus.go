// Package panefocus is the local "jump to a tmux pane" primitive: select the
// pane's window+pane in tmux and bring its terminal tab forward. It depends only
// on leaf packages (tmux, terminal), so the CLI focus command, the watch TUI, the
// remote server, and the HQ supervisor can all jump a pane without a cross-cycle.
package panefocus

import (
	"fmt"
	"regexp"
	"strings"

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
		bringForward(sess)
	}
}

// bringForward puts a session on screen — by focusing the tab that shows it, or by
// OPENING one when nothing does.
//
// A session with no attached client has no tab to focus, so the old code did the tmux
// select (invisible to anyone) and then searched the terminal's tabs for a title that
// could not be there. The click did nothing at all, and there was nothing to see: from
// the user's side "jump" looked broken. Measured on a real fleet: `disk-drift-triage`,
// spawned with `--headless` (which opens no tab by design), sat at `session_attached=0`
// while its neighbours were 1.
//
// Note the test is ATTACHMENT, not how the session was started. `restart-feed` carries
// the same `⌁` headless marker in its window name and IS attached — someone opened it
// later — and jumping to it works. The marker records how a session began; only the
// client count says whether it is on screen now.
func bringForward(sess string) {
	// Resolve the terminal that hosts THIS session (not a global guess), so a session in
	// iTerm2 focuses iTerm2 even when other sessions are in Ghostty.
	term := terminal.ForSession(sess)
	if Attached(sess) {
		term.FocusTab(sess)
		return
	}
	// Nothing is showing it: open a tab that attaches. This is the same call `gtmux new`
	// and `restore` make, and it is the only way to see a detached session at all.
	_, _ = term.SpawnTabs([]string{sess}, false)
}

// Attached reports whether any terminal client is attached to a session — i.e. whether
// there is a window on screen to jump to. Exported because the radar surfaces it: a row
// you cannot jump to should say so before you click it.
func Attached(sess string) bool {
	if sess == "" || tmux.Bin == "" {
		return false
	}
	out, err := tmux.Run("display-message", "-p", "-t", sess, "#{session_attached}")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != "0" && strings.TrimSpace(out) != ""
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
