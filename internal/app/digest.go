// The agent-digest layer (supervisor MVP): a DETERMINISTIC, zero-LLM-token
// cognitive digest per radar row, assembled entirely from stores gtmux already
// owns — radar identity/state, the transcript (goal = the session's last user
// prompt, last = the tail of its last reply), the waiting marker's kind, the
// live prompt options (ask) for a waiting pane, and the errored/background
// modifiers. It is the supervisor's (`gtmux hq`) primary read surface and a
// human "fleet at a glance" on its own: `gtmux digest [--json]`, GET /api/digest.
//
// Design rule: every field degrades to "" when its source is absent (a session
// with no transcript still renders from radar signals alone) — agents need not
// cooperate, and the CLI stays cgo-free with zero new dependencies.
package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// digestRow is one agent's digest — the JSON contract for `gtmux digest --json`
// and GET /api/digest. Additive to (not a replacement for) `agents --json`.
type digestRow struct {
	PaneID  string `json:"pane_id,omitempty"` // tmux rows only
	Loc     string `json:"loc,omitempty"`
	Agent   string `json:"agent"`
	Source  string `json:"source"`         // "tmux" | "native"
	Status  string `json:"status"`         // working | waiting | idle | running
	Kind    string `json:"kind,omitempty"` // waiting only: permission | plan | question
	Role    string `json:"role,omitempty"` // "supervisor" for the hq session
	Project string `json:"project,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Goal    string `json:"goal,omitempty"`  // the session's last user prompt
	Last    string `json:"last,omitempty"`  // tail of the last assistant reply
	Ask     string `json:"ask,omitempty"`   // waiting only: the parsed prompt options
	Error   string `json:"error,omitempty"` // errored-idle modifier text
	Bg      string `json:"bg,omitempty"`    // background-running modifier label
	Since   int64  `json:"since,omitempty"` // epoch the current state began
	// usage-watch (usage-watch change): the session's token snapshot + the first
	// breached/projected layer. Zero/empty when no usage data (non-Claude).
	Tok       int64   `json:"tok,omitempty"`  // cumulative output tokens
	Ctx       float64 `json:"ctx,omitempty"`  // live context fraction 0–1
	Rate      int64   `json:"rate,omitempty"` // output tokens/min (recent window)
	UsageWarn string  `json:"usage_warn,omitempty"`
}

// Truncation caps: digest rows are the "短状态" tier — tens of tokens each. Deep
// context is the supervisor drilling into the pane, not a bigger digest.
const (
	goalMax = 200
	lastMax = 280
	askMax  = 240
)

// snip collapses whitespace runs to single spaces and truncates to max runes
// (rune-safe, "…"-suffixed). "" in → "" out.
func snip(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

// joinAsk renders parsed prompt options as one compact line: "1.Yes · 2.No…".
func joinAsk(opts []prompt.Option) string {
	if len(opts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(opts))
	for _, o := range opts {
		parts = append(parts, fmt.Sprintf("%d.%s", o.N, o.Label))
	}
	return snip(strings.Join(parts, " · "), askMax)
}

// turnDigest extracts (goal, last) from a session's most-recent turn. Empty
// slice → both "" (a just-started session degrades gracefully).
func turnDigest(turns []transcript.Turn) (goal, last string) {
	if len(turns) == 0 {
		return "", ""
	}
	t := turns[len(turns)-1]
	goal = snip(t.Prompt, goalMax)
	// The reply TAIL is what's most current; snip from the end, not the start.
	resp := strings.Join(strings.Fields(t.Response), " ")
	if r := []rune(resp); len(r) > lastMax {
		resp = "…" + strings.TrimSpace(string(r[len(r)-lastMax:]))
	}
	return goal, resp
}

// sessionRef resolves a row's (agentKey, sessionID) for the transcript lookup:
// tmux rows via the pane's resume record, native rows via the native store.
func sessionRef(p agentPane) (agentKey, sessionID string) {
	if p.source == "native" {
		if rec, ok := native.Load(p.sessionID); ok {
			return rec.Agent, rec.SessionID
		}
		return "", ""
	}
	if rec, ok := resume.Load(p.loc); ok && rec.SessionID != "" {
		return rec.Agent, rec.SessionID
	}
	return "", ""
}

// gatherDigest assembles the digest rows over the current radar (same ordering:
// needs-you first). Pure joins; no LLM, no new persistence.
func gatherDigest() []digestRow {
	panes := gatherAgents()
	out := make([]digestRow, 0, len(panes))
	for _, p := range panes {
		row := digestRow{
			PaneID: p.paneID, Loc: p.loc, Agent: p.agent, Source: p.source,
			Status: p.status, Role: p.role, Project: p.project, Branch: p.branch,
			Error: p.errorText, Bg: p.bgText, Since: p.since,
		}
		if p.status == "waiting" && p.paneID != "" {
			row.Kind = state.ReadMarker(state.WaitingPath(p.paneID))
			row.Ask = joinAsk(prompt.ParseOptions(tmux.CapturePane(p.paneID)))
		}
		if agentKey, sessionID := sessionRef(p); sessionID != "" {
			if turns, err := transcript.Load(agentKey, sessionID, 1); err == nil {
				row.Goal, row.Last = turnDigest(turns)
			}
			if u, ok := uwatch.ForSession(agentKey, sessionID, time.Now()); ok {
				row.Tok, row.Ctx, row.Rate = u.OutTok, u.CtxFrac, u.RatePerMin
				row.UsageWarn = uwatch.EvaluateSession(u)
			}
		}
		out = append(out, row)
	}
	return out
}

// digestJSONBytes is the machine form (CLI --json and GET /api/digest share it).
func digestJSONBytes() ([]byte, error) {
	return json.MarshalIndent(gatherDigest(), "", "  ")
}

// cmdDigest implements `gtmux digest [--json]`.
func cmdDigest(args []string) int {
	jsonOut := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			i18n.Say("usage: gtmux digest [--json]", "用法：gtmux digest [--json]")
			i18n.Say("  A cognitive digest of every agent: goal, latest reply, what it's asking.",
				"  每个 agent 的认知摘要：目标、最新回复、正在问什么。")
			return 0
		default:
			i18n.Sae("gtmux digest: unknown option '"+a+"'", "gtmux digest: 未知选项 '"+a+"'")
			return 2
		}
	}
	if jsonOut {
		b, err := digestJSONBytes()
		if err != nil {
			i18n.Sae("gtmux: "+err.Error(), "gtmux: "+err.Error())
			return 1
		}
		fmt.Println(string(b))
		return 0
	}
	rows := gatherDigest()
	if len(rows) == 0 {
		i18n.Say("No live agents.", "当前没有 agent。")
		return 0
	}
	for _, r := range rows {
		printDigestRow(r)
	}
	return 0
}

// printDigestRow renders one human block (en+zh labels via i18n).
func printDigestRow(r digestRow) {
	head := r.Loc
	if head == "" {
		head = i18n.Tr("elsewhere", "不在 tmux")
	}
	status := r.Status
	if r.Kind != "" {
		status += "·" + r.Kind
	}
	meta := ""
	if r.Project != "" {
		meta = "  [" + r.Project
		if r.Branch != "" {
			meta += " · " + r.Branch
		}
		meta += "]"
	}
	tag := ""
	if r.Role == "supervisor" {
		tag = " ⌂" + i18n.Tr("HQ", "中控")
	}
	fmt.Printf("● %s · %s · %s%s%s\n", head, r.Agent, status, tag, meta)
	if r.Goal != "" {
		fmt.Printf("  %s %s\n", i18n.Tr("goal:", "目标:"), r.Goal)
	}
	if r.Last != "" {
		fmt.Printf("  %s %s\n", i18n.Tr("last:", "最新:"), r.Last)
	}
	if r.Ask != "" {
		fmt.Printf("  %s %s\n", i18n.Tr("asks:", "问你:"), r.Ask)
	}
	if r.Error != "" {
		fmt.Printf("  %s %s\n", i18n.Tr("error:", "错误:"), snip(r.Error, lastMax))
	}
	if r.Bg != "" {
		fmt.Printf("  %s %s\n", i18n.Tr("bg:", "后台:"), snip(r.Bg, lastMax))
	}
	if r.UsageWarn != "" {
		fmt.Printf("  %s %s\n", i18n.Tr("usage:", "用量:"), r.UsageWarn)
	}
}
