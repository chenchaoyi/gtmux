package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// taskJSON is the `gtmux tasks --json` contract: a tracked dispatch with its LIVE
// status derived from the radar (the pane is the source of truth).
type taskJSON struct {
	ID        string `json:"id"`
	Pane      string `json:"pane"`
	Session   string `json:"session,omitempty"`
	Agent     string `json:"agent,omitempty"`
	Model     string `json:"model,omitempty"`
	Goal      string `json:"goal,omitempty"`
	Status    string `json:"status"` // waiting | done | working | gone
	Source    string `json:"source"` // hq-dispatched | user-direct | agent-self
	Worktree  string `json:"worktree,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Snoozed   bool   `json:"snoozed,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

// taskStatusFor maps a pane's radar status to the ledger lifecycle string:
// waiting (needs you) → done (idle-after-work, review me) → working.
func taskStatusFor(paneStatus string) string {
	switch paneStatus {
	case "waiting":
		return "waiting"
	case "idle":
		return "done"
	default:
		return "working"
	}
}

// taskStatus maps a tracked pane's live radar status to the ledger lifecycle,
// "gone" when the pane is no longer live. The "needs-you first" ordering follows.
func taskStatus(t dispatch.Task, live map[string]agentPane) string {
	p, ok := live[t.Pane]
	if !ok {
		return "gone"
	}
	return taskStatusFor(p.status)
}

func taskRank(status string) int {
	switch status {
	case "waiting":
		return 0
	case "done":
		return 1
	case "working":
		return 2
	default:
		return 3
	}
}

// gatherTasks joins the ledger with the live radar and orders needs-you first.
func gatherTasks() []taskJSON {
	live := map[string]agentPane{}
	for _, p := range gatherAgents() {
		live[p.paneID] = p
	}
	now := time.Now().Unix()
	tasks := dispatch.ListTasks()
	out := make([]taskJSON, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, taskJSON{
			ID: t.ID, Pane: t.Pane, Session: t.Session, Agent: t.Agent, Model: t.Model,
			Goal: t.Goal, Status: taskStatus(t, live), Source: t.SourceOrDefault(),
			Worktree: t.Worktree, Branch: t.Branch,
			Snoozed: t.Snoozed(now), CreatedAt: t.CreatedAt,
		})
	}
	// stable needs-you-first order
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && taskRank(out[j].Status) < taskRank(out[j-1].Status); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// cmdTasks implements `gtmux tasks [--json]` — the dispatch/needs-you ledger.
func cmdTasks(args []string) int {
	jsonOut := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			i18n.Say("usage: gtmux tasks [--json]", "用法：gtmux tasks [--json]")
			i18n.Say("  Dispatched tasks (gtmux spawn) with live status, needs-you first.",
				"  已派活的任务（gtmux spawn），带实时状态，需要你的排在前面。")
			return 0
		default:
			i18n.Sae("gtmux tasks: unknown option '"+a+"'", "gtmux tasks: 未知选项 '"+a+"'")
			return 2
		}
	}
	rows := gatherTasks()
	if jsonOut {
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	if len(rows) == 0 {
		i18n.Say("No dispatched tasks.", "没有派活记录。")
		return 0
	}
	for _, r := range rows {
		glyph, label := taskGlyph(r.Status)
		snooze := ""
		if r.Snoozed {
			snooze = i18n.Dim + " 💤" + i18n.Reset
		}
		loc := r.Pane
		if r.Session != "" {
			loc = r.Session + " " + r.Pane
		}
		src := ""
		if r.Source != dispatch.SourceHQDispatched { // only tag the notable channels
			src = i18n.Dim + " [" + r.Source + "]" + i18n.Reset
		}
		fmt.Printf("%s %s  %s  %s%s%s\n", glyph, i18n.PadRight(label, 8), i18n.PadRight(loc, 22), r.Goal, src, snooze)
		if r.Worktree != "" {
			fmt.Printf("    %s %s (%s)\n", i18n.Dim+i18n.Tr("wt:", "worktree:"), r.Worktree, r.Branch+i18n.Reset)
		}
	}
	return 0
}

func taskGlyph(status string) (glyph, label string) {
	switch status {
	case "waiting":
		return i18n.Yellow + "⏸" + i18n.Reset, i18n.Tr("waiting", "等输入")
	case "done":
		return i18n.Green + "✳" + i18n.Reset, i18n.Tr("done", "已完成")
	case "working":
		return i18n.Cyan + "⠿" + i18n.Reset, i18n.Tr("working", "运行中")
	default:
		return i18n.Dim + "○" + i18n.Reset, i18n.Tr("gone", "已消失")
	}
}
