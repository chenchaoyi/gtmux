package app

// The command table: ONE source for the three ways gtmux answers "what can you do".
//
// It answered them three different ways before, and badly. `gtmux --help` printed 129
// lines: it scrolled off the top, every command got a paragraph, the list was flat, and
// nothing marked the four commands you use daily. Every command's own `--help` printed
// that same wall, whatever you had asked about. And the facts an agent needs before
// running anything — what a flag accepts, what it refuses, whether the command changes
// something — existed only inside the error you got for guessing wrong. Three of those
// cost the supervisor on this machine a turn each in one day: which subcommands exist,
// that `--why` stops at 300 bytes, and the five values `--kind` takes.
//
// So: one table, three renderings. The screen (grouped, one line each), one command's own
// help (its flags, with their values and limits), and `--help --json` for whatever is
// reading rather than looking.
type cmdFlag struct {
	Name     string   // as typed: "--kind <k>"
	EN, ZH   string   // what it does
	Values   []string // everything it accepts; nil when open-ended
	MaxBytes int      // the ceiling it refuses above, 0 when there is none
	Requires []string // flags that must come with it
	Required bool
}

type command struct {
	Name   string // "agents"
	Args   string // "[--watch|--json]", shown after the name
	Group  string // which group it belongs to
	EN, ZH string // the one line the screen carries
	Writes bool   // does running it change anything
	Flags  []cmdFlag
	// Detail is what one command's own help adds under its flags: the paragraph that
	// used to live in the wall, kept only where it says something a flag list cannot.
	DetailEN, DetailZH string
	Internal           bool // not on the screen (still in --json): gtmux runs it, you do not
	// OwnHelp marks a command that prints its own detailed help. The table carries
	// its one line so the screen and the JSON know it exists; its flags stay where
	// they already are, and both renderings point at them.
	OwnHelp bool
}

type cmdGroup struct {
	ID     string
	EN, ZH string // the heading
	// ModeEN says what this group's commands do to the machine. An agent reads this
	// screen too and needs that before it runs one; a person reading it loses nothing.
	ModeEN, ModeZH string
	ReadsOnly      bool
	// Compact prints the group as one wrapped list of names instead of a row each.
	// It keeps the screen one screen while the commands stay discoverable.
	Compact bool
}

var helpGroups = []cmdGroup{
	{ID: "look", EN: "LOOK", ZH: "看", ModeEN: "· reads only", ModeZH: "· 只读", ReadsOnly: true},
	{ID: "go", EN: "GO THERE", ZH: "跳过去", ModeEN: "· moves your terminal", ModeZH: "· 会动你的终端"},
	{ID: "hq", EN: "HQ", ZH: "HQ", ModeEN: "· writes into panes", ModeZH: "· 会往 pane 里写字"},
	{ID: "remote", EN: "FROM YOUR PHONE", ZH: "手机上", ModeEN: "· opens a port", ModeZH: "· 会开端口"},
	{ID: "setup", EN: "SET UP AND UPDATE", ZH: "安装和更新", ModeEN: "· changes this Mac", ModeZH: "· 会改这台 Mac"},
	{ID: "more", EN: "ALSO", ZH: "其余", ModeEN: "· gtmux <command> --help for any of these", ModeZH: "· 都可以用 gtmux <命令> --help 细看", Compact: true},
}

var helpCommands = []command{
	{
		Name: "agents", Args: "[--watch|--json]", Group: "look",
		EN: "who is waiting, working, idle, and where",
		ZH: "谁在等你、谁在跑、谁空着，各在哪",
		Flags: []cmdFlag{
			{Name: "--watch", EN: "the live board: ↑↓ select, ⏎ jump, r refresh, q quit", ZH: "实时看板：↑↓ 选择，⏎ 跳转，r 刷新，q 退出"},
			{Name: "--json", EN: "the same rows as a structured array, for scripts and apps", ZH: "同样的行，输出成结构化数组，给脚本和 app 用"},
		},
		DetailEN: "Every pane with a coding agent in it, grouped by what it wants from you and sorted so the one that wants something is first. The pane id in the last column is what `gtmux focus` takes.",
		DetailZH: "每个跑着 coding agent 的 pane，按它需要你做什么分组，需要你的排在最前。最后一列的pane id 就是 `gtmux focus` 要的那个。",
	},
	{
		Name: "digest", Args: "[--json]", Group: "look",
		EN:       "what each agent is doing and asking",
		ZH:       "每个 agent 在做什么、在问什么",
		Flags:    []cmdFlag{{Name: "--json", EN: "the same digest as data", ZH: "同一份摘要的数据版"}},
		DetailEN: "Per agent: the goal it was given, the end of its last reply, and what it is asking when it waits. No model calls; this is read off what is already on disk and on screen. It is also what HQ reads.",
		DetailZH: "每个 agent 一段：它领到的目标、最新回复的结尾、以及它等待时在问什么。不调用任何模型，读的是磁盘和屏幕上已有的东西。HQ 读的也是这份。",
	},
	{
		Name: "overview", Args: "[--popup]", Group: "look",
		EN:    "sessions, windows and panes, counted",
		ZH:    "session、window、pane 各有多少",
		Flags: []cmdFlag{{Name: "--popup", EN: "the form prefix+g opens inside tmux", ZH: "tmux 里 prefix+g 弹出来的那个形态"}},
	},
	{
		Name: "usage", Args: "[--json|--activity]", Group: "look",
		EN: "tokens spent, and how much of your plan is left",
		ZH: "花了多少 token，额度还剩多少",
		Flags: []cmdFlag{
			{Name: "--json", EN: "conversations, rollups, plan windows and the last seven days", ZH: "对话、汇总、额度窗口，以及最近七天"},
			{Name: "--activity", EN: "the year as a calendar heatmap, as wide as the terminal", ZH: "把这一年画成日历热力格，终端多宽画多宽"},
		},
		DetailEN: "The plan windows lead: they are the one number local counting cannot produce. The conversation column counts each conversation since it started, which is why a single row can exceed the week's total.",
		DetailZH: "额度窗口排最前：它是本地数不出来的那个数。对话那列算的是每段对话自它开始以来的量，所以单独一行可以比整周的总量还大。",
	},
	{
		Name: "limits", Args: "[--json]", Group: "look",
		EN: "the subscription windows on their own",
		ZH: "只看订阅窗口的余量",
		Flags: []cmdFlag{
			{Name: "--json", EN: "the windows as data", ZH: "窗口的数据版"},
			{Name: "--refresh", EN: "read the plans again instead of using the cache", ZH: "不用缓存，重新读一遍额度"},
		},
		DetailEN: "Real server numbers, from what each agent itself reports: Claude by running its own `/usage` headlessly, Codex by reading the rate-limit response already in its rollout. Cached, so asking twice costs nothing.",
		DetailZH: "真实的服务端数字，来自各个 agent 自己的上报：Claude 是无界面跑它自己的 `/usage`，Codex 是直接读它 rollout 里已有的限流响应。有缓存，问第二次不花钱。",
	},

	{
		Name: "logs", Args: "[--since 1h|--follow]", Group: "look",
		EN: "what gtmux saw and did on this Mac",
		ZH: "gtmux 在这台 Mac 上看到了什么、做了什么",
		Flags: []cmdFlag{
			{Name: "--since <when>", EN: "how far back: 30m, 2h, 3d, or a date (default 1h)", ZH: "往前看多久：30m、2h、3d，或一个日期（默认 1h）"},
			{Name: "--until <when>", EN: "stop at this point", ZH: "看到这个时间为止"},
			{Name: "--component <c>", EN: "one part of gtmux", ZH: "只看 gtmux 的某一部分",
				Values: []string{"serve", "tunnel", "hook", "cli", "hq", "restore", "hygiene", "menubar", "log"}},
			{Name: "--level <l>", EN: "this level and above", ZH: "这个级别及以上",
				Values: []string{"debug", "info", "warn", "error"}},
			{Name: "--acts", EN: "only what gtmux did, not what it saw", ZH: "只看 gtmux 做了什么，不看它看到了什么"},
			{Name: "--actor <who>", EN: "who started it: user, hq, agent, menubar, phone, browser, guest, system, or one of them exactly (agent:%7)", ZH: "谁发起的：user、hq、agent、menubar、phone、browser、guest、system，或精确到某一个（agent:%7）"},
			{Name: "--event <glob>", EN: "event names, with * as a wildcard: 'act.*', act.pair", ZH: "事件名，* 为通配：'act.*'、act.pair"},
			{Name: "--follow, -f", EN: "keep printing new entries, across midnight", ZH: "持续打印新记录，跨过零点也不断"},
			{Name: "--stats", EN: "how much is kept and how much of the window went wrong, instead of the entries", ZH: "不打印条目，只说存了多少、这段时间里出了多少问题"},
			{Name: "--json", EN: "the raw entries, one per line", ZH: "原样输出，一行一条"},
		},
		DetailEN: "One store for every gtmux process, like the system log: serve, the tunnel client, the hook, every command and the menu bar write to ~/.local/share/gtmux/logs/, one file per day. It holds diagnostics and every action that changes something, with who started it, what it acted on and how it ended. Entries are English, never hold message text, and have credentials replaced. The store keeps 30 days or 100 MB, whichever comes first (logs.retainDays and logs.maxMB in config.json).",
		DetailZH: "所有 gtmux 进程共用一个日志库，和系统日志一样：serve、隧道客户端、hook、每条命令、菜单栏都写进 ~/.local/share/gtmux/logs/，每天一个文件。里面有诊断记录，也有每一次改动东西的操作，记下谁发起、作用在什么上、结果如何。记录用英文，从不包含消息正文，凭证会被替换掉。日志库保留 30 天或 100MB，先到哪个算哪个（config.json 里的 logs.retainDays 和 logs.maxMB）。",
	},
	{
		Name: "focus", Args: "<name|%pane>", Group: "go", Writes: true,
		EN:    "jump this terminal to that session or pane",
		ZH:    "跳到那个 session 或 pane",
		Flags: []cmdFlag{{Name: "--last, -l", EN: "the agent pane that finished most recently", ZH: "最近完成的那个 agent pane"}},
	},
	{
		Name: "restore", Args: "[--pick|--plan]", Group: "go", Writes: true,
		EN: "reopen every session in its own terminal tab",
		ZH: "每个 session 一个终端 tab，全接回来",
		Flags: []cmdFlag{
			{Name: "<name>", EN: "this tab joins that session", ZH: "让当前 tab 接上那个 session"},
			{Name: "--pick, -p", EN: "list them and choose: a number, Enter for all, q to cancel", ZH: "列出来选：数字 / 回车全选 / q 取消"},
			{Name: "--one", EN: "this tab joins the next session nobody is attached to", ZH: "让当前 tab 接上下一个没人连着的 session"},
			{Name: "--dry-run", EN: "print what it would do and do nothing", ZH: "只打印会做什么，不动手"},
			{Name: "--plan [--json]", EN: "what would come back, session by session; starts no tmux", ZH: "逐个 session 预览会恢复什么；不启动 tmux"},
			{Name: "--resume-agents=<mode>", EN: "what to do with the agent conversations under each pane", ZH: "每个 pane 底下的 agent 对话怎么处理",
				Values: []string{"auto", "type", "off"}},
		},
		DetailEN: "auto runs each captured conversation again (`claude --resume …`), type puts the command in the pane and leaves it unsent, off reopens the panes and starts nothing. The default follows autoResumeAgentSessions, which is auto.\n\nAfter a reboot, restore starts tmux and waits for tmux-continuum to bring back the last automatic save: layout, directories and screen text, but not the programs that were running. Those come back through --resume-agents.",
		DetailZH: "auto 把每个抓到的对话重新跑起来（`claude --resume …`），type 只把命令填进 pane 不回车，off 只恢复 pane、不起任何 agent。默认跟随 autoResumeAgentSessions，也就是 auto。\n\n电脑重启之后，restore 会启动 tmux 并等 tmux-continuum 恢复最近一次自动存档：布局、目录、屏幕文本，但不含当时正在跑的程序。那些靠 --resume-agents 回来。",
	},
	{
		Name: "new", Args: "[name]", Group: "go", Writes: true,
		EN: "start a session and open a terminal tab for it",
		ZH: "新开一个 session，并给它开一个终端 tab",
	},
	{
		Name: "adopt", Args: "<session_id>…", Group: "go", Writes: true,
		EN:       "take an agent running outside tmux into tmux",
		ZH:       "把 tmux 之外跑着的 agent 收进 tmux",
		DetailEN: "A conversation gtmux only senses (no pane, read-only on the radar) is resumed inside a fresh tmux session, after which it is a pane like any other.",
		DetailZH: "只是被感知到的对话（没有 pane，在雷达上只读）会以一个新的 tmux session 重新拉起来，之后它就和别的 pane 一样了。",
	},
	{
		Name: "panes", Args: "[--json|--watched]", Group: "go",
		EN: "every pane, agents and plain shells alike",
		ZH: "所有 pane，agent 和普通 shell 都在",
		Flags: []cmdFlag{
			{Name: "--json", EN: "the session/window/pane tree as data", ZH: "session/window/pane 这棵树的数据版"},
			{Name: "watch %N", EN: "put a plain pane on the radar as its own row", ZH: "把一个普通 pane 单独放上雷达"},
			{Name: "unwatch %N", EN: "take it off again", ZH: "再把它拿下来"},
			{Name: "--watched", EN: "just the ones you pinned", ZH: "只看你钉上去的那些"},
		},
		DetailEN: "Each pane is tagged tier=agent|plain. focus, send and attach work on any of them; this is the full set the pane browser shows.",
		DetailZH: "每个 pane 带一个 tier=agent|plain 的标记。focus、send、attach 对任何一个都有效；pane 浏览器里看到的就是这一整套。",
	},

	{
		Name: "hq", Args: "[--here|--new-pane]", Group: "hq", Writes: true,
		EN: "open HQ, the agent that watches the rest for you",
		ZH: "打开 HQ，它替你盯着其余的 agent",
		Flags: []cmdFlag{
			{Name: "--pane %N", EN: "start it in that pane", ZH: "在那个 pane 里起"},
			{Name: "--here", EN: "start it in this pane", ZH: "就在当前 pane 里起"},
			{Name: "--new-pane", EN: "split a new pane and start it there", ZH: "劈一个新 pane，在那儿起"},
			{Name: "--rotate", EN: "hand over and start a fresh conversation", ZH: "交接完，换一段新对话"},
		},
		DetailEN: "HQ watches the other agents, reports what changed while you were elsewhere, and acts within what you allow. Without a flag it opens the existing HQ, or starts one in a new session.",
		DetailZH: "HQ 盯着其余 agent，把你不在时发生的变化报给你，并在你允许的范围内动手。不带参数时它打开已有的 HQ，没有就新开一个 session 起一个。",
	},
	{
		Name: "knowledge", Args: "<verb>", Group: "hq", Writes: true,
		EN: "what HQ has learned on this machine",
		ZH: "HQ 在这台机器上学到的东西",
		Flags: []cmdFlag{
			{Name: "list [--topic t]", EN: "every live entry", ZH: "所有在库条目"},
			{Name: "show <id>", EN: "one entry in full", ZH: "看某一条全文"},
			{Name: "lint [--json]", EN: "audit the base: what to fix, never fixed for you", ZH: "给知识库做体检：只报该修什么，从不替你改"},
			{Name: "carriers", EN: "which agents carry the machine block, and is it current", ZH: "哪些 agent 装了本机块，是不是最新的"},
			{Name: "add --topic <t> --title \"…\"", EN: "record one lesson (HQ home only)", ZH: "记一条教训（只能在 HQ 家目录里跑）",
				Required: true},
			{Name: "--kind <k>", EN: "what the entry IS", ZH: "这条属于哪一类",
				Values: []string{"facts", "howto", "pitfalls", "judgment", "decisions"}},
			{Name: "--why \"…\"", EN: "the reason, on promote/retire/withdraw", ZH: "晋升、退休、撤回时的理由", MaxBytes: 300},
			{Name: "--sensitive", EN: "keep it on this Mac: never distributed, never exported", ZH: "只留本机：不分发、不导出",
				Requires: []string{"--confirmed"}},
		},
		DetailEN: "The whole list of verbs is `gtmux knowledge` with no arguments. Writing verbs are accepted only from the HQ home; list and show work anywhere.",
		DetailZH: "全部动词用不带参数的 `gtmux knowledge` 看。写入类的动词只接受来自 HQ 家目录的调用；list 和 show 在哪儿都能跑。",
	},
	{
		Name: "capture", Args: "\"<lesson> @<topic>\"", Group: "hq", Writes: true,
		EN:       "hand HQ one lesson to file",
		ZH:       "随手交给 HQ 一条教训",
		Flags:    []cmdFlag{{Name: "--list", EN: "the queue waiting for HQ to judge", ZH: "等 HQ 判定的队列"}},
		DetailEN: "The cheapest way in: any agent drops a line, HQ decides what becomes an entry.",
		DetailZH: "成本最低的入口：任何 agent 随手丢一句，由 HQ 判断哪些该成条目。",
	},

	{
		Name: "serve", Args: "[--port N]", Group: "remote", Writes: true,
		EN: "serve the radar to your phone on this network",
		ZH: "在同一网络里把雷达提供给手机",
		Flags: []cmdFlag{
			{Name: "--port N", EN: "which port to listen on", ZH: "监听哪个端口"},
			{Name: "--bind ADDR", EN: "which address to listen on", ZH: "绑哪个地址"},
			{Name: "--token TOKEN", EN: "the bearer token; one is generated and kept on first run", ZH: "bearer token；第一次跑会自动生成并存下来"},
			{Name: "--relay-url URL", EN: "point push at a relay so alerts reach the lock screen", ZH: "把推送指向中继，提醒才能到锁屏",
				Requires: []string{"--relay-token"}},
		},
		DetailEN: "A read-only HTTP radar plus a typed reply channel, behind whatever you put in front of it. The token is the whole gate: treat it as a password.",
		DetailZH: "一个只读的 HTTP 雷达，外加一条可以打字回复的通道，前面挡什么由你决定。token 就是全部门禁，当密码看待。",
	},
	{
		Name: "tunnel", Args: "[--backend|--quick]", Group: "remote", Writes: true,
		EN: "reach it from anywhere, without a VPN",
		ZH: "不用 VPN，从任何地方连上来",
		Flags: []cmdFlag{
			{Name: "--backend <b>", EN: "which way out", ZH: "走哪条出口", Values: []string{"standard", "self"}},
			{Name: "--quick", EN: "an account-less ephemeral URL", ZH: "不用账号的临时地址"},
			{Name: "--redeem <code>", EN: "unlock Direct", ZH: "解锁 Direct"},
			{Name: "--service", EN: "keep it on across reboots", ZH: "常开，重启不掉"},
		},
		DetailEN: "Prints a public URL, a token, and a QR the phone can scan. Standard is a stable hosted address you pair once; self is Direct over 443, self-hosted or unlocked.",
		DetailZH: "会打印一个公网地址、一个 token，和手机能扫的二维码。Standard 是固定的托管地址，配一次就行；self 是走 443 的 Direct，自托管或者用码解锁。",
	},
	{
		Name: "pair", Args: "[list|revoke <id>]", Group: "remote", Writes: true,
		EN:       "pair your own devices, list or revoke them",
		ZH:       "配对你自己的设备，查看或吊销",
		DetailEN: "One one-time code, three ways to use it: scan it on the phone, open it in a browser, or paste the printed `gtmux attach` line into another computer's terminal. Someone else's device goes through `share` instead, which is scoped.",
		DetailZH: "一个一次性配对码，三种用法：手机扫、浏览器打开、或者把打印出来的那行 `gtmux attach`粘进另一台电脑的终端。别人的设备走 `share`，那是带范围限制的。",
	},
	{
		Name: "attach", Args: "<target> [%N]", Group: "remote", Writes: true,
		EN: "open a remote pane in this terminal",
		ZH: "把远程的某个 pane 开在这个终端里",
		Flags: []cmdFlag{
			{Name: "--read-only", EN: "watch without typing", ZH: "只看不打字"},
			{Name: "--token TOKEN", EN: "when the target is a bare host", ZH: "目标只给了主机名时用"},
			{Name: "--code CODE", EN: "a short code someone read out to you, instead of the link", ZH: "对方念给你的短码，代替那条链接"},
		},
		DetailEN: "Your terminal becomes the remote pane, raw, over a WebSocket. A pair link (…/#c=) enrolls this terminal as your own device; a share link (…#code=) connects as a scoped guest, and --code is for when that link was read out to you rather than sent. Either is kept for that host, so later just `gtmux attach <host>`. Ctrl-] detaches.",
		DetailZH: "你的终端直接变成那个远程 pane，原生透传，走 WebSocket。配对链接（…/#c=）把这个终端登记成你自己的设备；分享链接（…#code=）以受限访客接入，--code 是对方把链接念给你、而不是发给你时用的。两者都会为那台 host 记下来，之后直接 `gtmux attach <host>`。Ctrl-] 退出。",
	},
	{
		Name: "devices", Args: "[revoke <id>]", Group: "remote", Writes: true,
		EN: "the devices that can reach this Mac",
		ZH: "能连到这台 Mac 的设备名册",
		Flags: []cmdFlag{
			{Name: "--push", EN: "inspect the push tokens", ZH: "看推送 token"},
			{Name: "--forget-push <what>", EN: "clear them", ZH: "清掉推送 token", Values: []string{"<id>", "orphans", "all"}},
		},
	},
	{
		Name: "awake", Args: "[on|off]", Group: "remote", Writes: true,
		EN:       "keep this Mac working with the lid closed",
		ZH:       "合上盖子也继续干活",
		Flags:    []cmdFlag{{Name: "--json", EN: "the current state as data", ZH: "当前状态的数据版"}},
		DetailEN: "So serve, the tunnel and the phone keep answering after you shut the lid. `on` asks for your admin password once and checks it took effect; `off` needs no password.",
		DetailZH: "这样合盖之后 serve、隧道和手机那头都还在。`on` 会问一次管理员密码并确认真的生效了；`off` 不需要密码。",
	},

	{
		Name: "doctor", Args: "[--fix [--yes] | --bundle]", Group: "setup", Writes: true,
		EN: "check this Mac, then set up what is missing",
		ZH: "体检这台 Mac，然后把缺的配上",
		Flags: []cmdFlag{
			{Name: "--fix", EN: "set the rest up, explaining and asking before each change", ZH: "把其余项配好，每一步都先解释再征求同意"},
			{Name: "--yes", EN: "apply every step without asking", ZH: "全部应用，不再逐项问", Requires: []string{"--fix"}},
			{Name: "--bundle [path]", EN: "pack logs, status, launchd output, this report and versions into one file for a bug report; tokens replaced, nothing sent", ZH: "把日志、状态、launchd 输出、这份报告和版本号打成一个文件，用于报告问题；token 已替换，不会发送"},
			{Name: "--with-events", EN: "also pack the event journal, which holds prompt heads", ZH: "连事件流一起打包，里面有提示词开头", Requires: []string{"--bundle [path]"}},
		},
		DetailEN: "Grouped by what it affects: tmux, restore, the terminal, agents and notifications. This is the way in: run it first, run it again whenever something stops working.",
		DetailZH: "按影响分组：tmux、恢复、终端、agent 和通知。这就是入口：第一次先跑它，之后哪儿不对了再跑一次。",
	},
	{
		Name: "install", Args: "[hooks|app]", Group: "setup", Writes: true,
		EN: "hooks, which are how gtmux sees agents",
		ZH: "装 hook，也就是 gtmux 看见 agent 的方式",
		Flags: []cmdFlag{
			{Name: "hooks [--yes]", EN: "register the agent hooks", ZH: "注册 agent hook"},
			{Name: "--agent <a>", EN: "wire an agent other than Claude Code", ZH: "接入 Claude Code 之外的 agent",
				Values: []string{"codex", "cursor", "gemini", "copilot", "kiro", "opencode"}},
			{Name: "app", EN: "the menu-bar app, which delivers desktop notifications", ZH: "菜单栏 app，桌面通知由它发"},
		},
		DetailEN: "No target and it asks. `doctor --fix` does the same thing as part of the walk-through.",
		DetailZH: "不给参数就问你。`doctor --fix` 在它那趟流程里也会做同一件事。",
	},
	{
		Name: "uninstall", Args: "[hooks|app]", Group: "setup", Writes: true,
		EN:       "take back out what gtmux installed",
		ZH:       "把 gtmux 装过的东西撤掉",
		DetailEN: "Without hooks the radar stops seeing agents; without the app there are no desktop notifications. No target and it asks.",
		DetailZH: "没有 hook，雷达就看不见 agent；没有 app，就没有桌面通知。不给参数就问你。",
	},
	{
		Name: "update", Args: "[--check]", Group: "setup", Writes: true,
		EN: "update gtmux, the CLI and the menu-bar app",
		ZH: "更新 gtmux，CLI 和菜单栏 app 一起",
		Flags: []cmdFlag{
			{Name: "--check", EN: "report what is available and change nothing", ZH: "只报告有没有新版，什么都不动"},
			{Name: "--cli-only", EN: "leave the app alone", ZH: "只更新 CLI，不动 app"},
		},
	},
	{
		Name: "whatsnew", Args: "[--since v]", Group: "setup",
		EN:       "read what changed, per release",
		ZH:       "看每个版本改了什么",
		DetailEN: "`update` prints a short summary; this is the whole list.",
		DetailZH: "`update` 只印个摘要，这里是全部。",
	},
	{
		Name: "app", Group: "setup", Writes: true,
		EN:       "open the menu-bar app",
		ZH:       "打开菜单栏 app",
		DetailEN: "The status dot appears in the top-right menu bar. Also spelled `menubar`. Install it with the curl installer or macapp/build.sh.",
		DetailZH: "状态点会出现在右上角菜单栏里。也可以写作 `menubar`。安装用 curl 安装脚本或者macapp/build.sh。",
	},

	{Name: "spawn", Group: "more", Writes: true, OwnHelp: true,
		EN: "launch an agent and hand it a task", ZH: "起一个 agent，并交给它一个任务"},
	{Name: "tasks", Group: "more", OwnHelp: true,
		EN: "the dispatch and needs-you ledger", ZH: "派工和待你处理的台账"},
	{Name: "reap", Group: "more", Writes: true, OwnHelp: true,
		EN: "reclaim a dispatch that finished", ZH: "回收一个已经做完的派工"},
	{Name: "send", Group: "more", Writes: true, OwnHelp: true,
		EN: "type a message into a pane", ZH: "往某个 pane 里发一句话"},
	{Name: "events", Group: "more", OwnHelp: true,
		EN: "the event stream HQ reads", ZH: "HQ 读的那条事件流"},
	{Name: "share", Group: "more", Writes: true, OwnHelp: true,
		EN: "share one pane with someone else", ZH: "把某个 pane 分享给别人"},
	{Name: "status", Group: "more", OwnHelp: true,
		EN: "one line, for scripts and prompts", ZH: "一行输出，给脚本和提示符用"},
	{Name: "config", Group: "more", Writes: true, OwnHelp: true,
		EN: "read and change gtmux's settings", ZH: "看和改 gtmux 的设置"},
	{Name: "quiet", Group: "more", Writes: true, OwnHelp: true,
		EN: "how loud HQ is allowed to be", ZH: "HQ 能吵到什么程度"},
	{Name: "resource", Group: "more", OwnHelp: true,
		EN: "disk, memory and CPU, per agent", ZH: "磁盘、内存、CPU，按 agent 分"},
	{
		Name: "hook", Group: "setup", Writes: true, Internal: true,
		EN:       "run by an agent as a hook, reads stdin",
		ZH:       "由 agent 作为 hook 调用，读 stdin",
		DetailEN: "gtmux runs this, you do not. It writes the pane's state and fires the notification.",
		DetailZH: "这是 gtmux 自己调的，不用你跑。它写入 pane 状态并触发通知。",
	},
}

// findCommand returns the table entry for a command name, or nil.
func findCommand(name string) *command {
	for i := range helpCommands {
		if helpCommands[i].Name == name {
			return &helpCommands[i]
		}
	}
	return nil
}
