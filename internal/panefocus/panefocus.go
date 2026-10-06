// Package panefocus is the local "jump to a tmux pane" primitive: select the
// pane's window+pane in tmux and bring its terminal tab forward. It depends only
// on leaf packages (tmux, terminal), so the CLI focus command, the watch TUI, the
// remote server, and the HQ supervisor can all jump a pane without a cross-cycle.
package panefocus

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// A tmux pane id is `%` followed by digits and NOTHING else. Anchored at both ends: the
// id arrives from POST /api/focus, so it is remote input, and an unanchored pattern let
// `%12x` through validation to fail later with "pane no longer exists" — an answer about
// the wrong thing. (There is no injection either way: tmux is exec'd with separate
// arguments, never through a shell.)
var paneIDRe = regexp.MustCompile(`^%[0-9]+$`)

// The terminal driver and the attachment check; tests stand in for both, since the
// real ones drive a terminal app over AppleScript.
var (
	terminalFor = terminal.ForSession
	isAttached  = Attached
)

var (
	// ErrNoPane: the id is not a pane id, or the pane no longer exists.
	ErrNoPane = errors.New("no such pane")
	// ErrNoTab: a client is attached to the session, but no terminal tab shows it.
	ErrNoTab = errors.New("no terminal tab is showing the session")
)

// JumpPane selects a pane's window+pane in tmux and brings its terminal tab forward.
// It reports what did not work: a jump that selected the pane in tmux but left the
// terminal where it was used to come back as success, so POST /api/focus answered 200
// for a jump nobody saw (%12, 2026-10-06). The watch TUI ignores the error; the server
// reports it.
func JumpPane(paneID string) error {
	if tmux.Bin == "" || tmux.Display(paneID, "#{pane_id}") == "" {
		return fmt.Errorf("pane %s: %w", paneID, ErrNoPane)
	}
	sess := tmux.Display(paneID, "#{session_name}")
	if win := tmux.Display(paneID, "#{window_id}"); win != "" {
		tmux.OK("select-window", "-t", win)
	}
	if !tmux.OK("select-pane", "-t", paneID) {
		return fmt.Errorf("tmux could not select pane %s", paneID)
	}
	if sess == "" {
		return fmt.Errorf("pane %s has no session to bring forward", paneID)
	}
	_, err := BringForward(sess)
	return err
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
// BringForward is exported because there are TWO jump paths — the TUI/serve one through
// JumpPane, and the CLI's `gtmux focus`, which resolves a session name and owns its own
// error messages. Fixing only the first is exactly what happened: `gtmux focus %30` on a
// real detached session still printed "no tab is showing it, run gtmux restore" after the
// fix was in. One primitive, two callers.
//
// Returns whether it had to OPEN a window, so a caller with a voice can say so.
func BringForward(sess string) (opened bool, err error) {
	// Resolve the terminal that hosts THIS session (not a global guess), so a session in
	// iTerm2 focuses iTerm2 even when other sessions are in Ghostty.
	term := terminalFor(sess)
	if isAttached(sess) {
		res, err := term.FocusTab(sess)
		if err == nil && res != "ok" {
			err = fmt.Errorf("%s: %w", sess, ErrNoTab) // "notfound": nothing was focused
		}
		return false, err
	}
	// Nothing is showing it: open a tab that attaches. This is the same call `gtmux new`
	// and `restore` make, and it is the only way to see a detached session at all.
	_, err = term.SpawnTabs([]string{sess}, false)
	return true, err
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
// Returns ErrNoPane if id isn't a pane id or the pane no longer exists, and an error
// when the jump did not reach the screen.
func FocusPaneByID(id string) error {
	if !paneIDRe.MatchString(id) {
		return fmt.Errorf("not a pane id %q: %w", id, ErrNoPane)
	}
	return JumpPane(id)
}
