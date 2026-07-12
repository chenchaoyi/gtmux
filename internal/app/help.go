package app

import (
	"fmt"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

const usageEN = `Getting started:
  new here? run  gtmux doctor        — checks your setup, grouped by concern
             then gtmux doctor --fix — one-stop setup (hooks, set-titles, restore,
                                        the menu-bar app), explained + confirmed

Usage:
  gtmux [--lang=en|zh] <command> [options]
  gtmux                    (no command) prints this help

Commands:
  overview [--popup]      sessions / windows / panes summary
                          --popup is what prefix+g opens
  agents [--watch|--json] coding agents across your panes: waiting / working /
                          idle, where, and the pane id to jump to. --watch is a
                          live dashboard (↑/↓ select · enter jump · r · q);
                          --json prints a structured array (for scripts/apps)
  digest [--json]         a cognitive digest of every agent: its goal, latest
                          reply, and what it's asking you — the "one glance"
                          fleet view (and the supervisor's read surface)
  hq                      open (or focus) the supervisor (中控) agent — one
                          session that watches, reports on, and drives the rest
  usage [--json]          token usage per session + per-type rollup, with
                          layered thresholds and ahead-of-time warnings
  restore                 one terminal tab per session, attach all
    restore --pick|-p     list & choose (numbers / Enter=all / q=cancel)
    restore <name>        attach that session by name in THIS tab
    restore --one         attach the next unattached session in THIS tab
    restore --dry-run     print what would happen, change nothing
    restore --resume-agents=auto|type|off
                          after restoring, relaunch captured agent conversations
                          (claude --resume etc.) into their panes. auto runs them;
                          type pre-fills the command; off skips. Default follows
                          the autoResumeAgentSessions config (on)
  focus <name|pane-id>    jump to that session's terminal tab; a tmux pane id
                          (%N) lands on that exact window+pane
    focus --last|-l       jump to the most-recently-finished agent pane
  new [name]              create a tmux session and open a terminal tab for it
  adopt <session_id>…     move a sensed non-tmux (native) agent session into tmux
  serve [--port N]        read-only HTTP radar for the remote mobile app, behind
                          a VPN/tunnel: GET /api/agents (the --json contract),
                          /api/pane, /api/events (SSE), POST /api/focus. --bind
                          ADDR --token TOKEN (a persistent token is auto-generated
                          on first run); --relay-url URL --relay-token TOKEN point
                          push at a relay so alerts reach the phone's lock screen
  tunnel                  expose the read-only radar from ANYWHERE (no VPN app)
                          via an outbound Cloudflare tunnel; prints a public URL,
                          token, and a scannable pairing QR. Default is a STABLE
                          hosted address (pair once); --quick for an account-less
                          ephemeral URL. --port N --name LABEL
  devices [revoke <id>]   list phones paired via per-device tokens (from a short-
                          lived QR code), and revoke one (effective immediately).
                          Talks to the local radar — run while serve/tunnel is up
  doctor [--fix [--yes]]  health check, grouped by concern: tmux / restore /
                          terminal / agents+notifications. --fix sets up the
                          rest — set-titles, plugins, the Claude hook — one step
                          at a time, explaining and asking before each change
                          (--yes applies all). This is the one-stop setup.
  update [--check]        self-update to the latest release — CLI + menu-bar app
                          (--check only reports; --cli-only skips the app)
  install-hooks [--yes]   register the Claude hook directly (doctor --fix also
                          does this); --agent <codex|cursor|gemini|copilot|kiro>
                          wires another agent (codex via its additive hooks system,
                          coexisting with any existing notify)
  uninstall-hooks         reverse install-hooks; --agent <key> for another agent
  app                     launch the menu-bar app (Gtmux.app) — the status dot
                          appears in the top-right menu bar (also: menubar)
  uninstall-app           remove the menu-bar app (Gtmux.app) + its login item
                          (install it via the curl installer or macapp/build.sh)
  hook                    internal: run BY Claude Code as a hook (reads stdin);
                          writes pane state + fires the notification
  -h, --help              show this help
  -v, --version           print the version

Options:
  --lang=en|zh   output language (default en; or set GTMUX_LANG)

Notes:
  - "agents" status: ⠿ working (busy) · ⏸ waiting (blocked on YOU for a
    permission/approval — sorts to the top) · ✳ idle (finished its turn, your
    move). waiting needs claude-notify (Claude Code's permission Notification);
    its idle-timeout nudge does NOT mark waiting, so long-idle stays idle.
  - restore/focus drive your host terminal (Ghostty 1.3+ or iTerm2) via
    AppleScript: the first run asks for Automation permission ("wants to control
    …") — allow it. The terminal is auto-detected (override: GTMUX_TERMINAL).
  - After a reboot, restore starts tmux and waits for tmux-continuum to restore
    the last autosave (layout/dirs/screen text — not running programs).
`

const usageZH = `快速开始：
  第一次用？先跑  gtmux doctor        —— 按主题分组体检你的配置
             再跑 gtmux doctor --fix —— 一站式配置（hook、set-titles、重启恢复、
                                        菜单栏 app），每步先解释并确认

用法：
  gtmux [--lang=en|zh] <命令> [选项]
  gtmux                    （不带命令）显示本帮助

命令：
  overview [--popup]      session / window / pane 汇总
                          --popup 就是 prefix+g 弹的那个弹窗
  agents [--watch|--json] 各 pane 里的 coding agent：等输入 / 运行中 / 空闲、
                          在哪、以及可跳转的 pane id。--watch 是实时面板
                          （↑/↓ 选择 · enter 跳转 · r 刷新 · q 退出）；
                          --json 输出结构化数组（给脚本 / app 用）
  digest [--json]         每个 agent 的认知摘要：目标、最新回复、正在问什么
                          —— 一眼看清全部 agent（也是中控的主要信息源）
  hq                      打开（或跳到）中控 agent —— 一个替你盯全部 agent、
                          汇报并代为驱动的会话
  usage [--json]          每会话 token 用量 + 按类型汇总；分层阈值 + 按速率
                          提前预警（撞墙前告诉你）
  restore                 每个 session 一个终端 tab，全部接回
    restore --pick|-p     列出来选（编号 / 回车=全部 / q=取消）
    restore <名字>         按名字把当前 tab 接回指定 session
    restore --one         只把当前 tab 接回下一个无人连接的 session
    restore --dry-run     只打印将要做什么，不实际执行
    restore --resume-agents=auto|type|off
                          恢复后把捕获到的 agent 会话接回各窗格（claude --resume
                          等）。auto 直接执行；type 只预填命令；off 跳过。默认跟随
                          autoResumeAgentSessions 配置（默认开）
  focus <名字|pane-id>    跳到该 session 的终端 tab；给 tmux pane id（%N）
                          则精确落到那个 window+pane
    focus --last|-l       跳到最近完成的 agent pane
  new [name]              新建一个 tmux session 并为它开一个终端 tab
  adopt <session_id>…     把感知到的非 tmux（native）agent 会话转入 tmux
  serve [--port N]        给远程手机 App 的只读 HTTP 雷达，放在 VPN/隧道之后：
                          GET /api/agents（即 --json 契约）、/api/pane、
                          /api/events（SSE）、POST /api/focus。--bind ADDR
                          --token TOKEN（首次运行自动生成并持久化 token）；
                          --relay-url URL --relay-token TOKEN 把推送指向中继，
                          让 agent 提醒推到手机锁屏
  tunnel                  把只读雷达暴露到任何地方（免 VPN app）：走出站
                          Cloudflare 隧道，打印公网 URL、token 和可扫的配对
                          二维码。默认给固定的托管地址（配一次即可），--quick 走
                          免账号的临时地址。--port N --name 标签
  devices [revoke <id>]   列出用一次性 QR 码配对的手机（每设备独立 token），
                          并可吊销某台（即刻生效）。需在 serve/tunnel 运行时使用
  doctor [--fix [--yes]]  体检，按主题分组：tmux / 恢复 / 终端 / agent+通知。
                          --fix 把其余项配好（set-titles、插件、Claude hook），
                          逐项进行，每步都先解释并征求确认（--yes 全部应用）。
                          这就是一站式安装入口。
  update [--check]        自我更新到最新版，含 CLI + 菜单栏 app（--check 只检查；
                          --cli-only 只更新 CLI 不动 app）
  install-hooks [--yes]   直接注册 Claude hook（doctor --fix 也会做这件事）；
                          --agent <codex|cursor|gemini|copilot|kiro> 接入其他
                          agent（codex 走追加式 hooks 系统，与已有 notify 并存）
  uninstall-hooks         撤销 install-hooks；--agent <key> 注销其他 agent
  app                     启动菜单栏 app（Gtmux.app）—— 状态点出现在右上角
                          菜单栏（别名：menubar）
  uninstall-app           删除菜单栏 app（Gtmux.app）及登录项
                          （安装请用 curl 安装脚本或 macapp/build.sh）
  hook                    内部命令：由 Claude Code 作为 hook 调用（读 stdin）；
                          写入 pane 状态并触发通知
  -h, --help              显示本帮助
  -v, --version           打印版本号

选项：
  --lang=en|zh   输出语言（默认 en；也可用 GTMUX_LANG 环境变量设默认）

说明：
  - "agents" 状态：⠿ 运行中（忙）· ⏸ 等输入（卡在等你批准 / 授权，排最前）·
    ✳ 空闲（完成一轮，轮到你）。⏸ 需要 claude-notify（Claude Code 的权限
    Notification）；它的空闲提醒不标 ⏸，所以久置会停在 idle。
  - restore/focus 通过 AppleScript 控制宿主终端（Ghostty 1.3+ 或 iTerm2）：首次
    运行会弹自动化授权（提示「想要控制…」），点允许。终端会自动识别（可用
    GTMUX_TERMINAL 覆盖）。
  - 电脑重启后，restore 会启动 tmux 并等 tmux-continuum 恢复最近一次自动存档
    （布局 / 目录 / 屏幕文本，不含正在运行的程序）。
`

func usage() {
	fmt.Printf("gtmux %s — %s\n\n", Version, tagline())
	if i18n.Lang() == "zh" {
		fmt.Print(usageZH)
	} else {
		fmt.Print(usageEN)
	}
}
