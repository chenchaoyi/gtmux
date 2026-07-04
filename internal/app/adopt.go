package app

import (
	"runtime"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

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
			usage()
			return 0
		default:
			sids = append(sids, a)
		}
	}
	if len(sids) == 0 {
		i18n.Sae("usage: gtmux adopt <session_id> [<session_id>…]   (resume a native session in tmux)",
			"用法：gtmux adopt <session_id> [<session_id>…]   （在 tmux 里恢复一个 native 会话）")
		return 1
	}

	var created []string
	failed := 0
	for _, sid := range sids {
		rec, ok := native.Load(sid)
		if !ok {
			i18n.Sae("no native session "+sid, "没有 native 会话 "+sid)
			failed++
			continue
		}
		cmd, ok := resume.Command(resume.Record{Agent: rec.Agent, SessionID: rec.SessionID, Cwd: rec.Cwd})
		if !ok {
			i18n.Sae(rec.Agent+" can't be resumed by id — skipping "+sid,
				rec.Agent+" 无法按 id 恢复，跳过 "+sid)
			failed++
			continue
		}
		name, err := tmux.Run("new-session", "-d", "-P", "-F", "#{session_name}")
		if err != nil || name == "" {
			i18n.Sae("failed to create a tmux session for "+sid, "为 "+sid+" 创建 tmux session 失败")
			failed++
			continue
		}
		// Type the resume command into the new session's shell (the same mechanism
		// `restore` uses), then drop the native record — it reappears as a tmux row
		// once the resumed session's hooks fire (de-duped by session id meanwhile).
		if pane := tmux.Display(name, "#{pane_id}"); pane != "" {
			_ = tmux.SendText(pane, cmd, true)
		}
		native.Remove(sid)
		created = append(created, name)
	}

	if len(created) == 0 {
		return 1
	}
	i18n.Say("Adopted into tmux — close the original terminal(s); the resumed session takes over the conversation.",
		"已收编进 tmux —— 请关闭原终端，恢复的会话会接管该对话。")
	if runtime.GOOS == "darwin" {
		term := terminal.Active()
		if _, err := term.SpawnTabs(created, false); err != nil {
			i18n.Sae("could not open a "+term.Name()+" tab — attach with:  tmux attach -t "+created[0],
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
