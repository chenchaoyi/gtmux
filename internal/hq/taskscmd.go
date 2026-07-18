package hq

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

// taskJSON is the `gtmux tasks --json` contract: a tracked dispatch with its LIVE
// status derived from the radar (the pane is the source of truth). The attention-
// ledger fields (hq-attention-system) are additive/optional.
type taskJSON struct {
	ID          string `json:"id"`
	Pane        string `json:"pane"`
	Session     string `json:"session,omitempty"`
	Agent       string `json:"agent,omitempty"`
	Model       string `json:"model,omitempty"`
	Goal        string `json:"goal,omitempty"`
	Status      string `json:"status"` // waiting | done | working | gone | archived
	Source      string `json:"source"` // hq-dispatched | user-direct | agent-self
	Worktree    string `json:"worktree,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Snoozed     bool   `json:"snoozed,omitempty"`
	CreatedAt   int64  `json:"created_at,omitempty"`
	Tier        string `json:"tier,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Surfaced    bool   `json:"surfaced,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
}

// taskStatus maps a tracked pane's live radar status to the ledger lifecycle,
// "gone" when the pane is no longer live. The "needs-you first" ordering follows.
func taskStatus(t dispatch.Task, live map[string]radar.Pane) string {
	p, ok := live[t.Pane]
	if !ok {
		return "gone"
	}
	return radar.TaskStatusFor(p.Status)
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

// rowFor builds a taskJSON from a ledger entry + its live status.
func rowFor(t dispatch.Task, status string, now int64) taskJSON {
	return taskJSON{
		ID: t.ID, Pane: t.Pane, Session: t.Session, Agent: t.Agent, Model: t.Model,
		Goal: t.Goal, Status: status, Source: t.SourceOrDefault(),
		Worktree: t.Worktree, Branch: t.Branch,
		Snoozed: t.Snoozed(now), CreatedAt: t.CreatedAt,
		Tier: t.Tier, Priority: t.Priority, Surfaced: t.Surfaced,
		Disposition: t.Disposition, Archived: t.Archived,
	}
}

// gatherTasks joins the LIVE ledger with the radar and orders needs-you first.
func gatherTasks() []taskJSON {
	live := map[string]radar.Pane{}
	for _, p := range radar.GatherAgents() {
		live[p.PaneID] = p
	}
	now := time.Now().Unix()
	tasks := dispatch.ListTasks()
	out := make([]taskJSON, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, rowFor(t, taskStatus(t, live), now))
	}
	// stable needs-you-first order
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && taskRank(out[j].Status) < taskRank(out[j-1].Status); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// gatherArchivedTasks returns archived ledger entries as rows (status "archived",
// most-recently-archived first) — the `--verbose` retro-query.
func gatherArchivedTasks() []taskJSON {
	now := time.Now().Unix()
	arch := dispatch.ListArchived()
	out := make([]taskJSON, 0, len(arch))
	for _, t := range arch {
		out = append(out, rowFor(t, "archived", now))
	}
	return out
}

// CmdTasks implements `gtmux tasks [--json] [--verbose]` — the attention ledger.
func CmdTasks(args []string) int {
	jsonOut, verbose := false, false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--verbose", "-v":
			verbose = true
		case "-h", "--help":
			i18n.Say("usage: gtmux tasks [--json] [--verbose]", "用法：gtmux tasks [--json] [--verbose]")
			i18n.Say("  The attention ledger (gtmux spawn dispatches + attention items), live",
				"  注意力账本（gtmux spawn 派活 + 注意力条目），带实时状态，需要你的排在前面。")
			i18n.Say("  status, needs-you first. --verbose adds archived entries + tier/disposition.",
				"  --verbose 追加已归档条目 + 分级/处置/surfaced 列。")
			return 0
		default:
			i18n.Sae("gtmux tasks: unknown option '"+a+"'", "gtmux tasks: 未知选项 '"+a+"'")
			return 2
		}
	}
	rows := gatherTasks()
	if verbose {
		rows = append(rows, gatherArchivedTasks()...) // archived after the live set
	}
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
		fmt.Printf("%s %s  %s  %s%s%s%s\n", glyph, i18n.PadRight(label, 8), i18n.PadRight(loc, 22), r.Goal, src, snooze, verboseTail(r, verbose))
		if r.Worktree != "" {
			fmt.Printf("    %s %s (%s)\n", i18n.Dim+i18n.Tr("wt:", "worktree:"), r.Worktree, r.Branch+i18n.Reset)
		}
	}
	return 0
}

// verboseTail renders the attention columns (tier · priority · surfaced ·
// disposition) as a dimmed suffix, only under --verbose and only when set.
func verboseTail(r taskJSON, verbose bool) string {
	if !verbose {
		return ""
	}
	var parts []string
	if r.Tier != "" {
		parts = append(parts, r.Tier)
	}
	if r.Priority != 0 {
		parts = append(parts, fmt.Sprintf("p%d", r.Priority))
	}
	if r.Surfaced {
		parts = append(parts, "surfaced")
	}
	if r.Disposition != "" {
		parts = append(parts, r.Disposition)
	}
	if len(parts) == 0 {
		return ""
	}
	return i18n.Dim + "  · " + strings.Join(parts, " · ") + i18n.Reset
}

func taskGlyph(status string) (glyph, label string) {
	switch status {
	case "waiting":
		return i18n.Yellow + "⏸" + i18n.Reset, i18n.Tr("waiting", "等输入")
	case "done":
		return i18n.Green + "✳" + i18n.Reset, i18n.Tr("done", "已完成")
	case "working":
		return i18n.Cyan + "⠿" + i18n.Reset, i18n.Tr("working", "运行中")
	case "archived":
		return i18n.Dim + "▪" + i18n.Reset, i18n.Tr("archived", "已归档")
	default:
		return i18n.Dim + "○" + i18n.Reset, i18n.Tr("gone", "已消失")
	}
}
