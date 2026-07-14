package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// The attention-ledger operations (hq-attention-system): promote / re-prioritize /
// mark-surfaced / set-disposition on a live entry, and ARCHIVE a closed one out of
// the live set so `gtmux tasks` stays small (rollable) while remaining retro-
// queryable via `--verbose`. All mutations are in-place (no duplicate entry), which
// is what late-promotion needs: a QUIET item recorded now can be raised later when
// related events accrue.

// archiveDir holds closed ledger entries, out of the live set.
func archiveDir() string { return filepath.Join(tasksDir(), "archive") }

func archivePath(id string) string { return filepath.Join(archiveDir(), sanitizeID(id)+".json") }

// updateTask loads a LIVE task, applies mutate, stamps LastUpdate, and re-saves it.
// A missing task is a no-op returning false.
func updateTask(id string, now int64, mutate func(*Task)) bool {
	t, ok := LoadTask(id)
	if !ok {
		return false
	}
	mutate(&t)
	t.LastUpdate = now
	return AddTask(t) == nil
}

// Promote raises an entry's surfacing tier and/or priority AFTER it was first
// recorded — late promotion (a QUIET item that accrued related events). It mutates
// the SAME entry (no duplicate). An empty tier leaves the tier unchanged; a
// non-positive priority leaves the priority unchanged.
func Promote(id, tier string, priority int, now int64) bool {
	return updateTask(id, now, func(t *Task) {
		if tier != "" {
			t.Tier = tier
		}
		if priority > 0 {
			t.Priority = priority
		}
	})
}

// SetPriority re-orders an entry (higher = more urgent).
func SetPriority(id string, priority, now int64) bool {
	return updateTask(id, now, func(t *Task) { t.Priority = int(priority) })
}

// MarkSurfaced records that HQ has shown this item to the user (so a QUIET item
// isn't re-surfaced and the ledger can report surfaced-vs-not).
func MarkSurfaced(id string, now int64) bool {
	return updateTask(id, now, func(t *Task) {
		t.Surfaced = true
		if t.SurfacedAt == 0 {
			t.SurfacedAt = now
		}
	})
}

// SetDisposition records how an item was handled (auto-answered / relayed / todo …).
func SetDisposition(id, disposition string, now int64) bool {
	return updateTask(id, now, func(t *Task) { t.Disposition = disposition })
}

// ArchiveTask closes an entry: it stamps Archived/ArchivedAt, writes it under the
// archive dir, and removes it from the live set (and its reap-suggested marker), so
// the live ledger stays small while the entry remains retro-queryable. A missing
// task is a no-op returning false.
func ArchiveTask(id string, now int64) bool {
	t, ok := LoadTask(id)
	if !ok {
		return false
	}
	t.Archived = true
	t.ArchivedAt = now
	t.LastUpdate = now
	if err := os.MkdirAll(archiveDir(), 0o755); err != nil {
		return false
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return false
	}
	if err := os.WriteFile(archivePath(id), b, 0o644); err != nil {
		return false
	}
	// Remove from the live set (and its reap marker) only after the archive write
	// succeeded, so an entry is never lost between the two.
	state.Remove(taskPath(id))
	state.Remove(reapSuggestedPath(id))
	return true
}

// ListArchived returns archived entries, most-recently-archived first.
func ListArchived() []Task {
	entries, _ := os.ReadDir(archiveDir())
	var out []Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(archiveDir(), e.Name()))
		if err != nil {
			continue
		}
		var t Task
		if json.Unmarshal(b, &t) == nil {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ArchivedAt > out[j].ArchivedAt })
	return out
}

// LoadAnyTask loads a task from the live set OR the archive (live wins).
func LoadAnyTask(id string) (Task, bool) {
	if t, ok := LoadTask(id); ok {
		return t, true
	}
	b, err := os.ReadFile(archivePath(id))
	if err != nil {
		return Task{}, false
	}
	var t Task
	if json.Unmarshal(b, &t) != nil {
		return Task{}, false
	}
	return t, true
}
