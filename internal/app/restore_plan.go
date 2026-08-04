package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// restorePlan is what `gtmux restore` is about to bring back — the sessions in the
// last tmux-resurrect save, each annotated with the agent conversations that will be
// resumed into it. It's the SAME data behind two surfaces: the CLI prints it as a
// progress + review checklist, and the menu bar consumes `restore --plan --json` to
// expand its one-line "restore" affordance into the actual session list. Building it
// is READ-ONLY (no tmux, no side effects), so it's safe to compute before a restore
// or to poll from the app.
type restorePlan struct {
	SavePath string               `json:"savePath"`
	Sessions []restorePlanSession `json:"sessions"`
}

type restorePlanSession struct {
	Name    string             `json:"name"`
	Windows int                `json:"windows"`
	Panes   int                `json:"panes"`
	Agents  []restorePlanAgent `json:"agents,omitempty"`
}

type restorePlanAgent struct {
	Agent     string `json:"agent"`
	Loc       string `json:"loc"`
	Cwd       string `json:"cwd,omitempty"`
	SessionID string `json:"sessionId"`
	Goal      string `json:"goal,omitempty"`
	Alive     bool   `json:"alive"` // the conversation is still on disk (resumable)
}

// buildRestorePlan reads the resurrect last save + resume records into a plan. Empty
// (SavePath "") when there's nothing saved to restore.
func buildRestorePlan() restorePlan { return buildRestorePlanFrom(resurrectLastSave()) }

// buildRestorePlanFrom is buildRestorePlan over an explicit save path (the seam for
// tests); the resume records + transcripts still come from the ambient state dir.
//
// It walks the SAVE, not the resume store, and applies the same liveness gate and
// the same conversation-picking order as resumeAgents — the plan is a promise about
// what restore will do, so the two must not be able to disagree. (They used to: the
// plan listed a conversation for any pane position that had ever hosted one, which
// is exactly the phantom the resume path was injecting.)
func buildRestorePlanFrom(save string) restorePlan {
	if save == "" {
		return restorePlan{}
	}
	layout := loadSavedLayout(save)
	plan := restorePlan{SavePath: save}

	// Session rows in first-appearance order, counting distinct windows and distinct
	// window.pane positions (a set, so a duplicated save line can't inflate a count).
	at := map[string]int{}
	windows := map[string]map[string]bool{}
	positions := map[string]map[string]bool{}
	for _, sp := range layout.Panes {
		i, seen := at[sp.Session]
		if !seen {
			plan.Sessions = append(plan.Sessions, restorePlanSession{Name: sp.Session})
			i = len(plan.Sessions) - 1
			at[sp.Session] = i
			windows[sp.Session] = map[string]bool{}
			positions[sp.Session] = map[string]bool{}
		}
		windows[sp.Session][sp.Window] = true
		positions[sp.Session][sp.Window+"."+sp.Pane] = true
		plan.Sessions[i].Windows = len(windows[sp.Session])
		plan.Sessions[i].Panes = len(positions[sp.Session])
	}

	// Only panes the save shows running an agent get a conversation listed — a pane
	// that was a plain shell when the layout was saved is coming back as a plain
	// shell, whatever its (never-pruned) resume record remembers.
	records := resume.AllLocated()
	used := map[string]bool{}
	add := func(sp savedPane, rec resume.Record) {
		if rec.SessionID == "" || used[rec.SessionID] {
			return
		}
		used[rec.SessionID] = true
		_, alive := resume.Resolve(rec)
		i := at[sp.Session]
		plan.Sessions[i].Agents = append(plan.Sessions[i].Agents, restorePlanAgent{
			Agent:     rec.Agent,
			Loc:       sp.Loc,
			Cwd:       rec.Cwd,
			SessionID: rec.SessionID,
			Goal:      planGoal(rec.Agent, rec.SessionID),
			Alive:     alive,
		})
	}

	var pending []savedPane
	for _, sp := range layout.Panes {
		if !sp.evidence().allowsResume() {
			continue
		}
		if rec, ok := resume.Load(sp.Loc); ok && rec.SessionID != "" {
			add(sp, rec)
			continue
		}
		if agent, id := sp.savedSessionID(); id != "" {
			add(sp, resume.Record{Agent: agent, SessionID: id, Cwd: sp.Dir})
			continue
		}
		pending = append(pending, sp)
	}
	for _, sp := range pending {
		if rec, _ := pickCwdFallback(sp.Loc, sp.Dir, records, used, layout.Ref); rec != nil {
			add(sp, *rec)
		}
	}
	return plan
}

// locSession returns the session-name part of a locator ("session:window.pane" →
// "session"). Split on the LAST colon so a session name that itself contains a colon
// survives (the window.pane tail never does).
func locSession(loc string) string {
	if i := strings.LastIndex(loc, ":"); i >= 0 {
		return loc[:i]
	}
	return loc
}

// planGoal is the conversation's goal (its last user prompt), best-effort — "" when
// the transcript can't be read. Cheap: one tail turn.
func planGoal(agent, sessionID string) string {
	turns, err := transcript.Load(agent, sessionID, 1)
	if err != nil || len(turns) == 0 {
		return ""
	}
	return radar.Snip(turns[len(turns)-1].Prompt, 80)
}

// agentCount is the TOTAL agent count across the plan (resumable ↻ + dead ×).
func (p restorePlan) agentCount() int {
	n := 0
	for _, s := range p.Sessions {
		n += len(s.Agents)
	}
	return n
}

// resumableCount is how many agents will ACTUALLY come back — the ↻ ones whose
// transcript is still on disk. It's the number the end-of-restore "resumed N" reports,
// so the header should show this (not agentCount) or the two read as an off-by-one.
func (p restorePlan) resumableCount() int {
	n := 0
	for _, s := range p.Sessions {
		for _, a := range s.Agents {
			if a.Alive {
				n++
			}
		}
	}
	return n
}

// deadCount is the rest: agents listed with a × because their conversation is gone from
// disk and can't be resumed. Shown separately so a "17 → 16" never reads as a failure.
func (p restorePlan) deadCount() int { return p.agentCount() - p.resumableCount() }

// printRestorePlanJSON writes the plan as JSON (the menu bar's source). Always emits
// a well-formed object, `{"savePath":"","sessions":null}` when nothing is saved.
func printRestorePlanJSON() {
	b, _ := json.Marshal(buildRestorePlan())
	fmt.Println(string(b))
}

// printRestorePlan prints the plan as a human checklist — what `gtmux restore` is
// about to bring back, session by session, with the agent conversations under each.
// Used both for `restore --plan` (preview) and folded into the real restore's output.
func printRestorePlan(p restorePlan) {
	if len(p.Sessions) == 0 {
		i18n.Say("Nothing saved to restore.", "没有可恢复的存档。")
		return
	}
	// Count the RESUMABLE (↻) agents in the headline — that's the number that will
	// actually come back (and that the "resumed N" line reports). Any dead (×) ones are
	// noted separately so the two counts never look like a silent failure.
	headEN := fmt.Sprintf("%d session(s), %d agent conversation(s) to bring back", len(p.Sessions), p.resumableCount())
	headZH := fmt.Sprintf("待恢复：%d 个 session，%d 个 agent 会话", len(p.Sessions), p.resumableCount())
	if dead := p.deadCount(); dead > 0 {
		headEN += fmt.Sprintf(" (+%d with no saved transcript, shown ×)", dead)
		headZH += fmt.Sprintf("（另有 %d 个无存档记录，标记 ×）", dead)
	}
	i18n.Say(headEN+":", headZH+"：")
	for _, s := range p.Sessions {
		i18n.Say(
			fmt.Sprintf("  • %s — %d window(s), %d pane(s)", s.Name, s.Windows, s.Panes),
			fmt.Sprintf("  • %s —— %d 窗口 / %d 窗格", s.Name, s.Windows, s.Panes))
		for _, a := range s.Agents {
			mark := "↻"
			if !a.Alive {
				mark = "×" // conversation gone from disk — won't resume
			}
			goal := a.Goal
			if goal == "" {
				goal = a.Cwd
			}
			i18n.Say(
				fmt.Sprintf("      %s %s  %s", mark, a.Agent, goal),
				fmt.Sprintf("      %s %s  %s", mark, a.Agent, goal))
		}
	}
}
