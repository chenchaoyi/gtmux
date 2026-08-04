package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

// resumeMode is how `restore` relaunches captured agent conversations.
type resumeMode int

const (
	resumeAuto resumeMode = iota // send the resume command AND press Enter
	resumeType                   // type the command into the pane, leave it for you to run/delete
	resumeOff                    // don't touch the panes
)

// restoreResumeFlag is set from `restore --resume-agents=auto|type|off`; "" means
// fall back to the autoResumeAgentSessions config (default on → auto).
var restoreResumeFlag = ""

// effectiveResumeMode resolves the flag, else the config, else the default (auto).
func effectiveResumeMode() resumeMode {
	switch restoreResumeFlag {
	case "auto":
		return resumeAuto
	case "type":
		return resumeType
	case "off":
		return resumeOff
	}
	if autoResumeEnabled() {
		return resumeAuto
	}
	return resumeType // toggle off → type-but-don't-run (you press Enter or delete)
}

// autoResumeEnabled reads ~/.config/gtmux/config.json's autoResumeAgentSessions,
// defaulting to true (on) when the file/key is absent or unreadable.
func autoResumeEnabled() bool {
	var c struct {
		AutoResumeAgentSessions *bool `json:"autoResumeAgentSessions"`
	}
	if usercfg.Load(&c) != nil || c.AutoResumeAgentSessions == nil {
		return true
	}
	return *c.AutoResumeAgentSessions
}

// resumeAgents relaunches captured agent conversations into freshly-restored
// panes (#4). tmux-resurrect restores layout/dirs but NOT running programs, so a
// restored pane sits at a shell; for each such pane that WAS RUNNING AN AGENT when
// the layout was saved, we rebuild its `<agent> --resume <id>` command (with a cd
// into the original dir) and either run it (auto) or pre-fill it (type). Panes
// already running a program are skipped so re-running restore never clobbers a live
// agent.
//
// Two questions, two sources — keeping them apart is the whole design:
//
//   - "was an agent ALIVE in this pane?" — the tmux-resurrect save (restoresave.go).
//     It is a snapshot of the fleet minutes before the machine went down, and the
//     only witness to what was running. A pane the save shows at a bare shell is
//     never touched, no matter what its resume record remembers.
//   - "WHICH conversation?" — the resume record at the pane's locator, which the
//     hooks keep current (it follows /clear and compaction; the save's command line
//     only knows the id the process was launched with). When a pane with agent
//     evidence has no record, the save's own `--resume <id>` is the fallback, and
//     only then does the cwd+position guess run.
//
// A conversation is used at most once (dedup by session id).
func resumeAgents() {
	mode := effectiveResumeMode()
	if mode == resumeOff {
		restoreLogf("resumeAgents: mode=off — not touching panes")
		return
	}
	type shellPane struct{ id, loc, cwd string }
	var panes []shellPane
	for _, line := range tmux.Lines("list-panes", "-a", "-F",
		"#{pane_id}\t#{pane_current_command}\t#{session_name}:#{window_index}.#{pane_index}\t#{pane_current_path}") {
		f := strings.SplitN(line, "\t", 4)
		if len(f) != 4 {
			continue
		}
		if !isShellCommand(f[1]) {
			continue // a program is running here — don't type over it
		}
		panes = append(panes, shellPane{id: f[0], loc: f[2], cwd: f[3]})
	}

	// The liveness gate. With no parseable save there is no evidence to gate on, so
	// behave exactly as before rather than refusing every resume — a missing save
	// must degrade to the old behavior, not to silence.
	layout := loadSavedLayout(resurrectLastSave())
	gated := len(layout.Panes) > 0
	restoreLogf("resumeAgents: mode=%d shellPanes=%d savedPanes=%d liveness-gate=%v",
		mode, len(panes), len(layout.Panes), gated)
	if gated {
		eligible := panes[:0:0]
		for _, p := range panes {
			sp, ok := layout.ByLoc[p.loc]
			ev := evidenceMissing
			if ok {
				ev = sp.evidence()
			}
			if !ev.allowsResume() {
				restoreLogf("resume[not-live] pane=%s loc=%s cwd=%s — save says %s (cmd=%q full=%q shifted=%v); not resuming",
					p.id, p.loc, p.cwd, ev, sp.Cmd, sp.Full, sp.Shifted)
				continue
			}
			if ev == evidenceUnclear {
				restoreLogf("resume[unclear] pane=%s loc=%s — save can't name what ran (cmd=%q shifted=%v); allowing the resume",
					p.id, p.loc, sp.Cmd, sp.Shifted)
			}
			eligible = append(eligible, p)
		}
		panes = eligible
	}

	used := map[string]bool{} // session ids already resumed — never resume one twice
	n := 0
	run := func(paneID string, rec resume.Record) bool {
		if used[rec.SessionID] {
			return false
		}
		// Resume from where the conversation is actually FILED, not the cwd we last saw
		// the pane in — an agent that `cd`s mid-session moves the latter, and resuming
		// from the moved-to dir fails with "No conversation found with session ID".
		// A conversation that's gone entirely is skipped rather than left as an error
		// on the user's screen.
		rec, alive := resume.Resolve(rec)
		if !alive {
			restoreLogf("resume[skip] pane=%s session=%s — conversation not on disk (deleted/expired)", paneID, rec.SessionID)
			return false
		}
		cmd, ok := resume.Command(rec)
		if !ok {
			return false
		}
		if tmux.SendText(paneID, agentenv.Wrap(cmd), mode == resumeAuto) == nil {
			used[rec.SessionID] = true
			return true
		}
		return false
	}

	// Pass 1 — the pane's own evidence: its exact-locator record (hook-fresh), else
	// the conversation id the SAVE recorded this very pane resuming. Both are about
	// THIS pane; the guess in pass 2 is not, so it must not run first and claim a
	// conversation another pane can prove is its own.
	var pending []shellPane
	for _, p := range panes {
		rec, ok := resume.Load(p.loc)
		src := "exact"
		if !ok {
			if agent, id := layout.ByLoc[p.loc].savedSessionID(); id != "" {
				rec, ok, src = resume.Record{Agent: agent, SessionID: id, Cwd: layout.ByLoc[p.loc].Dir}, true, "save-cmd"
			}
		}
		if !ok {
			pending = append(pending, p)
			continue
		}
		ran := run(p.id, rec)
		restoreLogf("resume[%s] pane=%s loc=%s cwd=%s → session=%s ran=%v", src, p.id, p.loc, p.cwd, rec.SessionID, ran)
		if ran {
			n++
		}
	}
	// Pass 2 — fallback for panes whose exact locator had no record and whose saved
	// command line carried no id: recover a conversation ONLY when a saved record
	// shares this pane's cwd AND its original window.pane layout position (see
	// pickCwdFallback). This pass is a GUESS — the record it finds belonged to some
	// other locator — so on top of the liveness gate it also refuses records nobody
	// has touched in weeks.
	if len(pending) > 0 {
		all := resume.AllLocated() // most-recent first, each with its original locator
		for _, p := range pending {
			// Prefer the directory the SAVE recorded for this pane: it is the
			// pre-reboot truth, while the live cwd is "/" whenever resurrect failed
			// to restore the pane's directory (which the shifted-line bug causes).
			cwd := p.cwd
			if d := layout.ByLoc[p.loc].Dir; d != "" {
				cwd = d
			}
			chosen, cands := pickCwdFallback(p.loc, cwd, all, used, layout.Ref)
			if chosen == nil {
				restoreLogf("resume[no-match] pane=%s loc=%s cwd=%s (no exact record; no cwd+position candidate)", p.id, p.loc, cwd)
				continue
			}
			ran := run(p.id, *chosen)
			amb := ""
			if cands > 1 {
				amb = fmt.Sprintf(" AMBIGUOUS(%d cwd+position candidates)", cands)
			}
			restoreLogf("resume[cwd-fallback] pane=%s loc=%s cwd=%s → session=%s ran=%v%s",
				p.id, p.loc, cwd, chosen.SessionID, ran, amb)
			if ran {
				n++
			}
		}
	}

	restoreLogf("resumeAgents: done resumed=%d", n)
	reportResume(mode, n)
}

// fallbackMaxAge bounds how old a record may be to be recovered by the CWD FALLBACK
// (never by an exact-locator match, which is evidence rather than a guess). Measured
// against the save's own timestamp, so it asks "had anyone touched this conversation
// in the fortnight before the machine went down?" — a guess drawn from an older
// record is far likelier to be a ghost locator than the conversation that was live.
const fallbackMaxAge = 14 * 24 * time.Hour

// pickCwdFallback chooses the record to recover into a restored pane whose exact
// locator had no saved record. A candidate must share the pane's cwd AND the same
// window.pane layout position (session may have been renamed, but tmux-resurrect
// preserves window/pane indices) — position agreement is the evidence that THIS
// pane hosted THAT conversation. A pane that only shares a directory (a plain shell
// that never ran an agent) matches no record at its position and gets nil, which is
// what stops historical conversations being injected into every project-dir pane
// after a restore. Records are most-recent first; the newest matching one wins.
// candidates counts the position+cwd matches so an ambiguous recovery stays logged.
//
// ref is the save's timestamp; candidates older than fallbackMaxAge before it are
// skipped. A zero ref disables that check (no dated evidence to compare against).
func pickCwdFallback(loc, cwd string, all []resume.Located, used map[string]bool, ref time.Time) (rec *resume.Record, candidates int) {
	if cwd == "" {
		return nil, 0
	}
	wp := posSuffix(loc)
	for i := range all {
		r := &all[i]
		if r.Cwd != cwd || used[r.SessionID] || posSuffix(r.Loc) != wp {
			continue
		}
		if tooStaleToGuess(r.Record, ref) {
			continue
		}
		candidates++
		if rec == nil {
			rec = &all[i].Record
		}
	}
	return rec, candidates
}

// tooStaleToGuess reports whether a record is too old to be worth guessing from.
func tooStaleToGuess(r resume.Record, ref time.Time) bool {
	if ref.IsZero() || r.UpdatedAt == 0 {
		return false // nothing to compare — don't invent a reason to drop it
	}
	return ref.Sub(time.Unix(r.UpdatedAt, 0)) > fallbackMaxAge
}

// posSuffix returns the window.pane part of a locator ("session:window.pane" →
// "window.pane"). It splits on the LAST colon so a session name containing a colon
// doesn't corrupt the position. A locator with no colon returns unchanged.
func posSuffix(loc string) string {
	if i := strings.LastIndex(loc, ":"); i >= 0 {
		return loc[i+1:]
	}
	return loc
}

// reportResume prints the outcome AND (when something resumed) posts a menu-bar
// notification, so the user sees that restore brought their conversations back —
// the old code was silent when nothing matched, which read as "it didn't work".
func reportResume(mode resumeMode, n int) {
	if n == 0 {
		i18n.Say("No saved agent conversations matched the restored panes.",
			"没有可接回的 agent 会话（无匹配记录，或窗格已在运行）。")
		return
	}
	if mode == resumeAuto {
		i18n.Say(fmt.Sprintf("↻ resumed %d agent conversation(s).", n),
			fmt.Sprintf("↻ 已接回 %d 个 agent 会话。", n))
		notify.Send(notify.Options{
			Kind:    "done",
			Title:   "gtmux",
			Message: fmt.Sprintf("↻ 已接回 %d 个 agent 会话", n),
		})
	} else {
		i18n.Say(fmt.Sprintf("↻ pre-filled %d agent resume command(s) — press Enter in each pane to run.", n),
			fmt.Sprintf("↻ 已在 %d 个窗格预填 agent 接回命令，按 Enter 执行。", n))
		notify.Send(notify.Options{
			Kind:    "done",
			Title:   "gtmux",
			Message: fmt.Sprintf("↻ 已在 %d 个窗格预填接回命令", n),
		})
	}
}

// isShellCommand reports whether a pane's foreground command is an interactive
// shell (login shells show up as "-bash" etc.), i.e. nothing is running there.
func isShellCommand(name string) bool {
	switch strings.TrimPrefix(name, "-") {
	case "bash", "zsh", "fish", "sh", "dash", "tcsh", "ksh":
		return true
	}
	return false
}
