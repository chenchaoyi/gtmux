// Package app wires gtmux's commands together: it parses the global --lang flag,
// dispatches to each subcommand, and holds the command implementations
// (agents, overview, restore, focus, and the live watch TUI).
package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/connect"
	"github.com/chenchaoyi/gtmux/internal/hook"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// selfPath is the absolute path to this binary (for popup re-exec).
func selfPath() string {
	if p, err := os.Executable(); err == nil && p != "" {
		return p
	}
	return "gtmux"
}

// Run is the CLI entry point. It resolves the language, dispatches the
// subcommand, and returns the process exit code.
func Run(argv []string) int {
	// Default language from env; a global --lang=en|zh flag overrides it.
	if l := os.Getenv("GTMUX_LANG"); l == "zh" || l == "en" {
		i18n.SetLang(l)
	}
	var args []string
	for _, a := range argv {
		switch {
		case a == "--lang=zh":
			i18n.SetLang("zh")
		case a == "--lang=en":
			i18n.SetLang("en")
		case strings.HasPrefix(a, "--lang="):
			i18n.Sae("gtmux: unknown --lang value (use en|zh)", "gtmux: 无效的 --lang（可用 en|zh）")
			return 2
		default:
			args = append(args, a)
		}
	}

	// Bare `gtmux` (no command) prints usage. Run `gtmux overview` for the summary.
	sub := ""
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}

	switch sub {
	case "", "-h", "--help", "help":
		usage()
		return 0
	case "-v", "--version", "version":
		fmt.Println("gtmux " + Version)
		return 0
	case "overview", "ov":
		return cmdOverview(args)
	case "restore", "re":
		return cmdRestore(args)
	case "focus", "fo":
		return cmdFocus(args)
	case "agents", "ag":
		return cmdAgents(args)
	case "digest", "dg":
		return cmdDigest(args)
	case "usage":
		return cmdUsage(args)
	case "events":
		return cmdEvents(args)
	case "resource", "res":
		return cmdResource(args)
	case "limits":
		return cmdLimits(args)
	case "quiet":
		return cmdQuiet(args)
	case "config":
		return cmdConfig(args)
	case "share":
		return cmdShare(args)
	case "hq":
		return cmdHQ(args)
	case "hq-feed":
		return cmdHQFeed(args)
	case "status", "st":
		return cmdStatus(args)
	case "spawn":
		return cmdSpawn(args)
	case "tasks":
		return cmdTasks(args)
	case "reap":
		return cmdReap(args)
	case "send":
		return cmdSend(args)
	case "options", "opts":
		return cmdOptions(args)
	case "new", "n":
		return cmdNew(args)
	case "adopt":
		return cmdAdopt(args)
	case "attach":
		return connect.Run(args)
	case "pair":
		return cmdPair(args)
	case "serve":
		return cmdServe(args)
	case "tunnel":
		return cmdTunnel(args)
	case "tunnel-client": // hidden: the always-on Direct tunnel client (launchd service)
		return cmdSelfTunnelClient(args)
	case "devices":
		return cmdDevices(args)
	case "save-tab-order":
		return cmdSaveTabOrder(args)
	case "hook":
		return hook.Run(os.Stdin, args)
	case "doctor", "dr":
		return cmdDoctor(args)
	case "update", "upgrade":
		return cmdUpdate(args)
	case "install-hooks":
		return cmdInstallHooks(args)
	case "uninstall-hooks":
		return cmdUninstallHooks(args)
	case "app", "menubar":
		return cmdApp(args)
	case "uninstall-app":
		return cmdUninstallApp(args)
	default:
		i18n.Sae("gtmux: unknown command '"+sub+"' (try: overview | agents | restore | focus | --help)",
			"gtmux: 未知命令 '"+sub+"'（可用：overview | agents | restore | focus | --help）")
		return 2
	}
}
