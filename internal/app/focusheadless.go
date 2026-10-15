package app

import (
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"golang.org/x/term"
	"os"
	"strings"
)

var headlessInteractive = func() bool { return tmux.InTmux() && term.IsTerminal(int(os.Stdin.Fd())) }

func headlessFocus(target string) (int, []any) {
	session := tmux.Display(target, "#{session_name}")
	if session == "" {
		i18n.Sae("Session or pane no longer exists.", "会话或窗格已不存在。")
		return 1, []any{"stage", "target-missing"}
	}
	if !headlessInteractive() {
		i18n.Say("No local interactive tmux client. Connect with:", "当前没有本地交互式 tmux 客户端，请使用：")
		fmt.Println("  tmux attach -t " + shellQuote(session))
		return 2, []any{"stage", "attach-required"}
	}
	if !tmux.OK("switch-client", "-t", session) {
		return 1, []any{"stage", "switch-client"}
	}
	if strings.HasPrefix(target, "%") {
		win := tmux.Display(target, "#{window_id}")
		if win == "" || !tmux.OK("select-window", "-t", win) || !tmux.OK("select-pane", "-t", target) {
			return 1, []any{"stage", "select-pane"}
		}
	}
	return 0, []any{"stage", "tmux-client", "session", session}
}
