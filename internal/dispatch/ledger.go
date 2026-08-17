package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Task is one dispatch ledger entry — what `gtmux spawn` created and where it went.
// The lifecycle STATUS is NOT stored: it is derived from the dispatched pane's live
// radar state on read (the pane is the source of truth; this record just names the
// dispatch and remembers what to reclaim). session/worktree/branch record what spawn
// CREATED, for `gtmux reap`.
type Task struct {
	ID        string `json:"id"`
	Pane      string `json:"pane"`
	Session   string `json:"session"`
	Agent     string `json:"agent"`
	Model     string `json:"model,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
	Worktree  string `json:"worktree,omitempty"` // git worktree path spawn created ("" = none)
	Branch    string `json:"branch,omitempty"`   // branch spawn created ("" = none)
	Goal      string `json:"goal"`
	CreatedAt int64  `json:"created_at"`
	Delivered bool   `json:"delivered"`
	// State is the dispatch's delivery VERDICT (a dispatch.State: landed / queued /
	// failed / refused-*). Additive and optional — a legacy entry has none, and
	// Undelivered falls back to Delivered alone for it. It exists because `delivered`
	// is a bool over a three-way outcome: a QUEUED delivery reached the agent (it just
	// sits behind the current turn) yet is not `delivered:true`, and conflating it with
	// a ready-timeout would report accepted work as never-dispatched.
	State string `json:"state,omitempty"`
	// Source records which dispatch CHANNEL created the entry (dual-channel awareness):
	// SourceHQDispatched (gtmux spawn — the tracked path), SourceUserDirect (the user
	// typed a prompt straight into an agent window; HQ back-fills this), or
	// SourceAgentSelf (the agent started work on its own). Additive/optional — an entry
	// without it reads as hq-dispatched (Source() applies that default).
	Source string `json:"source,omitempty"`
	// OwnSession is true when spawn CREATED the tmux session (a fresh dispatch), false
	// when it reused an existing --pane. reap only kills a session spawn owns.
	OwnSession bool `json:"own_session,omitempty"`
	// SnoozeUntil silences reap suggestions for this task until this unix time
	// (incident ⑧). 0 = not snoozed.
	SnoozeUntil int64 `json:"snooze_until,omitempty"`

	// --- disposition fields (hq-attention-system) — additive/optional so a legacy
	// entry still loads, including one written when the ledger carried the retired
	// tier/priority/surfaced/archive fields (unknown JSON fields are ignored).
	Disposition string `json:"disposition,omitempty"` // free text: auto-answered / relayed / todo
	// AwaitingSince stamps when the entry entered the PENDING-DECISION set (the
	// DispositionAwaitingCommander disposition) — the clock the standing view orders
	// by, kept separate from LastUpdate, which every unrelated mutation moves. It is
	// additive in the strict sense: absent in every entry written before it existed,
	// and its zero value means exactly what those entries already meant — not pending.
	// Only the disposition decides membership; this only remembers when it started.
	AwaitingSince int64 `json:"awaiting_since,omitempty"`
	FirstSeen     int64 `json:"first_seen,omitempty"`  // when the item first entered the ledger
	LastUpdate    int64 `json:"last_update,omitempty"` // last mutation (disposition / …)
}

// Dispatch-channel sources for Task.Source (dual-channel awareness).
const (
	SourceHQDispatched = "hq-dispatched" // gtmux spawn — the tracked dispatch path
	SourceUserDirect   = "user-direct"   // user typed a prompt straight into an agent window
	SourceAgentSelf    = "agent-self"    // the agent started work on its own
)

// SourceOrDefault returns the task's dispatch channel, defaulting a legacy entry
// (written before the field existed) to hq-dispatched.
func (t Task) SourceOrDefault() string {
	if t.Source == "" {
		return SourceHQDispatched
	}
	return t.Source
}

// Undelivered reports whether the ledger says this dispatch's goal never reached the
// agent — the fact every status view must respect, because the pane cannot tell you.
//
// A dispatch that dies at the ready gate leaves a live, EMPTY, idle agent pane, and a
// status derived from that pane alone renders it `done` (idle → done) with the recorded
// goal — the INTENT, never the result — printed beside it. That is how three dispatches
// on 2026-08-09 read as finished work while not one of them had received a single word.
//
// `queued` is deliberately NOT undelivered: the agent accepted the instruction, it just
// runs after the current turn.
func (t Task) Undelivered() bool {
	if t.Delivered {
		return false
	}
	return t.State != string(StateQueued)
}

// MarkDelivered records that this pane's tracked dispatch DID land — after the fact and
// through another channel. The documented rescue for a failed spawn is
// `gtmux send --message-file <same file>` into the pane spawn left behind; without this
// the rescue works and the ledger still says the dispatch never landed, so the row would
// read `undelivered` forever while the worker is busy doing the job.
//
// It is gated on the delivered text actually BEING that goal, because the point of the
// undelivered status is that the ledger tells the truth: an unrelated "keep going" typed
// into the same pane must not launder a dispatch that never happened. No-op when the
// pane has no entry, its entry already landed, or the text is something else.
func MarkDelivered(pane, text string, now int64) bool {
	t, ok := TaskForPane(pane)
	if !ok || !t.Undelivered() || !deliversGoal(t.Goal, text) {
		return false
	}
	return updateTask(t.ID, now, func(t *Task) {
		t.Delivered = true
		t.State = string(StateLanded)
	})
}

// deliversGoal reports whether `text` opens with the goal the ledger recorded. The
// stored goal is whitespace-collapsed and truncated (radar.Snip 200) — so the comparison
// is head-only, over the same collapse, with the ellipsis dropped. A prefix rather than
// an equality because the rescue may add a line ("re-sending the goal below") ahead of
// nothing and because the stored copy is the truncated one.
func deliversGoal(goal, text string) bool {
	head := strings.TrimSuffix(strings.TrimSpace(goal), "…")
	if head == "" {
		return false
	}
	const n = 80 // enough to be unmistakable, short enough to survive the stored truncation
	if r := []rune(head); len(r) > n {
		head = string(r[:n])
	}
	return strings.HasPrefix(collapseSpace(text), head)
}

// collapseSpace mirrors radar.Snip's normalization (Fields+join) — dispatch cannot
// import radar (radar imports dispatch), and the two must agree or the head never matches.
func collapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// tasksDir is where ledger entries live.
func tasksDir() string { return filepath.Join(state.Dir(), "tasks") }

func taskPath(id string) string { return filepath.Join(tasksDir(), sanitizeID(id)+".json") }

// sanitizeID keeps an id safe as a filename.
func sanitizeID(id string) string { return filepath.Base(filepath.Clean("/" + id)) }

// NewID mints a short, unique-enough ledger id from a monotonic timestamp
// (base-36). `now` is passed in (nanoseconds) so it stays testable.
func NewID(nowNano int64) string {
	return "t" + strconv.FormatInt(nowNano, 36)
}

// AddTask writes a ledger entry (creating the dir).
func AddTask(t Task) error {
	if err := os.MkdirAll(tasksDir(), 0o755); err != nil {
		return err
	}
	// Attention-ledger: default FirstSeen once (preserved across later re-saves).
	if t.FirstSeen == 0 {
		t.FirstSeen = t.CreatedAt
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(taskPath(t.ID), b, 0o644)
}

// LoadTask returns a ledger entry by id.
func LoadTask(id string) (Task, bool) {
	b, err := os.ReadFile(taskPath(id))
	if err != nil {
		return Task{}, false
	}
	var t Task
	if json.Unmarshal(b, &t) != nil {
		return Task{}, false
	}
	return t, true
}

// ListTasks returns all ledger entries, newest first.
func ListTasks() []Task {
	entries, _ := os.ReadDir(tasksDir())
	var out []Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if t, ok := LoadTask(id); ok {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// TaskForPane returns the ledger entry whose dispatched pane is `pane` (the most
// recent if several ever shared it), false when none.
func TaskForPane(pane string) (Task, bool) {
	var found Task
	ok := false
	for _, t := range ListTasks() {
		if t.Pane == pane && (!ok || t.CreatedAt > found.CreatedAt) {
			found, ok = t, true
		}
	}
	return found, ok
}

// ResumableTask finds a PREVIOUS ATTEMPT at this same dispatch — one that created a
// session but never got its goal into the agent — so a re-run can adopt it instead of
// minting a second empty session beside it. Match rules, all required:
//
//   - `delivered:false` — a dispatch that landed is finished work, never something to
//     silently re-enter; re-running after a success is a genuinely new dispatch.
//   - `own_session` — spawn created that session, so spawn may reuse it. A `--pane`
//     dispatch borrowed someone else's pane and has nothing to resume.
//   - the same TARGET: the worktree path when this dispatch uses one (the strongest
//     key — a worktree is per-branch), else the session name spawn derives from
//     title/branch/goal, which is deterministic for the same command.
//
// Liveness is deliberately NOT checked here: the ledger records what was created, and
// the pane is the source of truth for what survived. The caller checks the pane and
// falls through to a fresh session when it is gone.
func ResumableTask(worktree, session string) (Task, bool) {
	var found Task
	ok := false
	for _, t := range ListTasks() {
		if t.Delivered || !t.OwnSession || t.Pane == "" {
			continue
		}
		match := false
		if worktree != "" {
			match = t.Worktree == worktree
		} else {
			match = t.Worktree == "" && session != "" && t.Session == session
		}
		if match && (!ok || t.CreatedAt > found.CreatedAt) {
			found, ok = t, true
		}
	}
	return found, ok
}

// RemoveTask deletes a ledger entry (and its reap-suggested marker).
func RemoveTask(id string) {
	state.Remove(taskPath(id))
	state.Remove(reapSuggestedPath(id))
}

// reapSuggestedPath is the per-task "already suggested for reap" dedup marker.
func reapSuggestedPath(id string) string {
	return filepath.Join(tasksDir(), "suggested", sanitizeID(id))
}

// MarkReapSuggested records that a reap suggestion has fired for this task, so the
// sweep does not re-suggest it every tick.
func MarkReapSuggested(id string) { _ = state.Touch(reapSuggestedPath(id)) }

// ReapSuggested reports whether a reap suggestion already fired for this task.
func ReapSuggested(id string) bool { return state.Exists(reapSuggestedPath(id)) }

// SnoozeTask stamps SnoozeUntil on a task (incident ⑧) and persists it, clearing
// the reap-suggested marker so the suggestion can resume once the snooze lapses. A
// missing task is a no-op returning false.
func SnoozeTask(id string, until int64) bool {
	t, ok := LoadTask(id)
	if !ok {
		return false
	}
	t.SnoozeUntil = until
	state.Remove(reapSuggestedPath(id))
	return AddTask(t) == nil
}

// Snoozed reports whether a task's reap suggestion is currently silenced at `now`.
func (t Task) Snoozed(now int64) bool { return t.SnoozeUntil > now }
