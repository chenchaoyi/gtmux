package hq

import (
	"sync"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

// attributor answers "which pane does this pane-less record belong to?" by the only
// mechanical route there is: the agent session id the record carries, looked up
// against the pane→session bindings.
//
// It exists because a pane id can be missing for reasons outside gtmux's control. On
// 2026-08-18 Claude moved a session into a background host with no $TMUX_PANE and 28
// records over 4h18m landed with no pane while the pane itself worked on. Recovering
// them meant matching the summary PROSE against transcripts, which on a measured
// sample of three named the right session once — a heuristic, and not one whose output
// belongs anywhere near an audit trail.
//
// So the recovery happens at READ time and by lookup. Nothing is written back: the log
// stays exactly as it was appended, and the attribution is rendered as `(~%13)` — never
// as the `(%13)` a recorded pane gets, because a binding says where the conversation
// lives NOW, which is a different claim from where the event happened.
type attributor struct {
	once sync.Once
	m    map[string]string // agent session id → pane id
}

// paneFor returns the pane a session is currently bound to, "" when nothing claims it.
// The pane table is built once per read and only when a record actually needs it, so a
// stream with no pane-less records costs no tmux calls at all.
func (a *attributor) paneFor(session string) string {
	if session == "" {
		return ""
	}
	a.once.Do(func() { a.m = attributionMap(radar.GatherPanes(), resume.Load) })
	return a.m[session]
}

// attributionMap turns the pane table into session id → pane id. Kept free of tmux and
// the filesystem so the RULES are testable as rules — the first cut of this filtered to
// `Tier == "agent"` and silently attributed nothing, which a live probe caught and a
// unit test could not have.
func attributionMap(panes []radar.PaneRow, load func(string) (resume.Record, bool)) map[string]string {
	m := map[string]string{}
	for _, p := range panes {
		// Every pane with a binding, not just the agent-tier ones. The tier is the
		// RADAR's read of what a pane is running right now, and this failure mode is
		// precisely one where that read can be wrong — the conversation is hosted
		// somewhere the pane's own command does not show. A resume record naming the
		// session is the answer regardless of how the pane is classified.
		if p.Loc == "" {
			continue
		}
		rec, ok := load(p.Loc)
		if !ok || rec.SessionID == "" {
			continue
		}
		// First binding wins: two panes claiming one session id is not something gtmux
		// can resolve, and picking the later one would just be arbitrary.
		if _, dup := m[rec.SessionID]; !dup {
			m[rec.SessionID] = p.PaneID
		}
	}
	return m
}

// attributedPane is the read-time gloss for one record: empty unless the record has no
// pane of its own AND carries a session some pane is bound to.
func (a *attributor) attributedPane(r events.Record) string {
	if r.Pane != "" {
		return ""
	}
	return a.paneFor(r.AgentSession)
}
