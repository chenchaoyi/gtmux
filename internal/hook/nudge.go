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
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
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

// nudgeHQ types one compact line into a live supervisor pane about aboutPane,
// gated on hqNudge + a live HQ (that isn't aboutPane itself). The shared path for
// every non-waiting nudge (resolved / done / asks / reap-suggest).
func nudgeHQ(aboutPane, msg string) {
	if aboutPane == "" || !hqNudgeEnabled() {
		return
	}
	target := findSupervisorPane(aboutPane)
	if target == "" {
		return
	}
	if tmux.SendText(target, msg, true) == nil {
		debugf("nudged HQ pane=%s: %s", target, msg)
	}
}

// nudgeResolved tells HQ that a wait CLEARED (incident ⑤): the user answered in the
// pane's own window, or the agent resumed — so HQ can drop any pending chase.
func nudgeResolved(pane, kind string) {
	loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
	msg := "[gtmux] resolved"
	if loc != "" {
		msg += " " + loc
	}
	msg += " (" + pane + ")"
	if kind != "" {
		msg += " — was " + kind
	}
	nudgeHQ(pane, msg)
}

// nudgeDone tells HQ a tracked dispatch finished (its pane went idle after work).
func nudgeDone(pane, goal string) {
	loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
	msg := "[gtmux] done"
	if loc != "" {
		msg += " " + loc
	}
	msg += " (" + pane + ")"
	if goal = strings.TrimSpace(goal); goal != "" {
		msg += " — " + goal
	}
	nudgeHQ(pane, msg)
}

// nudgeAsking tells HQ that a turn-END reply asked the user a question with NO menu
// (incident ⑥) — the case the waiting path can't see.
func nudgeAsking(pane, summary string) {
	loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
	msg := "[gtmux] asks"
	if loc != "" {
		msg += " " + loc
	}
	msg += " (" + pane + ")"
	if summary = strings.TrimSpace(summary); summary != "" {
		msg += ` — "` + summary + `"`
	}
	nudgeHQ(pane, msg)
}

// sweepReapSuggestions scans tracked dispatches for reap CANDIDATES (idle-after-work
// past the threshold, branch merged or absent, not snoozed, not already suggested)
// and nudges HQ once per candidate with the exact `gtmux reap` command (incident ⑦).
// Runs on Stop only when a live HQ exists, so the git checks touch only rare
// candidates. Suggests only — never reclaims (that stays suggest→approve→execute).
func sweepReapSuggestions() {
	if !hqNudgeEnabled() || findSupervisorPane("") == "" {
		return
	}
	now := time.Now().Unix()
	tune := dispatch.LoadTuning()
	for _, t := range dispatch.ListTasks() {
		if t.Pane == "" || t.Snoozed(now) || dispatch.ReapSuggested(t.ID) {
			continue
		}
		since := paneIdleSince(t.Pane)
		if since == 0 || now-since < tune.ReapIdleThreshold {
			continue // not idle, or not idle long enough
		}
		if t.Branch != "" && t.Worktree != "" {
			if merged, err := dispatch.BranchMerged(t.Worktree, t.Branch); err != nil || !merged {
				continue // only suggest a merged (safely reclaimable) dispatch
			}
		}
		loc := tmux.Display(t.Pane, "#{session_name}:#{window_index}.#{pane_index}")
		msg := "[gtmux] reap-suggest " + loc + " (" + t.Pane + ")"
		if t.Goal != "" {
			msg += " — " + t.Goal
		}
		msg += "  ·  gtmux reap " + t.ID
		nudgeHQ(t.Pane, msg)
		dispatch.MarkReapSuggested(t.ID)
	}
}

// paneIdleSince returns when a pane's turn finished (its finished marker mtime), or
// 0 when it is not idle (no finished marker, or an active/waiting marker present).
func paneIdleSince(pane string) int64 {
	if state.Exists(state.ActivePath(pane)) || state.Exists(state.WaitingPath(pane)) {
		return 0
	}
	fi, err := os.Stat(state.FinishedPath(pane))
	if err != nil {
		return 0
	}
	return fi.ModTime().Unix()
}
