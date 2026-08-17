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
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

// selfPath is the absolute path to this binary (for popup re-exec).
func selfPath() string {
	if p, err := os.Executable(); err == nil && p != "" {
		return p
	}
	return "gtmux"
}

// resolveLang resolves the default output language: GTMUX_LANG when set to a
// known value; else config.json's "lang" (the MACHINE-level choice — the one
// that keeps a launchd-started serve, hook subprocesses, and the user's shell
// speaking the same language, since none of them share an environment); else
// the system locale (LC_ALL over LANG, POSIX order — a zh* locale reads Chinese
// without any gtmux-specific setup); else English. A set-but-unknown value at
// either explicit layer is an explicit (if broken) choice and does NOT fall
// through — except the config value "auto", which deliberately means "follow
// the locale". (A global --lang flag is applied after this and beats all of it.)
func resolveLang(get func(string) string, cfgLang string) string {
	if l := get("GTMUX_LANG"); l != "" {
		if l == "zh" || l == "en" {
			return l
		}
		return "en"
	}
	if cfgLang != "" && cfgLang != "auto" {
		if cfgLang == "zh" || cfgLang == "en" {
			return cfgLang
		}
		return "en"
	}
	for _, k := range []string{"LC_ALL", "LANG"} {
		if strings.HasPrefix(get(k), "zh") {
			return "zh"
		}
	}
	return "en"
}

// Run is the CLI entry point. It resolves the language, dispatches the
// subcommand, and returns the process exit code.
func Run(argv []string) int {
	// Default language: env, then the machine-level config, then the locale; a
	// global --lang=en|zh flag overrides all of it.
	var lc struct {
		Lang string `json:"lang"`
	}
	_ = usercfg.Load(&lc) // malformed/missing config → empty, i.e. locale fallback
	i18n.SetLang(resolveLang(os.Getenv, lc.Lang))
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
	case "panes":
		return cmdPanes(args)
	case "digest", "dg":
		return cmdDigest(args)
	case "usage":
		return cmdUsage(args)
	case "events":
		return hq.CmdEvents(args)
	case "resource", "res":
		return cmdResource(args)
	case "limits":
		return cmdLimits(args)
	case "quiet":
		return cmdQuiet(args)
	case "awake", "server-mode": // server-mode: the pre-0.44.1 name, still accepted
		return cmdServerMode(sub, args)
	case "config":
		return cmdConfig(args)
	case "share":
		return cmdShare(args)
	case "hq":
		return hq.CmdHQ(args)
	case "status", "st":
		return cmdStatus(args)
	case "spawn":
		return cmdSpawn(args)
	case "tasks":
		return hq.CmdTasks(args)
	case "capture":
		return hq.CmdCapture(args)
	case "knowledge":
		return hq.CmdKnowledge(args)
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
	case "oneshot-run": // hidden: the in-pane runner behind `gtmux spawn --oneshot`
		return cmdOneshotRun(args)
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
	case "whatsnew":
		return cmdWhatsnew(args)
	case "install":
		return cmdInstall(args)
	case "install-hooks": // pre-0.44.1 name, still accepted
		return cmdInstallHooks(args)
	case "uninstall":
		return cmdUninstall(args)
	case "uninstall-hooks": // pre-0.44.1 name, still accepted
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
