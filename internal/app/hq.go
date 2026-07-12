// `gtmux hq` — the supervisor (中控) session. Spawns (or focuses, when live) a
// dedicated tmux session running the user's coding agent in the persistent hq
// home (state.HQHome()), whose seeded CLAUDE.md teaches the supervisor loop:
// read `gtmux digest --json` → judge → drill into a pane (tmux capture-pane)
// only when warranted → drive via `gtmux send` → report. The home doubles as
// the supervisor's cross-session memory: the instructions file is generated
// ONCE and never overwritten, so user edits and accumulated knowledge persist.
//
// The supervisor is deliberately "just an agent": it appears in the radar
// (marked role:"supervisor" via its cwd), jump/notifications work, and the
// phone can converse with it — no new machinery.
package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// hqSessionName is the preferred tmux session name (auto-named on collision —
// detection is by cwd, not name, so the name is cosmetic).
const hqSessionName = "HQ"

// hqAgentCommand is what gets typed into the fresh hq pane. GTMUX_HQ_AGENT
// overrides (e.g. "codex") for users whose supervisor isn't Claude.
func hqAgentCommand() string {
	if c := strings.TrimSpace(os.Getenv("GTMUX_HQ_AGENT")); c != "" {
		return c
	}
	return "claude"
}

// hqInstructionsPath is the seeded instructions file inside the hq home.
func hqInstructionsPath() string { return filepath.Join(state.HQHome(), "CLAUDE.md") }

// seedHQHome creates the hq home and writes the instructions file IF ABSENT.
// Never overwrites: the file is the user's to edit and the supervisor's place to
// accumulate knowledge. Returns whether this call seeded it.
func seedHQHome() (seeded bool, err error) {
	home := state.HQHome()
	if err := os.MkdirAll(home, 0o755); err != nil {
		return false, err
	}
	path := hqInstructionsPath()
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(hqInstructions), 0o644)
}

// findHQPane returns the pane id of a live supervisor pane ("" when none):
// any tmux pane whose cwd is the hq home. Cwd-keyed — session renames don't
// break it, and it's the same rule the radar's role field uses.
func findHQPane() string {
	home := state.HQHome()
	for _, line := range tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{pane_current_path}") {
		f := strings.SplitN(line, "\t", 2)
		if len(f) == 2 && f[1] == home {
			return f[0]
		}
	}
	return ""
}

// cmdHQ implements `gtmux hq`: focus the live supervisor, or seed + spawn one.
func cmdHQ(args []string) int {
	for _, a := range args {
		switch a {
		case "-h", "--help":
			i18n.Say("usage: gtmux hq", "用法：gtmux hq")
			i18n.Say("  Open (or focus) the supervisor (中控) agent — one session that watches,",
				"  打开（或跳到）中控 agent —— 一个替你盯全部 agent、汇报并代为驱动的会话。")
			i18n.Say("  reports on, and drives all your other agents. Home: ~/.config/gtmux/hq/",
				"  常驻目录：~/.config/gtmux/hq/（指令文件可自行编辑，知识随会话沉淀）")
			return 0
		default:
			i18n.Sae("gtmux hq: unknown option '"+a+"'", "gtmux hq: 未知选项 '"+a+"'")
			return 2
		}
	}
	if tmux.Bin == "" {
		i18n.Sae("tmux not installed (brew install tmux)", "未安装 tmux（brew install tmux）")
		return 1
	}

	seeded, err := seedHQHome()
	if err != nil {
		i18n.Sae("gtmux hq: "+err.Error(), "gtmux hq: "+err.Error())
		return 1
	}
	if seeded {
		i18n.Say("Seeded the supervisor home: "+hqInstructionsPath(),
			"已初始化中控目录："+hqInstructionsPath())
	}

	// Already live → focus it, never spawn a second.
	if pane := findHQPane(); pane != "" {
		if err := focusPaneByID(pane); err == nil {
			i18n.Say("Focused the running supervisor.", "已跳到正在运行的中控。")
			return 0
		}
		i18n.Say("A supervisor is already running (pane "+pane+").",
			"中控已在运行（pane "+pane+"）。")
		return 0
	}

	// Spawn: detached session in the hq home, type the agent command (the same
	// mechanism restore/adopt use), then open a terminal tab onto it.
	name, err := tmux.Run(append(newSessionArgs(hqSessionName), "-c", state.HQHome())...)
	if err != nil || name == "" {
		name, err = tmux.Run("new-session", "-d", "-P", "-F", "#{session_name}", "-c", state.HQHome())
	}
	if err != nil || name == "" {
		i18n.Sae("failed to create the supervisor tmux session", "创建中控 tmux session 失败")
		return 1
	}
	if pane := tmux.Display(name, "#{pane_id}"); pane != "" {
		_ = tmux.SendText(pane, hqAgentCommand(), true)
	}
	i18n.Say("Supervisor started in tmux session '"+name+"'.", "中控已在 tmux session '"+name+"' 启动。")
	if runtime.GOOS == "darwin" {
		term := terminal.Active()
		if _, err := term.SpawnTabs([]string{name}, false); err != nil {
			i18n.Sae("could not open a "+term.Name()+" tab — attach with:  tmux attach -t "+name,
				"无法打开 "+term.Name()+" tab，请手动接回：  tmux attach -t "+name)
		}
	} else {
		i18n.Say("attach with:  tmux attach -t "+name, "接回：  tmux attach -t "+name)
	}
	return 0
}

// hqInstructions is the generated-once supervisor playbook (bilingual). It is
// the DEFAULT policy: assess + report; drive conversationally; never answer
// another agent's permission prompt on the user's behalf. The user owns edits.
const hqInstructions = `# gtmux 中控 (Supervisor HQ)

You are the SUPERVISOR of every coding agent on this machine. gtmux runs them in
tmux and gives you a fleet toolbox. 你是这台机器上所有 coding agent 的中控管家。

## Toolbox 工具箱

- ` + "`gtmux digest --json`" + ` — the fleet digest: every agent's location (loc/pane_id),
  status (waiting/working/idle/running + kind), goal (its last user prompt), last
  (tail of its last reply), ask (a waiting prompt's numbered options), error/bg.
  这是你的主要信息源；平时只读它，别去逐个翻窗口。
- ` + "`gtmux agents --json`" + ` — raw radar rows (states only, no digest fields).
- ` + "`tmux capture-pane -p -t <pane_id>`" + ` — drill into ONE pane's live screen, only
  when the digest says it's worth it (waiting/errored/stuck). 需要细节才下钻。
- ` + "`gtmux send <pane_id> <text>`" + ` — type into a pane (+Enter). ` + "`--key <name>`" + ` for a
  control key. This DRIVES another agent — use it deliberately. 代用户驱动。
- ` + "`gtmux focus <pane_id>`" + ` — jump the user's terminal to that pane.

## Nudges 事件通知

gtmux may type a line like ` + "`[gtmux] waiting·permission gtmux:0.0 (%14) — <title>`" + `
into this session when another agent starts waiting. Treat it as an event, not a
user request: check its digest row, then follow the policy below.
这是事件推送，不是用户指令。

## Policy 默认守则 (the user may edit these)

1. When asked "现状/status", answer from ` + "`digest --json`" + ` — one line per agent:
   who needs the user, who's working on what, who finished. Lead with needs-you.
2. NEVER answer another agent's permission/plan/question prompt yourself — surface
   it to the user with your recommendation. 绝不代替用户回答权限/方案选择。
3. Driving (send) is fine for routine, reversible follow-ups the user asked for in
   conversation ("让它继续", "让它跑测试"). Say what you sent and to whom.
4. Keep notes: record durable, cross-project knowledge you learn (release steps,
   test harnesses, footguns) in files under this directory — it persists across
   your sessions. 把横向知识沉淀在本目录。
5. Be terse. The user reads you on a phone half the time.
`
