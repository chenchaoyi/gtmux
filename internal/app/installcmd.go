// `gtmux install [hooks|app|all]` and `gtmux uninstall [hooks|app|all]` — a
// symmetric pair, replacing the hyphenated `install-hooks` / `uninstall-hooks` /
// `uninstall-app` (which still work, unannounced, so nobody's muscle memory breaks).
//
// With no target either one ASKS rather than guessing. That matters most for
// uninstall, where the two targets have very different consequences — removing the
// hooks blinds the radar, removing the app kills notifications — so choosing for the
// user would be exactly the wrong kind of helpful. install asks for symmetry: the
// same command shape should behave the same way.
package app

import (
	"bufio"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// splitTarget pulls the target out of args and passes everything else through, so
// flags the underlying commands understand (--agent, --yes) still reach them.
func splitTarget(args []string) (target string, rest []string, help bool) {
	for _, a := range args {
		switch a {
		case "hooks", "app", "all":
			target = a
		case "-h", "--help":
			return "", nil, true
		default:
			rest = append(rest, a)
		}
	}
	return target, rest, false
}

func cmdInstall(args []string) int {
	target, rest, help := splitTarget(args)
	if help {
		return installUsage()
	}
	if target == "" {
		if target = askTarget(true); target == "" {
			return 1
		}
	}
	switch target {
	case "hooks":
		return cmdInstallHooks(rest)
	case "app":
		return installApp()
	case "all":
		if rc := cmdInstallHooks(rest); rc != 0 {
			return rc
		}
		return installApp()
	}
	return installUsage()
}

func cmdUninstall(args []string) int {
	target, rest, help := splitTarget(args)
	if help {
		return uninstallUsage()
	}
	if target == "" {
		if target = askTarget(false); target == "" {
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

// installApp fetches + runs the official installer. The app is a SIGNED bundle, so
// unlike the hooks it cannot be assembled locally — this is the same path
// `doctor --fix` takes.
func installApp() int {
	if _, err := os.Stat(gtmuxAppPath()); err == nil {
		i18n.Say("the menu-bar app is already installed — `gtmux app` launches it.",
			"菜单栏 app 已安装 —— 用 `gtmux app` 启动它。")
		return 0
	}
	i18n.Say("Fetching the official installer (it adds the signed menu-bar app).",
		"正在拉取官方安装脚本（用于安装已签名的菜单栏 app）。")
	i18n.Say("  Why the app: desktop notifications are delivered BY it — nothing else posts them.",
		"  为什么需要它：桌面通知由它负责发出，没有它就没有通知。")
	return runInstaller(false, fetchLatestTag())
}

// askTarget offers the choice. It states the CONSEQUENCE of each option rather than
// just its name — "hooks" means nothing to someone who has not read the docs, while
// "the radar stops seeing who's waiting" is the thing they actually care about.
func askTarget(install bool) string {
	if !isInteractive() {
		verb := "uninstall"
		if install {
			verb = "install"
		}
		i18n.Sae("gtmux "+verb+": say what — hooks | app | all",
			"gtmux "+verb+"：请指定对象 —— hooks | app | all")
		return ""
	}
	if install {
		i18n.Say("What should gtmux install?", "要安装什么？")
		i18n.Say("  1  hooks   the agent hooks — how the radar sees who's waiting",
			"  1  hooks   agent 钩子 —— 雷达靠它知道谁在等你")
		i18n.Say("  2  app     the menu-bar app — it delivers desktop notifications",
			"  2  app     菜单栏 app —— 桌面通知由它发出")
	} else {
		i18n.Say("What should gtmux remove?", "要卸载什么？")
		i18n.Say("  1  hooks   the agent hooks — the radar stops seeing who's waiting",
			"  1  hooks   agent 钩子 —— 雷达将不再知道谁在等你")
		i18n.Say("  2  app     the menu-bar app + its login item — no more desktop notifications",
			"  2  app     菜单栏 app 及登录项 —— 桌面通知将停止")
	}
	i18n.Say("  3  all     both", "  3  all     两者")
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
	i18n.Say("cancelled — nothing was changed.", "已取消 —— 什么都没有改动。")
	return ""
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

func installUsage() int {
	i18n.Say("usage: gtmux install [hooks|app|all]", "用法：gtmux install [hooks|app|all]")
	i18n.Say("  hooks  install the agent hooks (how the radar sees who's waiting)",
		"  hooks  安装 agent 钩子（雷达靠它知道谁在等你）")
	i18n.Say("  app    install the menu-bar app (it delivers desktop notifications)",
		"  app    安装菜单栏 app（桌面通知由它发出）")
	i18n.Say("  all    both", "  all    两者都装")
	i18n.Say("  hooks takes --agent <key> and --yes; with no target it asks.",
		"  hooks 支持 --agent <key> 与 --yes；不给参数时会询问。")
	return 0
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
