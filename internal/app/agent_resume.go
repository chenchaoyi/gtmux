package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
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
	path := filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var c struct {
		AutoResumeAgentSessions *bool `json:"autoResumeAgentSessions"`
	}
	if json.Unmarshal(b, &c) != nil || c.AutoResumeAgentSessions == nil {
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
// hook saved), then a CWD fallback (resume.All → most-recent record whose Cwd is
// the pane's restored dir) so a renamed session / reindexed window still resumes.
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
	// Pass 2 — CWD fallback for panes whose locator didn't match a record. This is a
	// HEURISTIC: when several sessions share a cwd it can resume the WRONG one into a
	// pane. It's logged with the candidate count so an AMBIGUOUS match (>1) — the prime
	// suspect for a "window came back on the wrong conversation" report — is traceable.
	if len(pending) > 0 {
		all := resume.All() // most-recent first
		for _, p := range pending {
			var chosen *resume.Record
			cands := 0
			for i := range all {
				if all[i].Cwd != "" && all[i].Cwd == p.cwd && !used[all[i].SessionID] {
					cands++
					if chosen == nil {
						chosen = &all[i]
					}
				}
			}
			if chosen == nil {
				restoreLogf("resume[no-match] pane=%s loc=%s cwd=%s (no exact record, no cwd candidate)", p.id, p.loc, p.cwd)
				continue
			}
			ran := run(p.id, *chosen)
			amb := ""
			if cands > 1 {
				amb = fmt.Sprintf(" AMBIGUOUS(%d cwd candidates — may be the wrong conversation)", cands)
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
