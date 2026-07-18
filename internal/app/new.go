package app

import (
	"runtime"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmdNew implements `gtmux new [name]`: create a detached tmux session (tmux
// auto-names it when no name is given) and open a Ghostty tab attached to it.
// Usable from the CLI and as the menu-bar app's "New session" action.
func cmdNew(args []string) int {
	if tmux.Bin == "" {
		i18n.Sae("tmux not installed (brew install tmux)", "未安装 tmux（brew install tmux）")
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
	radar.PreflightResource() // warn (not block) if a machine resource is at its red line

	// tmux uses '.' and ':' as target separators (session:window.pane), so a name
	// carrying them can't be addressed — swap them for '-'.
	name = strings.NewReplacer(".", "-", ":", "-").Replace(strings.TrimSpace(name))
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
		i18n.Say("attach with:  tmux attach -t "+created, "接回：  tmux attach -t "+created)
		return 0
	}
	term := terminal.Active()
	if _, err := term.SpawnTabs([]string{created}, false); err != nil {
		i18n.Sae("could not open a "+term.Name()+" tab — attach with:  tmux attach -t "+created,
			"无法打开 "+term.Name()+" tab，请手动接回：  tmux attach -t "+created)
		return 1
	}
	return 0
}
