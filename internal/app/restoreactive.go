package app

import (
	"bufio"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Restoring WHICH WINDOW AND PANE YOU WERE ON (restore-contract).
//
// tmux-resurrect saves this and tries to restore it — with `tmux switch-client`. That
// command moves an ATTACHED client; with no client attached it does nothing at all and
// reports no error. gtmux drives the restore headlessly (detached, from a LaunchAgent or
// a fresh terminal before you attach), so resurrect's attempt was always a no-op and
// every restore silently dropped you on window 0 of every session, whatever you had been
// looking at.
//
// Nobody reported this in the four restore regressions logged in one day — it had simply
// always been true, which is exactly the kind of thing a contract test finds and a
// bug-report queue does not.
//
// The fix is to replay it ourselves with `select-window` / `select-pane`, which operate
// on the SESSION and need no client. We read it from the same save file resurrect wrote,
// so there is no second source of truth.

// activeSpot is where a session was left: its active window, and that window's active
// pane.
type activeSpot struct {
	session string
	window  string
	pane    string // pane INDEX within the window ("" when the save didn't record one)
}

// parseActiveSpots reads a resurrect save and returns where each session was left.
//
// The save is tab-separated, one record per line. Two record types matter, and the field
// positions mirror the awk expressions in resurrect's own restore.sh so the two agree by
// construction:
//
//	window  <session> <window_index> :<name> <active> :<flags> <layout> ...
//	pane    <session> <window_index> :<name> <flags> <pane_index> ... <pane_active> ...
//
// A window is the session's active one when its flags carry `*`; a pane is its window's
// active one when field 9 is `1`.
func parseActiveSpots(savePath string) []activeSpot {
	f, err := os.Open(savePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	win := map[string]string{}  // session → active window index
	pane := map[string]string{} // "session\twindow" → active pane index
	order := []string{}         // sessions, in the order first seen (stable output)
	seen := map[string]bool{}   //
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), "\t")
		if len(fields) < 6 {
			continue
		}
		session := fields[1]
		switch fields[0] {
		case "window":
			// fields[5] is the flag column, prefixed with ':' by the save format.
			if strings.Contains(fields[5], "*") {
				win[session] = fields[2]
				if !seen[session] {
					seen[session], order = true, append(order, session)
				}
			}
		case "pane":
			if len(fields) < 9 || fields[8] != "1" {
				continue
			}
			pane[session+"\t"+fields[2]] = fields[5]
		}
	}

	out := make([]activeSpot, 0, len(order))
	for _, s := range order {
		w := win[s]
		out = append(out, activeSpot{session: s, window: w, pane: pane[s+"\t"+w]})
	}
	return out
}

// applyActiveSpots selects the saved window and pane in each session. Best-effort per
// session: a session that didn't come back must not stop the others from being placed
// correctly, and being on the wrong window is never worth failing a restore over.
func applyActiveSpots(spots []activeSpot) {
	for _, s := range spots {
		if s.session == "" || s.window == "" {
			continue
		}
		target := s.session + ":" + s.window
		if !tmux.OK("select-window", "-t", target) {
			continue // session or window absent — nothing to place
		}
		if s.pane != "" {
			tmux.OK("select-pane", "-t", target+"."+s.pane)
		}
	}
}

// restoreActiveSpots replays the active window/pane recorded in a save. Called after a
// driven restore completes.
func restoreActiveSpots(savePath string) {
	if savePath == "" {
		return
	}
	spots := parseActiveSpots(savePath)
	restoreLogf("active: replaying %d session spot(s) from %s", len(spots), savePath)
	applyActiveSpots(spots)
}
