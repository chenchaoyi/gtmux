// `gtmux uninstall [hooks|app|all]` — one front door for removing the pieces gtmux
// installed, replacing the hyphenated `uninstall-hooks` / `uninstall-app` (which
// still work, unannounced, so nobody's muscle memory breaks).
//
// Run with no target it ASKS rather than guessing: uninstalling is destructive and
// the two targets have very different consequences — removing the hooks blinds the
// radar, removing the app kills notifications — so picking one for the user would be
// exactly the wrong kind of helpful.
package app

import (
	"bufio"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func cmdUninstall(args []string) int {
	target := ""
	rest := []string{}
	for _, a := range args {
		switch a {
		case "hooks", "app", "all":
			target = a
		case "-h", "--help":
			return uninstallUsage()
		default:
			rest = append(rest, a) // pass through (e.g. --yes) to the real command
		}
	}
	if target == "" {
		if target = askUninstallTarget(); target == "" {
			return 1
		}
	}
	switch target {
	case "hooks":
		return cmdUninstallHooks(rest)
	case "app":
		return cmdUninstallApp(rest)
	case "all":
		if rc := cmdUninstallHooks(rest); rc != 0 {
			return rc
		}
		return cmdUninstallApp(rest)
	}
	return uninstallUsage()
}

// askUninstallTarget offers the choice. It states the CONSEQUENCE of each option,
// not just its name — "hooks" means nothing to someone who has never read the docs,
// while "the radar stops seeing your agents" is the thing they actually care about.
func askUninstallTarget() string {
	if !isInteractive() {
		i18n.Sae("gtmux uninstall: say what to remove — hooks | app | all",
			"gtmux uninstall：请指定要卸载什么 —— hooks | app | all")
		return ""
	}
	i18n.Say("What should gtmux remove?", "要卸载什么？")
	i18n.Say("  1  hooks   the agent hooks — the radar stops seeing who's waiting",
		"  1  hooks   agent 钩子 —— 雷达将不再知道谁在等你")
	i18n.Say("  2  app     the menu-bar app + its login item — no more desktop notifications",
		"  2  app     菜单栏 app 及登录项 —— 桌面通知将停止")
	i18n.Say("  3  all     both", "  3  all     两者都卸载")
	i18n.Say("  q  cancel", "  q  取消")
	os.Stdout.WriteString("> ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "1", "hooks":
		return "hooks"
	case "2", "app":
		return "app"
	case "3", "all", "both":
		return "all"
	}
	i18n.Say("cancelled — nothing was removed.", "已取消 —— 什么都没有删除。")
	return ""
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

func uninstallUsage() int {
	i18n.Say("usage: gtmux uninstall [hooks|app|all]", "用法：gtmux uninstall [hooks|app|all]")
	i18n.Say("  hooks  remove the agent hooks (the radar stops seeing who's waiting)",
		"  hooks  卸载 agent 钩子（雷达将不再知道谁在等你）")
	i18n.Say("  app    remove the menu-bar app + login item (no more desktop notifications)",
		"  app    卸载菜单栏 app 及登录项（桌面通知将停止）")
	i18n.Say("  all    both", "  all    两者都卸载")
	i18n.Say("  With no target it asks, because the two have very different consequences.",
		"  不给参数时会询问 —— 两者的后果差别很大，不替你猜。")
	return 0
}
