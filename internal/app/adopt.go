package app

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// adoptOpenTabs opens a terminal tab on each created session where gtmux can (macOS);
// opened is false elsewhere, and the caller prints how to attach. Never under `go test`:
// a test that reached this opened real tabs in the developer's terminal (2026-10-06, a
// negative control that ran the old adopt logic left six empty Ghostty tabs).
var adoptOpenTabs = func(sessions []string) (opened bool, termName string, err error) {
	if runtime.GOOS != "darwin" || testing.Testing() {
		return false, "", nil
	}
	term := terminal.Active()
	_, err = term.SpawnTabs(sessions, false)
	return true, term.Name(), err
}

// adoptWaitReady says whether the resumed agent took over the new pane and reached its
// composer: the same gate spawn and hq use before they type into a pane. A test stubs it.
var adoptWaitReady = func(pane, agent string) bool {
	return dispatchbridge.WaitAgentReady(pane, agent, time.Duration(dispatch.LoadTuning().ReadyTimeout)*time.Second)
}

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
// conversation in a fresh tmux session + terminal tab, then closing the original
// process so one instance is left (exitOriginal).
//
// Two rules keep a failure from costing the conversation (%12, 2026-10-06):
//   - Eligibility is asked again here, right before anything is created, with the
//     radar's own answer (radar.AdoptRefusal): idle, resumable, on disk, not a desktop
//     thread. The menu bar hides Adopt otherwise, but a row can start a turn before
//     the click lands, and the command can be typed with any id.
//   - The original is closed, and its record dropped, only after the resumed agent is
//     seen to have taken over the new pane. Any step that fails before that (no pane,
//     the command not typed, the agent never ready) removes the tmux session this
//     command created and leaves the original running and listed, so a retry starts
//     clean. It used to carry on past each of those and close the original anyway.
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
		cmd, resumable := resume.Command(resume.Record{Agent: rec.Agent, SessionID: rec.SessionID, Cwd: rec.Cwd})
		why := radar.AdoptRefusal(rec)
		if why == "" && !resumable {
			why = "not-resumable"
		}
		if why != "" {
			refuseAdopt(sid, rec, why)
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
		// `restore` uses), then wait for the agent to take the pane over.
		pane := tmux.Display(name, "#{pane_id}")
		if pane == "" {
			undoAdopt(sid, name, "the new tmux session has no pane", "新建的 tmux session 里没有 pane", nil)
			failed++
			continue
		}
		if err := tmux.SendText(pane, agentenv.Wrap(cmd), true); err != nil {
			undoAdopt(sid, name, "the resume command could not be typed into the new session", "恢复命令没能输入到新 session 里", err)
			failed++
			continue
		}
		if !adoptWaitReady(pane, rec.Agent) {
			blocker, _ := dispatchbridge.ReadyBlocker(pane, rec.Agent)
			undoAdopt(sid, name, "the resumed "+rec.Agent+" did not come up ("+blocker+")", "恢复的 "+rec.Agent+" 没有启动起来（"+blocker+"）", nil)
			failed++
			continue
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
	switch opened, termName, err := adoptOpenTabs(created); {
	case !opened:
		i18n.Say("attach with:  tmux attach -t "+created[0], "接回：  tmux attach -t "+created[0])
	case err != nil:
		i18n.Sae("could not open a "+termName+" tab; attach with:  tmux attach -t "+created[0],
			"无法打开 "+termName+" tab，请手动接回：  tmux attach -t "+created[0])
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// refuseAdopt reports why a conversation was not moved; nothing was created.
func refuseAdopt(sid string, rec native.Record, why string) {
	switch why {
	case "desktop-client":
		// A desktop thread belongs to ChatGPT's own process. Its hook record has no
		// agent PID to exit; resuming it here would leave two clients writing the same
		// conversation while claiming it was moved.
		diag.Did("act.adopt", sid, diag.Refused, "desktop Codex session cannot be moved safely", "reason", why)
		i18n.Sae("ChatGPT desktop conversations cannot be moved into tmux", "ChatGPT 桌面版会话不能转入 tmux")
	case "not-resumable":
		diag.Did("act.adopt", sid, diag.Refused, "that agent cannot resume a conversation by id", "reason", why, "agent", rec.Agent)
		i18n.Sae(rec.Agent+" can't be resumed by id, skipping "+sid, rec.Agent+" 无法按 id 恢复，跳过 "+sid)
	case "busy":
		diag.Did("act.adopt", sid, diag.Refused, "that conversation is mid-turn", "reason", why, "agent", rec.Agent)
		i18n.Sae(sid+" is still working or waiting; move it once its turn has ended",
			sid+" 还在进行中（工作或等待回应），等这一轮结束后再转入")
	default: // "no-conversation"
		diag.Did("act.adopt", sid, diag.Refused, "nothing of that conversation is on disk to resume", "reason", why, "agent", rec.Agent)
		i18n.Sae(sid+" has nothing on disk to resume yet; try again after its first reply",
			sid+" 在磁盘上还没有可恢复的内容，等它回复过一次再试")
	}
}

// undoAdopt removes the tmux session this adopt created and says the original was left
// as it was: running, and still listed, so the move can be tried again.
func undoAdopt(sid, session, en, zh string, err error) {
	_, _ = tmux.Run("kill-session", "-t", session)
	diag.Did("act.adopt", sid, diag.Failed, en+"; the original conversation was left running", "error", err, "session", session)
	i18n.Sae("could not move "+sid+": "+en+". The original conversation is still running; nothing was closed.",
		"没能转入 "+sid+"："+zh+"。原来的对话还在运行，什么都没关。")
}
