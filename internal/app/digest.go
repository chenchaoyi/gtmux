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

	"github.com/chenchaoyi/gtmux/internal/dispatch"
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
	Goal    string `json:"goal,omitempty"` // the session's last user prompt
	Last    string `json:"last,omitempty"` // tail of the last assistant reply
	Ask     string `json:"ask,omitempty"`  // waiting only: the parsed prompt options
	// Dispatch ledger (hq-dispatch): a pane dispatched by `gtmux spawn` carries its
	// task goal + lifecycle status. Additive + omitempty — absent for untracked panes.
	Task       string `json:"task,omitempty"`
	TaskStatus string `json:"task_status,omitempty"` // waiting | done | working | gone
	Error      string `json:"error,omitempty"`       // errored-idle modifier text
	Bg         string `json:"bg,omitempty"`          // background-running modifier label
	Since      int64  `json:"since,omitempty"`       // epoch the current state began
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
		// Dispatch ledger join: if this pane was dispatched by spawn, surface its
		// tracked goal + derived lifecycle status (additive).
		if p.paneID != "" {
			if tsk, ok := dispatch.TaskForPane(p.paneID); ok {
				row.Task = tsk.Goal
				row.TaskStatus = taskStatusFor(p.status)
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
	renderDigestTable(rows)
	return 0
}

// digestSectionSpec is one of the report's fixed sections, in display order.
// needs-you leads (it's the loudest signal); errored is appended only when
// non-empty — it's a modifier on an otherwise-idle row, not a base state.
var digestSectionSpec = []struct{ key, en, zh string }{
	{"needs_input", "needs input", "需要你"},
	{"working", "working", "进行中"},
	{"completed", "completed", "已完成"},
	{"errored", "errored", "出错"},
}

// digestBucket sorts a row into one report section. A row carrying an error
// marker is pulled into "errored" regardless of its underlying status; among
// the rest, waiting leads, idle means done, and "running" (the hookless
// can't-tell-idle-from-working fallback status) folds into "working" — it's
// still an active agent, just a less certain signal — its own grey glyph
// (via statusStyle) keeps that distinction visible on the row itself.
func digestBucket(r digestRow) string {
	switch {
	case r.Error != "":
		return "errored"
	case r.Status == "waiting":
		return "needs_input"
	case r.Status == "idle":
		return "completed"
	default:
		return "working"
	}
}

// digestBadge is the row's right-side badge: the dispatch task's lifecycle
// status takes priority (it's the most actionable signal), then how many
// options a waiting prompt offers, then a usage warning or background marker.
func digestBadge(r digestRow) string {
	switch {
	case r.Task != "" && r.TaskStatus != "":
		return r.TaskStatus
	case r.Ask != "":
		return fmt.Sprintf(i18n.Tr("%d opts", "%d 项"), strings.Count(r.Ask, " · ")+1)
	case r.UsageWarn != "":
		return "⚠"
	case r.Bg != "":
		return i18n.Tr("bg", "后台")
	default:
		return ""
	}
}

// digestLabel is a row's name-column identity: its tmux location (or
// "elsewhere" for a sensed native session), tagged for the HQ supervisor row.
func digestLabel(r digestRow) string {
	name := r.Loc
	if name == "" {
		name = i18n.Tr("elsewhere", "不在 tmux")
	}
	if r.Role == "supervisor" {
		name += " ⌂"
	}
	return name
}

// renderDigestTable prints the formatted, column-aligned status report: a
// one-line summary of counts by state, then a section per state with one
// aligned row per agent — status glyph · name · goal/last (truncated to the
// terminal width) · a right badge · a right-aligned relative time. This is
// gtmux's scannable "fleet at a glance" — no prose paragraphs.
func renderDigestTable(rows []digestRow) {
	buckets := map[string][]digestRow{}
	for _, r := range rows {
		k := digestBucket(r)
		buckets[k] = append(buckets[k], r)
	}

	summary := make([]string, 0, 4)
	for _, s := range digestSectionSpec {
		n := len(buckets[s.key])
		if s.key == "errored" && n == 0 {
			continue // an exceptional bucket — only surface it when non-empty
		}
		summary = append(summary, fmt.Sprintf("%d %s", n, i18n.Tr(s.en, s.zh)))
	}
	fmt.Println(strings.Join(summary, " · "))

	nameWidth := 8
	for _, r := range rows {
		if w := i18n.DispWidth(digestLabel(r)); w > nameWidth {
			nameWidth = w
		}
	}
	if nameWidth > 24 {
		nameWidth = 24
	}
	tw := termWidth()

	for _, s := range digestSectionSpec {
		rs := buckets[s.key]
		if len(rs) == 0 {
			continue
		}
		fmt.Printf("\n%s%s (%d)%s\n", i18n.Bold, i18n.Tr(s.en, s.zh), len(rs), i18n.Reset)
		for _, r := range rs {
			printDigestTableRow(r, nameWidth, tw)
		}
	}
}

// digestBadgeWidth/digestTimeWidth are fixed column widths for the report's
// two right-hand columns — small and stable enough not to need dynamic sizing.
const (
	digestBadgeWidth = 10
	digestTimeWidth  = 6
)

// fmtAgoShort renders a unix time as a compact "Ns/Nm/Nh/Nd" — tighter than
// devices.go's fmtAgo ("just now" / "Nm ago") so it fits the report's narrow
// right-aligned time column.
func fmtAgoShort(unix int64) string {
	if unix == 0 {
		return "?"
	}
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// printDigestTableRow prints one aligned row within its section.
func printDigestTableRow(r digestRow, nameWidth, tw int) {
	// errored-idle gets its own amber ⚠ marker (never a status color) — same
	// convention as `gtmux agents`, so the glyph itself flags "look here" even
	// outside the errored section's heading.
	glyph, color := "⚠", i18n.Amber
	if r.Error == "" {
		glyph, color, _ = statusStyle(r.Status)
	}
	name := i18n.TruncDisp(digestLabel(r), nameWidth)

	fixed := 2 + 2 + nameWidth + 2 + 2 + digestBadgeWidth + 1 + digestTimeWidth
	midWidth := tw - fixed
	if midWidth < 8 {
		midWidth = 8
	}
	// Priority: an error is why the row is in this section at all, so it wins;
	// then a waiting prompt's ask, the latest reply, the original goal, and
	// finally whatever background/usage text is available — so the middle
	// column is rarely blank even for a headless/no-transcript-yet row.
	mid := r.Error
	for _, cand := range []string{r.Ask, r.Last, r.Goal, r.Bg, r.UsageWarn} {
		if mid != "" {
			break
		}
		mid = cand
	}
	mid = i18n.TruncDisp(mid, midWidth)

	fmt.Printf("  %s%s%s %s%s%s  %s  %s  %s\n",
		color, glyph, i18n.Reset,
		i18n.Bold, i18n.PadRight(name, nameWidth), i18n.Reset,
		i18n.PadRight(mid, midWidth),
		i18n.PadLeft(digestBadge(r), digestBadgeWidth),
		i18n.PadLeft(fmtAgoShort(r.Since), digestTimeWidth))
}
