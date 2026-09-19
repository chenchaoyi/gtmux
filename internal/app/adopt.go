package app

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// adoptSessionName derives a meaningful tmux session name from the agent's cwd
// (its project basename), sanitized to tmux's rules (no '.'/':'/whitespace). ""
// when there's nothing usable → the caller lets tmux auto-name.
func adoptSessionName(cwd string) string {
	base := filepath.Base(strings.TrimRight(cwd, "/"))
	if base == "" || base == "." || base == "/" {
		return ""
	}
	name := strings.Map(func(r rune) rune {
		switch r {
		case '.', ':', ' ', '\t':
			return '-'
		}
		return r
	}, base)
	return strings.Trim(name, "-")
}

// newSessionArgs builds the detached-session create args, naming it when we have
// a usable name (else tmux auto-names).
func newSessionArgs(name string) []string {
	args := []string{"new-session", "-d", "-P", "-F", "#{session_name}"}
	if name != "" {
		args = append(args, "-s", name)
	}
	return args
}

// exitOriginal sends SIGTERM to the original agent process recorded at hook time,
// so a "move to tmux" leaves ONE live instance. Guards against PID reuse (kills
// only if the pid is still that command); a no-op when the pid is unknown/gone.
func exitOriginal(rec native.Record) bool {
	if rec.PID <= 0 {
		return false
	}
	if rec.Comm != "" && processComm(rec.PID) != rec.Comm {
		return false // pid gone or reused by a different command — don't touch it
	}
	return syscall.Kill(rec.PID, syscall.SIGTERM) == nil
}

// processComm returns a pid's short command name ("" if gone).
func processComm(pid int) string {
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return filepath.Base(strings.TrimSpace(string(out)))
}

// cmdAdopt implements `gtmux adopt <session_id> [<session_id>…]`: bring one or
// more sensed non-tmux (native) agent sessions under tmux by RESUMING each
// conversation in a fresh tmux session + terminal tab. It does NOT touch the
// original process — the resumed session takes over the conversation, so the user
// should close the original terminal. Only sessions whose agent is resumable can
// be adopted; the caller (menu bar) hides Adopt for the rest via the `adoptable`
// field on the native radar row.
func cmdAdopt(args []string) int {
	if tmux.Bin == "" {
		i18n.Sae("tmux not installed (brew install tmux)", "未安装 tmux（brew install tmux）")
		return 1
	}
	var sids []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			commandHelp("adopt")
			return 0
		default:
			sids = append(sids, a)
		}
	}
	if len(sids) == 0 {
		i18n.Sae("usage: gtmux adopt <conversation_id> […]   (take a conversation running outside tmux into tmux)",
			"用法：gtmux adopt <对话 id> […]   （把 tmux 之外跑着的一段对话收进 tmux）")
		return 1
	}

	var created []string
	failed := 0
	for _, sid := range sids {
		rec, ok := native.Load(sid)
		if !ok {
			diag.Did("act.adopt", sid, diag.Refused, "no conversation outside tmux has that id", "reason", "unknown")
			i18n.Sae("no conversation outside tmux with id "+sid, "tmux 之外没有 id 为 "+sid+" 的对话")
			failed++
			continue
		}
		cmd, ok := resume.Command(resume.Record{Agent: rec.Agent, SessionID: rec.SessionID, Cwd: rec.Cwd})
		if !ok {
			diag.Did("act.adopt", sid, diag.Refused, "that agent cannot resume a conversation by id",
				"reason", "not-resumable", "agent", rec.Agent)
			i18n.Sae(rec.Agent+" can't be resumed by id, skipping "+sid,
				rec.Agent+" 无法按 id 恢复，跳过 "+sid)
			failed++
			continue
		}
		name, err := tmux.Run(newSessionArgs(adoptSessionName(rec.Cwd))...)
		if err != nil || name == "" {
			// A name collision (or bad name) → let tmux auto-name.
			name, err = tmux.Run("new-session", "-d", "-P", "-F", "#{session_name}")
		}
		if err != nil || name == "" {
			diag.Did("act.adopt", sid, diag.Failed, "no tmux session could be created for it", "error", err)
			i18n.Sae("failed to create a tmux session for "+sid, "为 "+sid+" 创建 tmux session 失败")
			failed++
			continue
		}
		// Type the resume command into the new session's shell (the same mechanism
		// `restore` uses).
		if pane := tmux.Display(name, "#{pane_id}"); pane != "" {
			_ = tmux.SendText(pane, agentenv.Wrap(cmd), true)
		}
		// Exit the ORIGINAL agent process so there aren't two live instances on one
		// conversation (the user's choice). Best-effort + PID-reuse guarded — skipped
		// when we couldn't identify the process at hook time.
		closed := exitOriginal(rec)
		if closed {
			i18n.Say("• closed the original "+rec.Agent+" conversation", "• 已关掉原来那段 "+rec.Agent+" 对话")
		}
		native.Remove(sid)
		created = append(created, name)
		diag.Did("act.adopt", name, diag.OK, "resumed a conversation from outside tmux in a tmux session",
			"agent", rec.Agent, "conversation", sid, "closedOriginal", closed)
	}

	if len(created) == 0 {
		return 1
	}
	i18n.Say("Moved into tmux and resumed the conversation in a new tmux session.",
		"已转入 tmux，在新的 tmux session 里恢复了该对话。")
	if runtime.GOOS == "darwin" {
		term := terminal.Active()
		if _, err := term.SpawnTabs(created, false); err != nil {
			i18n.Sae("could not open a "+term.Name()+" tab; attach with:  tmux attach -t "+created[0],
				"无法打开 "+term.Name()+" tab，请手动接回：  tmux attach -t "+created[0])
		}
	} else {
		i18n.Say("attach with:  tmux attach -t "+created[0], "接回：  tmux attach -t "+created[0])
	}
	if failed > 0 {
		return 1
	}
	return 0
}
