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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/prompt"
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
// none / aboutPane IS the supervisor). The resolution rule lives in hqpane, shared
// with the serve tick — the `@gtmux_hq_home` stamp, then symlink-normalized
// cwd/start path.
func findSupervisorPane(aboutPane string) string {
	pane, _ := hqpane.FindOther(aboutPane)
	return pane
}

// wakeTarget answers "should a wake about aboutPane be built, and where does it go":
//
//   - (pane, true) — deliver it there;
//   - ("", true)   — HOLD it: no HQ resolves right now, but one was seen within
//     hqpane.SeenWindow, so it is queued for the next drain that finds one (a
//     restarting HQ pane, a tmux hiccup, a resolution bug we haven't found);
//   - ("", false)  — stay silent: the channel is off, the event is about HQ itself,
//     or this machine has never run a supervisor and so has nothing to hold it for.
//
// Callers check `ok` BEFORE building the line: with no HQ, the wake path must cost
// nothing — not a capture, not a `display-message`.
func wakeTarget(aboutPane string) (target string, ok bool) {
	if aboutPane == "" || !hqNudgeEnabled() {
		return "", false
	}
	target, self := hqpane.FindOther(aboutPane)
	if self {
		return "", false // never wake HQ about HQ
	}
	if target == "" && !hqpane.SeenRecently() {
		return "", false
	}
	return target, true
}

// nudgeSupervisor is the entry the hook calls on a fresh Waiting decision.
func nudgeSupervisor(waitingPane, kind string) {
	target, ok := wakeTarget(waitingPane)
	if !ok {
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
	deliverWake(target, hqwake.Line(class, wakeHead(waitingPane), title))
}

// nudgeHQ types one wake line into a live supervisor pane about aboutPane, gated on
// hqNudge + an HQ that isn't aboutPane itself. It reports whether the wake went
// anywhere (delivered or held) — callers with their own dedup state use it to avoid
// recording a wake that never happened.
func nudgeHQ(aboutPane, msg string) bool {
	target, ok := wakeTarget(aboutPane)
	if !ok {
		return false
	}
	deliverWake(target, msg)
	return true
}

// deliverWake types msg into an already-resolved supervisor pane, or queues it when
// the caller's wakeTarget said to hold. For callers that resolved the target
// themselves (and so must not pay for a second list-panes).
func deliverWake(target, msg string) {
	if target == "" {
		hqnudge.Enqueue(msg)
		debugf("held wake for an unresolved HQ: %s", msg)
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
	target, ok := wakeTarget(pane)
	if !ok {
		return
	}
	// Defense in depth (stuck-dispatch-waiting): a pane that only LOOKS finished because
	// it's blocked BEFORE a turn — a startup/permission gate, or a tracked dispatch still
	// holding its unsubmitted goal in the composer — must NOT wake as `done`. An
	// incidental Stop (e.g. a trust-gate keypress) can't relabel a stuck worker done; the
	// radar + slow-tick surface it as waiting instead.
	cap := tmux.CaptureFull(pane)
	if prompt.IsStartupGate(cap, "") {
		debugf("done suppressed (startup gate) pane=%s", pane)
		return
	}
	if _, tracked := dispatch.TaskForPane(pane); tracked {
		if draft, structured := dispatch.DraftOf(cap); structured && strings.TrimSpace(draft) != "" {
			debugf("done suppressed (unsubmitted draft) pane=%s", pane)
			return
		}
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

	switch due, dueAt := hqwake.DoneDue(pane, now, cfg.PaneMinGapSec); {
	case due:
		deliverWake(target, line)
		hqwake.StampDone(pane, now)
	case target != "":
		// Inside the merge window: replace the queued line; it types when due.
		hqnudge.DeliverKeyedAt(target, "done-"+pane, line, dueAt*int64(time.Second))
	default:
		hqnudge.Enqueue(line) // no live HQ to merge against — hold the line as-is
	}
}

// doneGoal resolves the finished session's goal: the dispatch ledger for a tracked
// task, else the pane's last user-direct prompt (its own marker — NOT the
// goal-changed dedup record, whose TTL would otherwise expire the goal too).
func doneGoal(pane string) string {
	if t, ok := dispatch.TaskForPane(pane); ok && strings.TrimSpace(t.Goal) != "" {
		return t.Goal
	}
	return state.ReadMarker(goalMarker(pane))
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

// goalChangedMarker holds the goal-changed DEDUP record for a pane (a fingerprint of
// the prompt plus when it was waked) — nothing else reads it.
func goalChangedMarker(pane string) string {
	return filepath.Join(state.Dir(), "goalchanged", pane)
}

// goalMarker holds the pane's last user-direct goal, which the done wake reads back.
// It is deliberately SEPARATE from the dedup record (hq-wake-reliability): the two
// were one file, so the dedup could not be given an expiry without churning the goal.
func goalMarker(pane string) string { return filepath.Join(state.Dir(), "goal", pane) }

// goalDedupTTL is how long an identical prompt is treated as a duplicate submission
// rather than a new instruction. The dedup exists to absorb a hook firing twice for
// one submission — NOT to decide that a user who repeats themselves means nothing.
// Before this window existed the marker never expired, so typing `继续` into a pane a
// second time (an hour later, a day later) reached HQ as silence.
const goalDedupTTL = 5 * time.Minute

// goalRecord is the dedup marker's payload.
type goalRecord struct {
	Hash string `json:"hash"` // sha256 of the FULL cleaned prompt, hex
	TS   int64  `json:"ts"`   // when the wake for it fired (unix seconds)
}

// goalStoreMax bounds the goal text kept on disk (the wake clamps it far shorter).
const goalStoreMax = 400

// goalWaked reports whether this exact prompt already waked HQ for this pane inside
// the dedup window. The fingerprint is over the FULL prompt, not the 40-rune head, so
// two different instructions that happen to share an opening are two wakes. A legacy
// plain-text marker (the pre-fingerprint format) fails to parse and reads as "not
// waked" — one extra wake, once, on upgrade.
func goalWaked(pane, hash string, now int64) bool {
	var rec goalRecord
	if json.Unmarshal([]byte(state.ReadMarker(goalChangedMarker(pane))), &rec) != nil {
		return false
	}
	return rec.Hash == hash && now-rec.TS < int64(goalDedupTTL.Seconds())
}

// nudgeGoalChanged tells HQ the user submitted a NEW prompt DIRECTLY into a non-HQ
// pane (dual-channel dispatch): the user dispatches via HQ OR straight into an agent
// window, and HQ must sense the latter so it records a user-direct task instead of
// chasing a stale ledger. Deduped per pane on a prompt fingerprint with a TTL; the
// goal is user text → delivered as DATA (goal:"…").
func nudgeGoalChanged(pane, goal string) {
	goal = strings.TrimSpace(goal)
	if pane == "" || goal == "" || !hqNudgeEnabled() {
		return
	}
	// The goal is the pane's, whether or not anything is listening: a `done` wake
	// minutes from now reads it back, and by then an HQ may well be live.
	_ = state.WriteMarker(goalMarker(pane), clampData(goal, goalStoreMax))

	sum := sha256.Sum256([]byte(goal))
	hash, now := hex.EncodeToString(sum[:]), time.Now().Unix()
	if goalWaked(pane, hash, now) {
		return // the same prompt, again, inside the window — one submission, one wake
	}
	if !nudgeHQ(pane, goalChangedLine(wakeHead(pane), goal)) {
		return // nothing fired — don't record a wake that never happened
	}
	rec, _ := json.Marshal(goalRecord{Hash: hash, TS: now})
	_ = state.WriteMarker(goalChangedMarker(pane), string(rec))
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
