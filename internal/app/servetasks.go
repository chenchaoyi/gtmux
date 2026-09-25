package app

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/server"
)

// tasksForServe joins the dispatch ledger with the radar's live pane statuses
// (chat-background-tasks). Both halves already exist and have never been put together:
// the ledger says what was sent and when, the radar says whether that pane is waiting,
// working or idle NOW. The phone's question needs both.
//
// A task whose pane is gone comes back as "gone" rather than being dropped. It happened,
// and a list that silently loses it reads as though it never did — which is exactly the
// failure the dispatch ledger exists to prevent.
func tasksForServe() ([]server.TaskInfo, error) {
	tasks := dispatch.ListTasks()
	if len(tasks) == 0 {
		return nil, nil
	}
	// One radar pass for the whole list: GatherAgents walks tmux, and doing it per task
	// would turn a ten-task ledger into ten scans.
	live := map[string]string{}
	for _, p := range radar.GatherAgents() {
		if p.PaneID != "" {
			live[p.PaneID] = p.Status
		}
	}
	return joinTasks(tasks, live), nil
}

// joinTasks is the join itself, split out so it can be tested without tmux: the ledger
// and a pane-to-status map in, the phone's answer out.
func joinTasks(tasks []dispatch.Task, live map[string]string) []server.TaskInfo {
	out := make([]server.TaskInfo, 0, len(tasks))
	for _, t := range tasks {
		status, ok := live[t.Pane]
		if !ok || t.Pane == "" {
			status = "gone"
		}
		out = append(out, server.TaskInfo{
			ID:     t.ID,
			Goal:   strings.TrimSpace(t.Goal),
			Agent:  t.Agent,
			Pane:   t.Pane,
			Status: status,
			Since:  taskSince(t),
			Source: t.Source,
		})
	}
	return out
}

// taskSince is when the task started, preferring the ledger's own creation stamp and
// falling back to first-seen for an entry written before CreatedAt was set.
func taskSince(t dispatch.Task) int64 {
	if t.CreatedAt > 0 {
		return t.CreatedAt
	}
	return t.FirstSeen
}
