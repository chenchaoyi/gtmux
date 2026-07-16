// The HQ wake channel, hook side (hq-perception-v2). Decision-dense events type
// ONE compact signal line into the live supervisor pane — the only knock; all
// process-level perception stays pull-side (events/digest). Lines are built by
// hqwake (the `» gtmux·<class> │ …` visual language), delivered draft-guarded by
// hqnudge, gated on a live HQ pane + the `hqNudge` config, and NEVER fire about
// the supervisor itself.
//
// The wake INFORMS only — gtmux never answers another agent's prompt; what the
// supervisor does with a wake is governed by its seeded playbook.
package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// hqNudgeEnabled reads ~/.config/gtmux/config.json's optional `hqNudge` key.
// Absent file/key/unreadable → true (on by default; the hq-pane check below is
// the real gate — no supervisor, no wake, zero cost).
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

// wakeHead builds the standard wake-line head: `<loc> (<pane>)`.
func wakeHead(pane string) string {
	loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
	if loc == "" {
		return "(" + pane + ")"
	}
	return loc + " (" + pane + ")"
}

// clampData trims + truncates an agent/user-authored payload for a one-line field.
func clampData(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// findSupervisorPane returns a live hq pane other than aboutPane itself ("" when
// none / aboutPane IS the supervisor). Cwd-keyed, matching the radar's role
// detection.
func findSupervisorPane(aboutPane string) string {
	home := state.HQHome()
	for _, line := range tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{pane_current_path}") {
		f := strings.SplitN(line, "\t", 2)
		if len(f) != 2 || f[1] != home {
			continue
		}
		if f[0] == aboutPane {
			return "" // the supervisor itself — never self-wake
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
	class := hqwake.ClassWaiting
	if kind != "" {
		class += "·" + kind
	}
	title := ""
	if t := clampData(tmux.Display(waitingPane, "#{pane_title}"), 80); t != "" {
		title = `title:"` + t + `"` // agent-authored → marked DATA, not an instruction
	}
	hqnudge.Deliver(target, hqwake.Line(class, wakeHead(waitingPane), title))
	debugf("waked supervisor pane=%s about=%s kind=%s", target, waitingPane, kind)
}

// nudgeHQ types one wake line into a live supervisor pane about aboutPane, gated
// on hqNudge + a live HQ (that isn't aboutPane itself).
func nudgeHQ(aboutPane, msg string) {
	if aboutPane == "" || !hqNudgeEnabled() {
		return
	}
	target := findSupervisorPane(aboutPane)
	if target == "" {
		return
	}
	hqnudge.Deliver(target, msg) // draft-guarded: queues behind a half-typed HQ draft
	debugf("waked HQ pane=%s: %s", target, msg)
}

// nudgeResolved tells HQ that a wait CLEARED (incident ⑤): the user answered in the
// pane's own window, or the agent resumed — so HQ can drop any pending chase. Always
// fires (the wake line IS the knock — no producer-side suppression).
func nudgeResolved(pane, kind string) {
	was := ""
	if kind != "" {
		was = "was " + kind
	}
	nudgeHQ(pane, hqwake.Line(hqwake.ClassResolved, wakeHead(pane), was))
}

// nudgeAsking tells HQ that a turn-END reply asked the user a question with NO menu
// (incident ⑥) — the case the waiting path can't see.
func nudgeAsking(pane, summary string) {
	ask := ""
	if s := clampData(summary, 100); s != "" {
		ask = `ask:"` + s + `"` // reply text → marked DATA
	}
	nudgeHQ(pane, hqwake.Line(hqwake.ClassAsks, wakeHead(pane), ask))
}

// wakeDone is the ANY-session completion wake (hq-perception-v2): a turn-end that
// left the pane idle fires an immediate `done` wake — unless the human was watching
// it happen (the focused pane of an attached client; then it counts toward the tick
// tally), per the configured mode. Immediate wakes are rate-merged per pane: a
// completion inside the merge window REPLACES the queued line (newest payload) and
// delivers when the window closes.
func wakeDone(pane, tail string, turnStart int64) {
	if pane == "" || !hqNudgeEnabled() {
		return
	}
	target := findSupervisorPane(pane)
	if target == "" {
		return
	}
	cfg := hqwake.Load()
	if cfg.Done == hqwake.DoneTick || (cfg.Done == hqwake.DoneUnattended && tmux.Attended(pane)) {
		hqwake.AddOutcome("done") // deferred to the summary tick — still never lost
		debugf("done tallied (attended/tick-mode) pane=%s", pane)
		return
	}
	now := time.Now().Unix()
	fields := make([]string, 0, 4)
	if turnStart > 0 && now > turnStart {
		fields = append(fields, fmtTurnDur(now-turnStart))
	}
	if g := clampData(doneGoal(pane), 80); g != "" {
		fields = append(fields, `goal:"`+g+`"`) // user-authored → DATA
	}
	if t := clampData(tail, 100); t != "" {
		fields = append(fields, `tail:"`+t+`"`) // agent-authored → DATA
	}
	fields = append(fields, hqwake.PullHint(now, 0))
	line := hqwake.Line(hqwake.ClassDone, wakeHead(pane), fields...)

	if due, dueAt := hqwake.DoneDue(pane, now, cfg.PaneMinGapSec); due {
		hqnudge.Deliver(target, line)
		hqwake.StampDone(pane, now)
	} else {
		// Inside the merge window: replace the queued line; it types when due.
		hqnudge.DeliverKeyedAt(target, "done-"+pane, line, dueAt*int64(time.Second))
	}
}

// doneGoal resolves the finished session's goal: the dispatch ledger for a tracked
// task, else the last user-direct prompt head (the goal-changed dedup marker).
func doneGoal(pane string) string {
	if t, ok := dispatch.TaskForPane(pane); ok && strings.TrimSpace(t.Goal) != "" {
		return t.Goal
	}
	return state.ReadMarker(goalChangedMarker(pane))
}

// fmtTurnDur renders a short human turn duration ("45s" / "3m" / "1h12m").
func fmtTurnDur(secs int64) string {
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm", secs/60)
	default:
		return fmt.Sprintf("%dh%02dm", secs/3600, (secs%3600)/60)
	}
}

// nudgeCrash tells HQ a turn DIED on an agent/API failure (StopFailure) — which
// must never read as a normal finish (severity important; always immediate).
func nudgeCrash(pane, errHead string) {
	field := "turn died (agent/API error)"
	if e := clampData(errHead, 100); e != "" {
		field = `err:"` + e + `"` // agent/runtime-authored → DATA
	}
	nudgeHQ(pane, hqwake.Line(hqwake.ClassCrash, wakeHead(pane), field))
}

// ── enrollment (建联): first sight of an agent pane → one new-session wake ────

// enrolledMarker dedups the new-session wake per pane (removed on SessionEnd so a
// reused pane id re-enrolls its next session). `gtmux hq` pre-stamps all live
// panes at start — its own fleet enrollment covers them — so only genuine
// newcomers wake.
func enrolledMarker(pane string) string {
	return filepath.Join(state.Dir(), "enrolled", pane)
}

// ensureEnrolled fires exactly one `new-session` wake the first time a pane is
// seen firing agent hooks. Stamps regardless of a live HQ (a later HQ start does
// its own full enrollment, so backfilling wakes would be noise).
func ensureEnrolled(pane, agentDisplay string) {
	if pane == "" || state.Exists(enrolledMarker(pane)) {
		return
	}
	_ = state.WriteMarker(enrolledMarker(pane), agentDisplay)
	if !hqNudgeEnabled() {
		return
	}
	agent := ""
	if agentDisplay != "" {
		agent = "agent:" + agentDisplay
	}
	nudgeHQ(pane, hqwake.Line(hqwake.ClassNewSession, wakeHead(pane), agent, "enroll it"))
}

// unenroll clears the enrollment marker (SessionEnd) and tallies the outcome so
// the next tick brief reports the departure.
func unenroll(pane string) {
	if pane == "" {
		return
	}
	if state.Exists(enrolledMarker(pane)) {
		state.Remove(enrolledMarker(pane))
		hqwake.AddOutcome("gone")
	}
}

// StampEnrolledAll marks every currently-live tmux pane as enrolled — called by
// `gtmux hq` at start, whose seeded first turn does the FULL fleet enrollment, so
// pre-existing panes must not each fire a new-session wake afterwards.
func StampEnrolledAll() {
	for _, pane := range tmux.Lines("list-panes", "-a", "-F", "#{pane_id}") {
		pane = strings.TrimSpace(pane)
		if pane == "" || state.Exists(enrolledMarker(pane)) {
			continue
		}
		_ = state.WriteMarker(enrolledMarker(pane), "")
	}
}

// goalChangedMarker dedups the goal-changed wake per pane on the prompt head, and
// doubles as the pane's last user-direct goal (the done wake reads it back).
func goalChangedMarker(pane string) string {
	return filepath.Join(state.Dir(), "goalchanged", pane)
}

// nudgeGoalChanged tells HQ the user submitted a NEW prompt DIRECTLY into a non-HQ
// pane (dual-channel dispatch): the user dispatches via HQ OR straight into an agent
// window, and HQ must sense the latter so it records a user-direct task instead of
// chasing a stale ledger. Deduped per pane on the prompt head; the head is user
// text → delivered as DATA (goal:"…"). Always fires (no producer-side suppression).
func nudgeGoalChanged(pane, head string) {
	head = strings.TrimSpace(head)
	if pane == "" || head == "" || !hqNudgeEnabled() {
		return
	}
	if state.ReadMarker(goalChangedMarker(pane)) == head {
		return // same prompt head already waked
	}
	target := findSupervisorPane(pane)
	if target == "" {
		return
	}
	_ = state.WriteMarker(goalChangedMarker(pane), head)
	hqnudge.Deliver(target, goalChangedLine(wakeHead(pane), head))
}

// goalChangedLine builds the dual-channel wake, marking the user-authored prompt
// head as DATA (`goal:"…"`) so it can't read to HQ as an instruction.
func goalChangedLine(head, promptHead string) string {
	return hqwake.Line(hqwake.ClassGoalChanged, head, `goal:"`+clampData(promptHead, 80)+`"`)
}

// sweepReapSuggestions scans tracked dispatches for reap CANDIDATES (idle-after-work
// past the threshold, branch merged or absent, not snoozed, not already suggested)
// and wakes HQ once per candidate with the exact `gtmux reap` command (incident ⑦).
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
		goal := ""
		if g := clampData(t.Goal, 60); g != "" {
			goal = `goal:"` + g + `"` // agent-authored → marked DATA
		}
		nudgeHQ(t.Pane, hqwake.Line(hqwake.ClassReapSuggest, wakeHead(t.Pane), goal, "gtmux reap "+t.ID))
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
