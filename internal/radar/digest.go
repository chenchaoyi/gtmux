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
package radar

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// DigestRow is one agent's digest — the JSON contract for `gtmux digest --json`
// and GET /api/digest. Additive to (not a replacement for) `agents --json`.
type DigestRow struct {
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
	// input-lock modifier: the pane is in tmux copy/view-mode, so typed input is
	// swallowed until it exits (send/spawn auto-exit before delivering). Flags which
	// pane is input-locked so the supervisor sees it. Absent = not in a mode.
	InMode bool `json:"in_mode,omitempty"`
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

// Snip collapses whitespace runs to single spaces and truncates to max runes
// (rune-safe, "…"-suffixed). "" in → "" out.
func Snip(s string, max int) string {
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
	return Snip(strings.Join(parts, " · "), askMax)
}

// turnDigest extracts (goal, last) from a session's most-recent turn. Empty
// slice → both "" (a just-started session degrades gracefully).
func turnDigest(turns []transcript.Turn) (goal, last string) {
	if len(turns) == 0 {
		return "", ""
	}
	t := turns[len(turns)-1]
	goal = Snip(t.Prompt, goalMax)
	// The reply TAIL is what's most current; Snip from the end, not the start.
	resp := strings.Join(strings.Fields(t.Response), " ")
	if r := []rune(resp); len(r) > lastMax {
		resp = "…" + strings.TrimSpace(string(r[len(r)-lastMax:]))
	}
	return goal, resp
}

// sessionRef resolves a row's (agentKey, sessionID) for the transcript lookup:
// tmux rows via the pane's resume record, native rows via the native store.
func sessionRef(p Pane) (agentKey, sessionID string) {
	if p.source == "native" {
		if rec, ok := native.Load(p.sessionID); ok {
			return rec.Agent, rec.SessionID
		}
		return "", ""
	}
	if rec, ok := resume.Load(p.Loc); ok && rec.SessionID != "" {
		return rec.Agent, rec.SessionID
	}
	return "", ""
}

// GatherDigest assembles the digest rows over the current radar (same ordering:
// needs-you first). Pure joins; no LLM, no new persistence.
func GatherDigest() []DigestRow {
	panes := GatherAgents()
	out := make([]DigestRow, 0, len(panes))
	for _, p := range panes {
		row := DigestRow{
			PaneID: p.PaneID, Loc: p.Loc, Agent: p.Agent, Source: p.source,
			Status: p.Status, Role: p.role, Project: p.project, Branch: p.branch,
			Error: p.ErrorText, Bg: p.BgText, Since: p.Since, InMode: p.inMode,
		}
		if p.Status == "waiting" && p.PaneID != "" {
			row.Kind = state.ReadMarker(state.WaitingPath(p.PaneID))
			row.Ask = joinAsk(prompt.ParseOptions(tmux.CapturePane(p.PaneID)))
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
		if p.PaneID != "" {
			if tsk, ok := dispatch.TaskForPane(p.PaneID); ok {
				row.Task = tsk.Goal
				row.TaskStatus = TaskStatusFor(p.Status)
			}
		}
		out = append(out, row)
	}
	return out
}

// DigestJSONBytes is the machine form (CLI --json and GET /api/digest share it).
func DigestJSONBytes() ([]byte, error) {
	return json.MarshalIndent(GatherDigest(), "", "  ")
}

// TaskStatusFor maps a pane's radar status to the ledger lifecycle string:
// waiting (needs you) → done (idle-after-work, review me) → working.
func TaskStatusFor(paneStatus string) string {
	switch paneStatus {
	case "waiting":
		return "waiting"
	case "idle":
		return "done"
	default:
		return "working"
	}
}
