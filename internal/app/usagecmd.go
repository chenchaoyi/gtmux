// `gtmux usage` — the usage-watch fleet view: per-session token snapshots + a
// per-agent-type rollup (summed rate judged against the type layer). Same rows
// serve GET /api/usage. See openspec usage-watch.
package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// usageRow is one session's usage + identity (the CLI/API shape).
type usageRow struct {
	PaneID    string  `json:"pane_id,omitempty"`
	Loc       string  `json:"loc,omitempty"`
	Agent     string  `json:"agent"`     // display name
	AgentKey  string  `json:"agent_key"` // hook key (threshold identity)
	Status    string  `json:"status"`
	Tok       int64   `json:"tok"`  // cumulative output tokens
	In        int64   `json:"in"`   // cumulative non-cached input tokens
	Ctx       float64 `json:"ctx"`  // live context fraction
	Rate      int64   `json:"rate"` // output tokens/min (recent window)
	UsageWarn string  `json:"usage_warn,omitempty"`
}

// usageRollup is a per-agent-type aggregate.
type usageRollup struct {
	AgentKey  string `json:"agent_key"`
	Sessions  int    `json:"sessions"`
	Tok       int64  `json:"tok"`
	Rate      int64  `json:"rate"` // summed across the type's sessions
	UsageWarn string `json:"usage_warn,omitempty"`
}

type usageReport struct {
	Sessions []usageRow    `json:"sessions"`
	Types    []usageRollup `json:"types"`
	Limits   limits.Report `json:"limits"` // real subscription windows (limits-watch)
}

// gatherUsage assembles rows over the current radar (radar ordering) + rollups.
func gatherUsage() usageReport {
	now := time.Now()
	panes := gatherAgents()
	rep := usageReport{}
	agg := map[string]*usageRollup{}
	for _, p := range panes {
		agentKey, sessionID := sessionRef(p)
		if sessionID == "" {
			continue
		}
		u, ok := uwatch.ForSession(agentKey, sessionID, now)
		if !ok {
			continue
		}
		row := usageRow{
			PaneID: p.paneID, Loc: p.loc, Agent: p.agent, AgentKey: agentKey,
			Status: p.status, Tok: u.OutTok, In: u.InTok, Ctx: u.CtxFrac,
			Rate: u.RatePerMin, UsageWarn: uwatch.EvaluateSession(u),
		}
		rep.Sessions = append(rep.Sessions, row)
		a := agg[agentKey]
		if a == nil {
			a = &usageRollup{AgentKey: agentKey}
			agg[agentKey] = a
		}
		a.Sessions++
		a.Tok += u.OutTok
		a.Rate += u.RatePerMin
	}
	for _, a := range agg {
		a.UsageWarn = uwatch.TypeRateWarn(a.AgentKey, a.Rate)
		rep.Types = append(rep.Types, *a)
	}
	sort.Slice(rep.Types, func(i, j int) bool { return rep.Types[i].AgentKey < rep.Types[j].AgentKey })
	// Real subscription-window remaining (cached; never spawns per call unless stale).
	rep.Limits, _ = limits.Get(limits.LoadConfig(), false, now)
	maybeNudgeLimits(rep.Limits)
	return rep
}

// maybeNudgeLimits informs a live HQ once when the subscription-limits warn state
// CHANGES (a weekly window crossed the threshold, or cleared). Deduped via a
// state marker so repeated usage gathers (serve polls) don't re-nudge. Uses the
// same channel as the waiting/usage nudges.
func maybeNudgeLimits(r limits.Report) {
	marker := filepath.Join(state.Dir(), "limitswarn")
	prior := state.ReadMarker(marker)
	if r.Warn == prior {
		return
	}
	if r.Warn == "" {
		state.Remove(marker)
		return
	}
	_ = state.WriteMarker(marker, r.Warn)
	if pane := findHQPane(); pane != "" {
		_ = tmux.SendText(pane, "[gtmux] limits·warn "+r.Warn, true)
	}
}

// usageJSONBytes serves the CLI --json and GET /api/usage identically.
func usageJSONBytes() ([]byte, error) {
	return json.MarshalIndent(gatherUsage(), "", "  ")
}

// cmdUsage implements `gtmux usage [--json]`.
func cmdUsage(args []string) int {
	jsonOut := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			i18n.Say("usage: gtmux usage [--json]", "用法：gtmux usage [--json]")
			i18n.Say("  Token usage per agent session + per-type rollup, with threshold warnings.",
				"  每个 agent 会话的 token 用量 + 按类型汇总,含阈值预警。")
			i18n.Say("  Thresholds: ~/.config/gtmux/usage.json (per agent type; see docs/cli.md).",
				"  阈值:~/.config/gtmux/usage.json(按 agent 类型;见 docs/cli.md)。")
			return 0
		default:
			i18n.Sae("gtmux usage: unknown option '"+a+"'", "gtmux usage: 未知选项 '"+a+"'")
			return 2
		}
	}
	if jsonOut {
		b, err := usageJSONBytes()
		if err != nil {
			i18n.Sae("gtmux: "+err.Error(), "gtmux: "+err.Error())
			return 1
		}
		fmt.Println(string(b))
		return 0
	}
	rep := gatherUsage()
	if len(rep.Sessions) == 0 {
		i18n.Say("No sessions with usage data.", "没有带用量数据的会话。")
		return 0
	}
	for _, r := range rep.Sessions {
		head := r.Loc
		if head == "" {
			head = r.Agent
		}
		line := fmt.Sprintf("● %-24s %8s out · ctx %3d%% · %6s/m", head,
			compact(r.Tok), int(r.Ctx*100), compact(r.Rate))
		if r.UsageWarn != "" {
			line += "   ⚠ " + r.UsageWarn
		}
		fmt.Println(line)
	}
	for _, t := range rep.Types {
		line := fmt.Sprintf("Σ %-24s %8s out · %6s/m · %d %s", t.AgentKey,
			compact(t.Tok), compact(t.Rate), t.Sessions, i18n.Tr("sessions", "个会话"))
		if t.UsageWarn != "" {
			line += "   ⚠ " + t.UsageWarn
		}
		fmt.Println(line)
	}
	// Subscription windows (real remaining) — the headline "how much room is left".
	if len(rep.Limits.Windows) > 0 {
		parts := make([]string, 0, len(rep.Limits.Windows))
		for _, w := range rep.Limits.Windows {
			parts = append(parts, fmt.Sprintf("%s %d%%", w.Label, w.PctUsed))
		}
		fmt.Println(i18n.Tr("Plan  ", "额度  ") + strings.Join(parts, " · "))
	}
	return 0
}

// compact renders token counts like the warn strings do.
func compact(n int64) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
