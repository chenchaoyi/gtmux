// The supervisor waiting-nudge (supervisor MVP, P1): when an agent enters
// waiting and a supervisor (中控) session is live, type ONE compact event line
// into the supervisor's pane so it learns of the blocker without polling. The
// nudge rides the notification pipeline's dedup (an unchanged waiting state is
// not re-nudged — the caller gates on d.notify), never fires about the
// supervisor itself, is a no-op when no hq pane is live, and can be disabled
// with `"hqNudge": false` in ~/.config/gtmux/config.json.
//
// The nudge INFORMS only — gtmux never answers another agent's prompt; what the
// supervisor does with it is governed by its hq instructions file.
package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// hqNudgeEnabled reads ~/.config/gtmux/config.json's optional `hqNudge` key.
// Absent file/key/unreadable → true (on by default; the hq-pane check below is
// the real gate — no supervisor, no nudge, zero cost).
func hqNudgeEnabled() bool {
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json"))
	if err != nil {
		return true
	}
	var c struct {
		HQNudge *bool `json:"hqNudge"`
	}
	if json.Unmarshal(b, &c) != nil || c.HQNudge == nil {
		return true
	}
	return *c.HQNudge
}

// nudgeLine builds the injected message. kind may be "" (a generic wait).
func nudgeLine(kind, loc, pane, title string) string {
	msg := "[gtmux] waiting"
	if kind != "" {
		msg += "·" + kind
	}
	if loc != "" {
		msg += " " + loc
	}
	msg += " (" + pane + ")"
	if title = strings.TrimSpace(title); title != "" {
		msg += " — " + title
	}
	return msg
}

// findSupervisorPane returns a live hq pane other than the waiting pane itself
// ("" when none / the waiting pane IS the supervisor). Cwd-keyed, matching the
// radar's role detection.
func findSupervisorPane(waitingPane string) string {
	home := state.HQHome()
	for _, line := range tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{pane_current_path}") {
		f := strings.SplitN(line, "\t", 2)
		if len(f) != 2 || f[1] != home {
			continue
		}
		if f[0] == waitingPane {
			return "" // the supervisor itself is the waiting pane — never self-nudge
		}
		return f[0]
	}
	return ""
}

// nudgeSupervisor is the entry the hook calls on a fresh Waiting decision.
func nudgeSupervisor(waitingPane, kind string) {
	if waitingPane == "" || !hqNudgeEnabled() {
		return
	}
	target := findSupervisorPane(waitingPane)
	if target == "" {
		return
	}
	loc := tmux.Display(waitingPane, "#{session_name}:#{window_index}.#{pane_index}")
	title := tmux.Display(waitingPane, "#{pane_title}")
	msg := nudgeLine(kind, loc, waitingPane, title)
	if tmux.SendText(target, msg, true) == nil {
		debugf("nudged supervisor pane=%s about=%s kind=%s", target, waitingPane, kind)
	}
}
