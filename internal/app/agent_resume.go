package app

import (
	"fmt"
	"strings"

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
// restored pane sits at a shell; for each such pane that has a resume record, we
// rebuild its `<agent> --resume <id>` command (with a cd into the original dir)
// and either run it (auto) or pre-fill it (type). Panes already running a program
// are skipped so re-running restore never clobbers a live agent.
//
// Matching is two-pass: first the exact locator (session:window.pane, the key the
// hook saved), then a fallback (pickCwdFallback) that recovers a conversation only
// when a saved record shares the pane's cwd AND its window.pane position — so a
// session renamed since the save still resumes, but a bare shell pane that merely
// shares a project dir is never injected with a historical conversation.
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
	restoreLogf("resumeAgents: mode=%d shellPanes=%d", mode, len(panes))

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

	// Pass 1 — exact locator match; collect the misses for the CWD fallback.
	var pending []shellPane
	for _, p := range panes {
		if rec, ok := resume.Load(p.loc); ok {
			ran := run(p.id, rec)
			restoreLogf("resume[exact] pane=%s loc=%s cwd=%s → session=%s ran=%v", p.id, p.loc, p.cwd, rec.SessionID, ran)
			if ran {
				n++
			}
		} else {
			pending = append(pending, p)
		}
	}
	// Pass 2 — fallback for panes whose exact locator had no record: recover a
	// conversation ONLY when a saved record shares this pane's cwd AND its original
	// window.pane layout position (see pickCwdFallback). The position requirement is
	// what stops the "multiple cc sessions after restore" bug: a bare shell pane that
	// merely sits in a project dir (never hosted an agent) has no record at its
	// position, so it correctly gets nothing — the old cwd-only match injected a
	// historical conversation into every such pane.
	if len(pending) > 0 {
		all := resume.AllLocated() // most-recent first, each with its original locator
		for _, p := range pending {
			chosen, cands := pickCwdFallback(p.loc, p.cwd, all, used)
			if chosen == nil {
				restoreLogf("resume[no-match] pane=%s loc=%s cwd=%s (no exact record; no cwd+position candidate)", p.id, p.loc, p.cwd)
				continue
			}
			ran := run(p.id, *chosen)
			amb := ""
			if cands > 1 {
				amb = fmt.Sprintf(" AMBIGUOUS(%d cwd+position candidates)", cands)
			}
			restoreLogf("resume[cwd-fallback] pane=%s loc=%s cwd=%s → session=%s ran=%v%s",
				p.id, p.loc, p.cwd, chosen.SessionID, ran, amb)
			if ran {
				n++
			}
		}
	}

	restoreLogf("resumeAgents: done resumed=%d", n)
	reportResume(mode, n)
}

// pickCwdFallback chooses the record to recover into a restored pane whose exact
// locator had no saved record. A candidate must share the pane's cwd AND the same
// window.pane layout position (session may have been renamed, but tmux-resurrect
// preserves window/pane indices) — position agreement is the evidence that THIS
// pane hosted THAT conversation. A pane that only shares a directory (a plain shell
// that never ran an agent) matches no record at its position and gets nil, which is
// what stops historical conversations being injected into every project-dir pane
// after a restore. Records are most-recent first; the newest matching one wins.
// candidates counts the position+cwd matches so an ambiguous recovery stays logged.
func pickCwdFallback(loc, cwd string, all []resume.Located, used map[string]bool) (rec *resume.Record, candidates int) {
	if cwd == "" {
		return nil, 0
	}
	wp := posSuffix(loc)
	for i := range all {
		r := &all[i]
		if r.Cwd != cwd || used[r.SessionID] || posSuffix(r.Loc) != wp {
			continue
		}
		candidates++
		if rec == nil {
			rec = &all[i].Record
		}
	}
	return rec, candidates
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
