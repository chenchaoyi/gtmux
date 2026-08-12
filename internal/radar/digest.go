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
	"github.com/chenchaoyi/gtmux/internal/driver"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/resource"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// DigestRow is one agent's digest — the JSON contract for `gtmux digest --json`
// and GET /api/digest. Additive to (not a replacement for) `agents --json`.
type DigestRow struct {
	PaneID string `json:"pane_id,omitempty"` // tmux rows only
	Loc    string `json:"loc,omitempty"`
	Agent  string `json:"agent"`
	Source string `json:"source"`         // "tmux" | "native"
	Status string `json:"status"`         // working | waiting | idle | running
	Kind   string `json:"kind,omitempty"` // waiting only: permission | plan | question
	Role   string `json:"role,omitempty"` // "supervisor" for the hq session
	// Verdict is the FLEET-LEVEL judgment, present ONLY on the supervisor row (hq-verdict-
	// single-source). Every surface used to derive this for itself, and they diverged: on
	// a machine at its red resource tier the menu bar said "machine under pressure" while
	// the phone's HQ page said "all normal — nothing needs you", about the same fleet at
	// the same moment. The judgment is decided here now; only the WORDING stays local.
	Verdict *HQVerdict `json:"verdict,omitempty"`
	Project string     `json:"project,omitempty"`
	Branch  string     `json:"branch,omitempty"`
	Goal    string     `json:"goal,omitempty"` // the session's last user prompt
	Last    string     `json:"last,omitempty"` // tail of the last assistant reply
	Ask     string     `json:"ask,omitempty"`  // waiting only: the parsed prompt options
	// Dispatch ledger (hq-dispatch): a pane dispatched by `gtmux spawn` carries its
	// task goal + lifecycle status. Additive + omitempty — absent for untracked panes.
	Task       string `json:"task,omitempty"`
	TaskStatus string `json:"task_status,omitempty"` // undelivered | waiting | done | working | gone
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
	// Sense grades this row's perception tier (agent-drivers): "driver" — the
	// hook feeds its state AND the transcript feeds its content; "partial" — the
	// hook is in but no structured content resolved; "screen" — pure
	// capture/process inference (Layer 1). Additive; consumers may weight trust.
	Sense string `json:"sense,omitempty"`
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
// HQVerdict is the supervisor's overall read of the fleet: WHICH state, plus the facts a
// surface needs to write a sentence about it.
//
// It deliberately carries no rendered text. A headline is user-facing prose and each
// surface owns its own language state — the phone has a follow-system/EN/中文 toggle while
// serve has a single GTMUX_LANG — so shipping a pre-rendered string would silently
// override a reader's language choice. Decide centrally, render at the edge.
//
// There is no "absent" state: the absence of a supervisor row IS that state, which every
// surface already reads correctly.
type HQVerdict struct {
	// State is priority-ordered and the ordering is part of the contract:
	// hq_call > needs_you > resource > working > normal.
	State string `json:"state"`
	// Waiting is how many WORKER sessions are blocked on the user (the supervisor is not
	// counted — its own wait is the hq_call state).
	Waiting int `json:"waiting"`
	// First names the worker that has waited LONGEST, so a one-waiter sentence can name
	// it; "" when nobody is waiting.
	First string `json:"first,omitempty"`
	// Workers is how many worker sessions exist at all, so a surface can say "N others
	// normal" without recounting rows it may have filtered differently.
	Workers int `json:"workers"`
}

// HQ verdict states. Exported so a surface's tests can reference them by name.
const (
	VerdictHQCall   = "hq_call"   // the supervisor itself is waiting on the user
	VerdictNeedsYou = "needs_you" // one or more workers are waiting on the user
	VerdictResource = "resource"  // the machine is at its critical tier
	VerdictWorking  = "working"   // the supervisor is mid-turn
	VerdictNormal   = "normal"    // a quiet fleet
)

// hqVerdict is the pure resolver (no clock, no shell) — the single ordering every surface
// now reads. resourceCritical is passed in rather than sampled here so the decision stays
// testable and the sampling stays on one, cheap path.
func hqVerdict(rows []DigestRow, resourceCritical bool) *HQVerdict {
	var hq *DigestRow
	var workers []DigestRow
	for i := range rows {
		if rows[i].Role == "supervisor" {
			hq = &rows[i]
			continue
		}
		workers = append(workers, rows[i])
	}
	if hq == nil {
		return nil // no supervisor: the row's absence is the "absent" state
	}
	v := &HQVerdict{Workers: len(workers)}
	// Longest-waiting first: the one stuck longest is the one to unblock.
	oldest := int64(0)
	for _, w := range workers {
		if w.Status != "waiting" {
			continue
		}
		v.Waiting++
		if v.First == "" || (w.Since > 0 && (oldest == 0 || w.Since < oldest)) {
			v.First, oldest = digestSessionName(w), w.Since
		}
	}
	switch {
	case hq.Status == "waiting":
		v.State = VerdictHQCall
	case v.Waiting > 0:
		v.State = VerdictNeedsYou
	case resourceCritical:
		v.State = VerdictResource
	case hq.Status == "working":
		v.State = VerdictWorking
	default:
		v.State = VerdictNormal
	}
	return v
}

// digestSessionName is the tmux session name out of a row's locator ("api:0.0" → "api"),
// falling back to the agent label for a native row that has no locator.
func digestSessionName(r DigestRow) string {
	if i := strings.Index(r.Loc, ":"); i > 0 {
		return r.Loc[:i]
	}
	if r.Loc != "" {
		return r.Loc
	}
	return r.Agent
}

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
			// Only surface an Ask line for a CLEAN, replyable menu — a rich picker
			// (AskUserQuestion beside preview panels) parses to garbage labels, so it
			// gets no fake "1.x · 2.y" ask (the user replies in the terminal).
			if opts := prompt.ParseOptions(tmux.CapturePane(p.PaneID)); prompt.OptionsReplyable(opts) {
				row.Ask = joinAsk(opts)
			}
		}
		hooked, content := false, false
		if agentKey, sessionID := sessionRef(p); sessionID != "" {
			// A resolved session record IS the hook's signature — the hook wrote it.
			hooked = true
			// Content rides the agent's driver (a pure re-wiring of transcript.Load;
			// nil where no parser is registered or the capability is switched off).
			if d := driver.For(agentKey); d.Content != nil {
				if turns, err := d.Content(sessionID, 1); err == nil {
					row.Goal, row.Last = turnDigest(turns)
					content = true
				}
			}
			if u, ok := uwatch.ForSession(agentKey, sessionID, time.Now()); ok {
				row.Tok, row.Ctx, row.Rate = u.OutTok, u.CtxFrac, u.RatePerMin
				row.UsageWarn = uwatch.EvaluateSession(u)
			}
		}
		row.Sense = senseOf(hooked, content)
		// Dispatch ledger join: if this pane was dispatched by spawn, surface its
		// tracked goal + derived lifecycle status (additive).
		if p.PaneID != "" {
			if tsk, ok := dispatch.TaskForPane(p.PaneID); ok {
				row.Task = tsk.Goal
				row.TaskStatus = TaskStatusOf(tsk, p.Status)
			}
		}
		out = append(out, row)
	}
	attachHQVerdict(out)
	return out
}

// senseOf grades a row's perception tier from facts the digest already holds
// (zero new collection): the hook's signature is the session record it wrote
// (sessionRef resolved), the content channel is a transcript that actually
// loaded. Both → "driver"; hook without content (no parser, unreadable log, or
// the capability switched off) → "partial"; neither → "screen" — the row is pure
// Layer-1 capture/process inference. Content without the hook cannot occur (the
// transcript lookup is keyed by the hook-written record).
func senseOf(hooked, content bool) string {
	switch {
	case hooked && content:
		return "driver"
	case hooked:
		return "partial"
	default:
		return "screen"
	}
}

// DigestJSONBytes is the machine form (CLI --json and GET /api/digest share it).
func DigestJSONBytes() ([]byte, error) {
	return json.MarshalIndent(GatherDigest(), "", "  ")
}

// TaskStatusUndelivered is the ledger lifecycle for a dispatch whose goal never
// reached the agent. It is NOT derivable from the pane — the pane of a failed
// dispatch is a live, empty, idle agent, indistinguishable from one that finished.
const TaskStatusUndelivered = "undelivered"

// TaskStatusOf is the ledger-AWARE mapper — the one every status view should call.
// The pane is the source of truth for what a dispatch is DOING, but only the ledger
// knows whether it was ever told to do anything, so a dispatch that never landed keeps
// its own status regardless of the pane. (Deriving from the pane alone is what rendered
// three never-delivered dispatches as green `done` on 2026-08-09 — see the
// spawn-readiness-persistent-banner change.)
func TaskStatusOf(t dispatch.Task, paneStatus string) string {
	if t.Undelivered() {
		return TaskStatusUndelivered
	}
	return TaskStatusFor(paneStatus)
}

// TaskStatusFor maps a pane's radar status to the ledger lifecycle string:
// waiting (needs you) → done (idle-after-work, review me) → working. It knows only the
// pane — callers holding a ledger entry want TaskStatusOf.
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

// attachHQVerdict computes the fleet verdict and hangs it on the supervisor row.
//
// The resource sample is paid ONLY when a supervisor row exists — on a machine with no
// HQ there is nobody to deliver a verdict to, and the digest is polled often enough that
// an unconditional sample would be a real cost for nothing. It uses the machine-only
// snapshot, never the one that shells out to a full-table `ps`.
func attachHQVerdict(rows []DigestRow) {
	hasHQ := false
	for _, r := range rows {
		if r.Role == "supervisor" {
			hasHQ = true
			break
		}
	}
	if !hasHQ {
		return
	}
	critical := resource.MachineTier(resource.MachineSnapshot()) >= resource.TierRed
	v := hqVerdict(rows, critical)
	if v == nil {
		return
	}
	for i := range rows {
		if rows[i].Role == "supervisor" {
			rows[i].Verdict = v
			return
		}
	}
}
