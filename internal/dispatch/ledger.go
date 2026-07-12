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
	// OwnSession is true when spawn CREATED the tmux session (a fresh dispatch), false
	// when it reused an existing --pane. reap only kills a session spawn owns.
	OwnSession bool `json:"own_session,omitempty"`
	// SnoozeUntil silences reap suggestions for this task until this unix time
	// (incident ⑧). 0 = not snoozed.
	SnoozeUntil int64 `json:"snooze_until,omitempty"`
}

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
