package app

import (
	"runtime"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmdNew implements `gtmux new [name]`: create a detached tmux session (tmux
// auto-names it when no name is given) and open a Ghostty tab attached to it.
// Usable from the CLI and as the menu-bar app's "New session" action.
func cmdNew(args []string) int {
	if tmux.Bin == "" {
		i18n.Sae("tmux not installed (brew install tmux)", "未安装 tmux (brew install tmux)")
		return 1
	}
	name := ""
	for _, a := range args {
		switch a {
		case "-h", "--help":
			usage()
			return 0
		default:
			name = a
		}
	}

	// -P -F prints the created session's name (so we know tmux's auto-name).
	create := []string{"new-session", "-d", "-P", "-F", "#{session_name}"}
	if name != "" {
		create = append(create, "-s", name)
	}
	created, err := tmux.Run(create...)
	if err != nil || created == "" {
		i18n.Sae("failed to create session", "创建 session 失败")
		return 1
	}
	i18n.Say("Created session '"+created+"'", "已创建 session '"+created+"'")

	if runtime.GOOS != "darwin" {
		i18n.Say("attach with:  tmux attach -t "+created, "接回:  tmux attach -t "+created)
		return 0
	}
	if _, err := ghostty.SpawnTabs([]string{created}, false); err != nil {
		i18n.Sae("could not open a Ghostty tab — attach with:  tmux attach -t "+created,
			"无法打开 Ghostty tab —— 请手动接回:  tmux attach -t "+created)
		return 1
	}
	return 0
}
