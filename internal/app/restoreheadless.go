package app

import (
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func headlessRestore(sessions []string, dryRun bool) int {
	if dryRun {
		i18n.Say("[dry-run] tmux sessions remain detached:", "[dry-run] tmux 会话将在后台保留：")
	} else {
		i18n.Say("tmux sessions are available. Attach locally with:", "tmux 会话已可用，可在本机连接：")
	}
	for _, name := range sessions {
		fmt.Println("  tmux attach -t " + shellQuote(name))
	}
	return 0
}
