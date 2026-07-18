// The usage-watch fleet producer: per-session token snapshots + a per-agent-type
// rollup (summed rate judged against the type layer). Feeds `gtmux usage` and
// GET /api/usage. See openspec usage-watch.
package radar

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/resource"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// UsageRow is one session's usage + identity (the CLI/API shape).
type UsageRow struct {
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

// UsageRollup is a per-agent-type aggregate.
type UsageRollup struct {
	AgentKey  string `json:"agent_key"`
	Sessions  int    `json:"sessions"`
	Tok       int64  `json:"tok"`
	Rate      int64  `json:"rate"` // summed across the type's sessions
	UsageWarn string `json:"usage_warn,omitempty"`
}

type UsageReport struct {
	Sessions []UsageRow      `json:"sessions"`
	Types    []UsageRollup   `json:"types"`
	Limits   limits.Report   `json:"limits"`   // real subscription windows (limits-watch)
	Resource resource.Report `json:"resource"` // local machine resources (resource-watch)
}

// GatherUsage assembles rows over the current radar (radar ordering) + rollups.
func GatherUsage() UsageReport {
	now := time.Now()
	panes := GatherAgents()
	rep := UsageReport{}
	agg := map[string]*UsageRollup{}
	for _, p := range panes {
		agentKey, sessionID := sessionRef(p)
		if sessionID == "" {
			continue
		}
		u, ok := uwatch.ForSession(agentKey, sessionID, now)
		if !ok {
			continue
		}
		row := UsageRow{
			PaneID: p.PaneID, Loc: p.Loc, Agent: p.Agent, AgentKey: agentKey,
			Status: p.Status, Tok: u.OutTok, In: u.InTok, Ctx: u.CtxFrac,
			Rate: u.RatePerMin, UsageWarn: uwatch.EvaluateSession(u),
		}
		rep.Sessions = append(rep.Sessions, row)
		a := agg[agentKey]
		if a == nil {
			a = &UsageRollup{AgentKey: agentKey}
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
	// Local machine resources (cheap sampling; the warn NUDGE is emitted only from
	// the serve tick, not here — see slowTickEval — so no read-check-write race).
	rep.Resource = CurrentResource()
	return rep
}

// UsageJSONBytes serves the CLI --json and GET /api/usage identically.
func UsageJSONBytes() ([]byte, error) {
	return json.MarshalIndent(GatherUsage(), "", "  ")
}
