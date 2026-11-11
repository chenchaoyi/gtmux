package app

import "github.com/chenchaoyi/gtmux/internal/hq"

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
	{ID: "look", EN: "LOOK", ZH: "状态与用量", ModeEN: "· inspects state", ModeZH: "· 查看状态", ReadsOnly: true},
	{ID: "go", EN: "GO THERE", ZH: "会话与终端", ModeEN: "· moves your terminal", ModeZH: "· 打开或切换会话"},
	{ID: "hq", EN: "HQ", ZH: "HQ", ModeEN: "· writes into panes", ModeZH: "· 任务协作与知识管理"},
	{ID: "remote", EN: "FROM YOUR PHONE", ZH: "远程访问", ModeEN: "· opens a port", ModeZH: "· 连接手机、浏览器和其他电脑"},
	{ID: "setup", EN: "SET UP AND UPDATE", ZH: "安装和更新", ModeEN: "· changes this host", ModeZH: "· 配置本机环境"},
	{ID: "media", EN: "SCREENSHOTS", ZH: "截图与标注", ModeEN: "· capture, annotate and send", ModeZH: "· 截图、编辑和发送"},
	{ID: "more", EN: "ALSO", ZH: "更多命令", ModeEN: "· gtmux <command> --help for any of these", ModeZH: "· 使用 gtmux <命令> --help 查看详情", Compact: true},
}

var helpCommands = []command{
	{Name: "screenshot", Args: "<operation> [--json]", Group: "media", Writes: true, OwnHelp: true, EN: "capture, annotate and deliver screenshots through the CLI", ZH: "截图、标注并发送到指定会话"},
	{Name: "opencode", Args: "[OpenCode arguments]", Group: "more", Writes: true, OwnHelp: true, EN: "launch OpenCode with terminal ownership (V1/V2)", ZH: "启动 OpenCode，兼容新旧版终端会话"},
	{Name: "codex", Args: "[Codex arguments]", Group: "more", Writes: true, OwnHelp: true,
		EN: "launch Codex with verified terminal identity", ZH: "启动 Codex 并核实终端会话归属"},
	{Name: "artifact", Args: "add FILE --session-id ID | list | remove ID", Group: "more", Writes: true, OwnHelp: true, EN: "publish a document for your paired devices", ZH: "在已配对设备上预览会话文件", Flags: []cmdFlag{
		{Name: "--session-id ID", EN: "exact transcript session identity; required for add", ZH: "所属对话的真实 ID；add 必填"},
		{Name: "--home-id ID", EN: "conversation home identity when present", ZH: "对话带有目录身份时指定"},
		{Name: "--json", EN: "print metadata instead of a Markdown link", ZH: "输出元数据，替代 Markdown 链接"},
	}},
	{
		Name: "agents", Args: "[--watch|--json [--pane %N]]", Group: "look",
		EN: "who is waiting, working, idle, and where",
		ZH: "查看各会话的位置和运行状态",
		Flags: []cmdFlag{
			{Name: "--watch", EN: "the live board: ↑↓ select, ⏎ jump, r refresh, q quit", ZH: "实时看板：↑↓ 选择，⏎ 跳转，r 刷新，q 退出"},
			{Name: "--json", EN: "the same rows as a structured array, for scripts and apps", ZH: "以 JSON 数组输出会话列表"},
			{Name: "--pane %N", EN: "fresh status for one pane; requires --json, excludes --watch/--popup and native histories", ZH: "实时检查单个窗格；须与 --json 同用，不可与 --watch/--popup 同用，不扫描外部历史会话"},
		},
		DetailEN: "Every pane with a coding agent in it, grouped by what it wants from you and sorted so the one that wants something is first. The pane id in the last column is what `gtmux focus` takes.",
		DetailZH: "列出运行编程助手的窗格，按状态分组，优先显示需要处理的会话。使用最后一列的窗格编号执行 `gtmux focus` 跳转。",
	},
	{
		Name: "digest", Args: "[--json]", Group: "look",
		EN:       "what each agent is doing and asking",
		ZH:       "查看会话目标、进展和待处理问题",
		Flags:    []cmdFlag{{Name: "--json", EN: "the same digest as data", ZH: "以 JSON 输出摘要"}},
		DetailEN: "Per agent: the goal it was given, the end of its last reply, and what it is asking when it waits. No model calls; this is read off what is already on disk and on screen. It is also what HQ reads.",
		DetailZH: "汇总各会话的任务目标、最新回复摘要和待处理问题。内容来自本机已有记录与终端画面，不调用模型；HQ 使用相同摘要。",
	},
	{Name: "transcript", Args: "<session_id> [--json]", Group: "look", Writes: true,
		EN: "read a verified external conversation", ZH: "读取外部会话的对话记录",
		DetailEN: "Read-only unless --review-through explicitly records a DSH review declaration. Supports verified Codex and DeepSeek Harness conversations; does not enable HQ follow or resume the session. --json returns turns, dropped count and a polling revision.",
		DetailZH: "默认只读；--review-through 会明确记录 DSH 审阅声明。支持身份已确认的 Codex 和 DeepSeek Harness 会话。不会开启 HQ 跟进或恢复会话。--json 返回对话、省略数量和更新标识。",
		Flags:    []cmdFlag{{Name: "--home-id <id>", EN: "require this conversation home identity", ZH: "指定会话数据目录身份"}, {Name: "--json", EN: "structured transcript and revision", ZH: "输出结构化对话与更新标识"}, {Name: "--etag <revision>", EN: "skip unchanged content; requires --json", ZH: "跳过未变更内容；须与 --json 一起使用"}, {Name: "--messages", EN: "DSH public message page as JSON; no automatic review acknowledgement", ZH: "以 JSON 逐条读取 DSH 公开消息；读取不等于审阅"}, {Name: "--after-seq <n>", EN: "messages after source sequence; requires --messages; default -1", ZH: "读取此源序号之后的消息；须配合 --messages；默认 -1"}, {Name: "--limit <n>", EN: "message count, 1–200; requires --messages; default 100", ZH: "消息数量，1–200；须配合 --messages；默认 100"}, {Name: "--review-through <n>", EN: "explicitly declare review through a returned message sequence; requires --messages", ZH: "明确声明已审阅至本页返回的消息序号；须配合 --messages"}}},

	{
		Name: "follow", Args: "<session_id> [options]", Group: "setup", Writes: true,
		EN: "set follow for one external conversation", ZH: "设置外部会话的 HQ 跟进权限",
		Flags: []cmdFlag{
			{Name: "--hq <mode>", EN: "read and report this conversation; off also clears notification and learning permissions", ZH: "读取对话并跟进；关闭后同时关闭通知与知识采集", Values: []string{"on", "off"}},
			{Name: "--notify <mode>", EN: "receive this conversation's notifications while followed", ZH: "跟进期间接收此会话通知", Values: []string{"on", "off"}},
			{Name: "--knowledge <mode>", EN: "mine future reusable experience while followed", ZH: "跟进期间采集后续可复用经验", Values: []string{"on", "off"}},
			{Name: "--home-id <id>", EN: "require this conversation home identity", ZH: "指定会话数据目录身份"},
			{Name: "--revision <n>", EN: "refuse a stale settings save", ZH: "校验设置版本，避免覆盖其他设备的修改"},
			{Name: "--json", EN: "return the saved settings", ZH: "输出设置数据"},
		},
		DetailEN: "Without write flags, reads the policy. Requires verified identity in a configured Codex or DSH home. No remote input or adoption. Settings stay on this host and are not part of HQ migration.",
		DetailZH: "不带修改选项时读取设置。适用于已确认身份的外部 Codex 或 DSH 会话，不增加远程输入或转入 tmux 能力。设置只保存在本机，不随 HQ 档案迁移。",
	},

	{
		Name: "overview", Args: "[--popup]", Group: "look",
		EN:    "sessions, windows and panes, counted",
		ZH:    "查看会话、窗口和窗格数量",
		Flags: []cmdFlag{{Name: "--popup", EN: "the form prefix+g opens inside tmux", ZH: "在 tmux 弹出窗口中显示总览（prefix+g）"}},
	},
	{
		Name: "usage", Args: "[--json|--activity]", Group: "look",
		EN: "tokens spent, and how much of your plan is left",
		ZH: "查看 token 用量和剩余额度",
		Flags: []cmdFlag{
			{Name: "--json", EN: "conversations, rollups, plan windows and the last seven days", ZH: "对话、汇总、额度窗口，以及最近七天"},
			{Name: "--activity", EN: "the year as a calendar heatmap, as wide as the terminal", ZH: "按年显示用量热力图，适配终端宽度"},
		},
		DetailEN: "The plan windows lead: they are the one number local counting cannot produce. The conversation column counts each conversation since it started, which is why a single row can exceed the week's total.",
		DetailZH: "优先显示服务端返回的订阅额度。单个会话的用量按整个会话累计，可能高于所选周内的总用量。",
	},
	{
		Name: "limits", Args: "[--json]", Group: "look",
		EN: "the subscription windows on their own",
		ZH: "查看订阅额度及重置时间",
		Flags: []cmdFlag{
			{Name: "--json", EN: "the windows as data", ZH: "以 JSON 输出额度信息"},
			{Name: "--refresh", EN: "refresh, or wait for an active refresh and use its result", ZH: "刷新额度；已有刷新任务时等待其结果"},
		},
		DetailEN: "Real server numbers, from what each agent itself reports: Claude by running its own `/usage` headlessly, Codex by reading the rate-limit response already in its rollout. Cached, so asking twice costs nothing.",
		DetailZH: "额度以服务端数据为准：Claude 通过后台 `/usage` 查询，Codex 从会话记录中的限流响应读取。结果会缓存，重复查询优先使用缓存。",
	},

	{
		Name: "logs", Args: "[--since 1h|--follow|--access]", Group: "look",
		EN: "what gtmux saw and did on this host",
		ZH: "查看运行日志和操作记录",
		Flags: []cmdFlag{
			{Name: "--access", EN: "external client access history; --json returns a paginated object", ZH: "外部客户端访问历史；--json 返回分页对象"},
			{Name: "--origin <value>", EN: "source range; default external; other ranges inspect diagnostic records; --access only", ZH: "来源范围，默认 external；其他范围用于诊断；仅用于 --access", Values: []string{"external", "local", "unknown", "all"}},
			{Name: "--device-id <id>", EN: "exact full device ID; --access only", ZH: "按完整设备 ID 精确筛选；仅用于 --access"},
			{Name: "--operation <key>", EN: "key from operations in --access --json; --access only", ZH: "操作标识见 --access --json 的 operations；仅用于 --access"},
			{Name: "--outcome <value>", EN: "access outcome; --access only", ZH: "访问结果；仅用于 --access", Values: []string{"ok", "refused", "failed"}},
			{Name: "--group-by <device>", EN: "bounded device summaries of recorded activity; --access only", ZH: "按设备汇总已记录活动，支持分页；仅用于 --access", Values: []string{"device"}},
			{Name: "--before <cursor>", EN: "an earlier access-history page", ZH: "更早的访问历史页"},
			{Name: "--limit <1..200>", EN: "access records or summary identities per page (default 100)", ZH: "每页访问记录或汇总来源数（默认 100）"},
			{Name: "--since <when>", EN: "how far back: 30m, 2h, 3d, or a date (default 1h; access history uses retention window)", ZH: "时间范围：30m、2h、3d 或日期（默认 1h；访问历史默认整个保留期）"},
			{Name: "--until <when>", EN: "stop at this point", ZH: "结束时间"},
			{Name: "--component <c>", EN: "one part of gtmux", ZH: "按组件筛选",
				Values: []string{"serve", "tunnel", "hook", "cli", "hq", "restore", "hygiene", "menubar", "log"}},
			{Name: "--level <l>", EN: "this level and above", ZH: "最低日志级别",
				Values: []string{"debug", "info", "warn", "error"}},
			{Name: "--acts", EN: "only what gtmux did, not what it saw", ZH: "仅显示操作记录，排除诊断日志"},
			{Name: "--actor <who>", EN: "who started it: user, hq, agent, menubar, phone, browser, guest, system, or one of them exactly (agent:%7)", ZH: "谁发起的：user、hq、agent、menubar、phone、browser、guest、system，或精确到某一个（agent:%7）"},
			{Name: "--event <glob>", EN: "event names, with * as a wildcard: 'act.*', act.pair", ZH: "事件名，* 为通配：'act.*'、act.pair"},
			{Name: "--follow, -f", EN: "keep printing new entries, across midnight", ZH: "持续打印新记录，跨过零点也不断"},
			{Name: "--stats", EN: "how much is kept and how much of the window went wrong, instead of the entries", ZH: "仅显示日志数量和指定时间内的问题统计"},
			{Name: "--json", EN: "raw entries, one per line; --access returns a paginated object", ZH: "原样输出，一行一条；--access 返回分页对象"},
		},
		DetailEN: "One store for every gtmux process, like the system log: serve, the tunnel client, the hook, every command and the menu bar write to ~/.local/share/gtmux/logs/, one file per day. It holds diagnostics and every action that changes something, with who started it, what it acted on and how it ended. Entries are English, never hold message text, and have credentials replaced. The store keeps 30 days or 100 MB, whichever comes first (logs.retainDays and logs.maxMB in config.json).",
		DetailZH: "所有 gtmux 进程共用一个日志库，和系统日志一样：serve、隧道客户端、hook、每条命令、菜单栏都写进 ~/.local/share/gtmux/logs/，每天一个文件。里面有诊断记录，也有每一次改动东西的操作，记下谁发起、作用在什么上、结果如何。记录用英文，从不包含消息正文，凭证会被替换掉。日志库保留 30 天或 100MB，先到哪个算哪个（config.json 里的 logs.retainDays 和 logs.maxMB）。",
	},
	{
		Name: "focus", Args: "<name|%pane>", Group: "go", Writes: true,
		EN:    "jump this terminal to that session or pane",
		ZH:    "跳转到指定会话或窗格",
		Flags: []cmdFlag{{Name: "--last, -l", EN: "the agent pane that finished most recently", ZH: "跳转到最近完成任务的助手窗格"}},
	},
	{
		Name: "restore", Args: "[--pick|--plan]", Group: "go", Writes: true,
		EN: "reopen every session in its own terminal tab",
		ZH: "恢复已保存的会话及终端标签页",
		Flags: []cmdFlag{
			{Name: "<name>", EN: "this tab joins that session", ZH: "在当前标签页中连接指定会话"},
			{Name: "--pick, -p", EN: "list them and choose: a number, Enter for all, q to cancel", ZH: "选择要恢复的会话：编号选择，回车全选，q 取消"},
			{Name: "--one", EN: "this tab joins the next session nobody is attached to", ZH: "连接下一个尚未连接终端的会话"},
			{Name: "--dry-run", EN: "print what it would do and do nothing", ZH: "预览恢复操作，不执行"},
			{Name: "--plan [--pending] [--json]", EN: "preview saved sessions; --pending shows missing or detached sessions; starts no tmux", ZH: "预览存档会话；--pending 只列缺失或未连接的会话；不启动 tmux"},
			{Name: "--resume-agents=<mode>", EN: "what to do with the agent conversations under each pane", ZH: "设置恢复窗格后如何启动助手",
				Values: []string{"auto", "type", "off"}},
		},
		DetailEN: "auto runs each captured conversation again (`claude --resume …`), type puts the command in the pane and leaves it unsent, off reopens the panes and starts nothing. The default follows autoResumeAgentSessions, which is auto.\n\nAfter a reboot, restore starts tmux and directly runs tmux-resurrect to bring back the last save and check its layout. Screen text returns only if capture was configured. --resume-agents recovers agent conversations that were running when saved and can still be found.",
		DetailZH: "auto 重新启动已保存的助手会话（如 `claude --resume …`）；type 仅填入恢复命令，不执行；off 只恢复窗格。默认由 autoResumeAgentSessions 配置决定，初始值为 auto。\n\n重启后，restore 启动 tmux 并调用 tmux-resurrect 恢复最近的存档、核对布局。屏幕文本需要预先启用保存；--resume-agents 控制是否恢复存档中仍可找到的助手会话。",
	},
	{
		Name: "new", Args: "[name]", Group: "go", Writes: true,
		EN: "start a session and open a terminal tab for it",
		ZH: "新建会话并打开终端标签页",
	},
	{
		Name: "adopt", Args: "<session_id>…", Group: "go", Writes: true,
		EN:       "take an agent running outside tmux into tmux",
		ZH:       "将外部助手会话转入 tmux",
		DetailEN: "A conversation gtmux only senses (no pane, read-only on the radar) is resumed inside a fresh tmux session, after which it is a pane like any other.",
		DetailZH: "为雷达中的只读外部会话新建 tmux 会话，并恢复对应对话。完成后可使用常规窗格操作。",
	},
	{
		Name: "panes", Args: "[--json|--watched]", Group: "go",
		EN: "every pane, agents and plain shells alike",
		ZH: "列出所有助手窗格和普通终端",
		Flags: []cmdFlag{
			{Name: "--json", EN: "the session/window/pane tree as data", ZH: "以 JSON 输出会话、窗口和窗格结构"},
			{Name: "watch %N", EN: "put a plain pane on the radar as its own row", ZH: "将普通窗格固定到雷达"},
			{Name: "unwatch %N", EN: "take it off again", ZH: "取消固定窗格"},
			{Name: "--watched", EN: "just the ones you pinned", ZH: "仅显示已固定的窗格"},
		},
		DetailEN: "Each pane is tagged tier=agent|plain. focus, send and attach work on any of them; this is the full set the pane browser shows.",
		DetailZH: "每个窗格以 tier=agent|plain 标明类型。助手窗格和普通终端都支持 focus、send、attach，窗格浏览器显示相同列表。",
	},

	hqHelpCommand(),
	{
		Name: "knowledge", Args: "<verb>", Group: "hq", Writes: true,
		EN: "what HQ has learned on this machine",
		ZH: "管理本机知识库",
		Flags: []cmdFlag{
			{Name: "list [--topic t]", EN: "every live entry", ZH: "列出知识库条目"},
			{Name: "show <id>", EN: "one entry in full", ZH: "读取条目全文"},
			{Name: "lint [--json]", EN: "audit the base: what to fix, never fixed for you", ZH: "检查知识库质量，仅报告问题"},
			{Name: "style [--json]", EN: "how an entry should read, with an example for each rule", ZH: "查看条目编写规则及修改示例"},
			{Name: "search \"<text>\"", EN: "ask the base a question in words, in either language", ZH: "用一句话问知识库，中英都行"},
			{Name: "receipts [--capture key] [--json]", EN: "committed acceptances and dismissals, with sources", ZH: "已提交的采纳、驳回及其来源"},
			{Name: "show <id> --json", EN: "entry detail with retained sources", ZH: "条目详情与保留的来源"},
			{Name: "carriers", EN: "which agents carry the machine block, and is it current", ZH: "查看各助手的本机引导安装状态与版本"},
			{Name: "add --topic <t> --title \"…\"", EN: "record one lesson (HQ home only)", ZH: "新增经验条目（仅限 HQ 目录）",
				Required: true},
			{Name: "--kind <k>", EN: "what the entry IS", ZH: "条目分类",
				Values: []string{"facts", "howto", "pitfalls", "judgment", "decisions"}},
			{Name: "--why \"…\"", EN: "the reason, on promote/retire/withdraw", ZH: "启用、退役或撤回条目的原因", MaxBytes: 300},
			{Name: "--sensitive", EN: "exclude from machine/repo distribution; whole-HQ backups still include it", ZH: "不向本机全局或仓库分发；整个 HQ 的备份仍包含它",
				Requires: []string{"--confirmed"}},
		},
		DetailEN: "The whole list of verbs is `gtmux knowledge` with no arguments. Writing verbs are accepted only from the HQ home; list, show and receipts work anywhere. add and supersede accept --capture to retain selected sources.",
		DetailZH: "运行 `gtmux knowledge` 查看全部子命令。写入操作仅允许在 HQ 目录执行；list、show 和 receipts 可从任意目录读取。add 和 supersede 支持用 --capture 保留来源。",
	},
	{
		Name: "advice", Args: "[\"<what you advised>\"]", Group: "hq", Writes: true,
		EN: "what HQ proposed, and what became of it",
		ZH: "查看 HQ 建议及处理结果",
		Flags: []cmdFlag{
			{Name: "--tally [--since 30d]", EN: "how much was advised, and how much was taken", ZH: "查看建议数量和采纳统计"},
			{Name: "--open", EN: "only the advice still waiting on an outcome", ZH: "仅显示待处理建议"},
			{Name: "<id> --taken", EN: "the commander went with it", ZH: "记录用户采纳"},
			{Name: "<id> --declined", EN: "they went another way", ZH: "记录用户拒绝",
				Requires: []string{"--words"}},
			{Name: "--words \"…\"", EN: "their own answer, kept verbatim", ZH: "保留用户原话", MaxBytes: 600},
			{Name: "<id> --moot", EN: "events settled it first", ZH: "标记问题已自行解决"},
			{Name: "--why \"…\"", EN: "the reasoning behind the advice", ZH: "这条建议的依据", MaxBytes: 600},
		},
		DetailEN: "The only ledger that can say whether HQ's judgement is worth anything, and whether it is still speaking up. Recording runs from the HQ home; reading works anywhere. You file nothing.",
		DetailZH: "记录 HQ 建议及其处理结果，用于复盘建议质量。写入操作须在 HQ 目录执行，查询不受目录限制。",
	},
	{
		Name: "capture", Args: "\"<lesson> @<topic>\"", Group: "hq", Writes: true,
		EN:       "hand HQ one lesson to file",
		ZH:       "提交经验供 HQ 整理",
		Flags:    []cmdFlag{{Name: "--list", EN: "the queue waiting for HQ to judge", ZH: "查看待整理的经验"}},
		DetailEN: "The cheapest way in: any agent drops a line, HQ decides what becomes an entry.",
		DetailZH: "提交简短经验，由 HQ 判断是否整理为知识库条目。",
	},

	{
		Name: "serve", Args: "[--port N]", Group: "remote", Writes: true,
		EN: "serve the radar to your phone on this network",
		ZH: "启动本地服务，供手机和浏览器访问",
		Flags: []cmdFlag{
			{Name: "--port N", EN: "which port to listen on", ZH: "监听端口"},
			{Name: "--bind ADDR", EN: "which address to listen on", ZH: "监听地址"},
			{Name: "--memory-diagnostics DIR", EN: "opt-in local memory socket in an existing owned 0700 directory; foreground only", ZH: "在本人拥有的现有 0700 目录中启用本机内存 socket；仅前台运行"},
			{Name: "--token TOKEN", EN: "the bearer token; one is generated and kept on first run", ZH: "访问令牌；首次启动时自动生成并保存"},
			{Name: "--relay-url URL", EN: "point push at a relay so alerts reach the lock screen", ZH: "配置推送中继，用于锁屏通知",
				Requires: []string{"--relay-token"}},
		},
		DetailEN: "Serves the radar and browser view. Paired devices and guest links have separate access; browser input is limited to authorized panes.",
		DetailZH: "提供雷达和网页版。已配对设备与访客链接分别授权；网页只能向获准的 pane 输入。",
	},
	{
		Name: "tunnel", Args: "[--backend|--quick]", Group: "remote", Writes: true,
		EN: "a public address over an outbound tunnel",
		ZH: "通过隧道提供远程访问地址",
		Flags: []cmdFlag{
			{Name: "--backend <b>", EN: "which way out", ZH: "选择隧道服务", Values: []string{"cloudflare", "self"}},
			{Name: "--quick", EN: "an account-less ephemeral URL", ZH: "创建免账号的临时地址"},
			{Name: "--redeem <code>", EN: "unlock Direct", ZH: "解锁 Direct"},
			{Name: "--region <r>", EN: "with --redeem: where you would like your Direct server", ZH: "与 --redeem 配合，指定 Direct 服务器地区", Requires: []string{"--redeem"}},
			{Name: "--servers", EN: "the Direct servers you may use, with the round trip from this host", ZH: "列出可用 Direct 服务器及本机实测延迟"},
			{Name: "--json", EN: "with --servers: the same list as data", ZH: "与 --servers 配合，以 JSON 输出服务器列表", Requires: []string{"--servers"}},
			{Name: "--server <id>", EN: "move this host to that Direct server", ZH: "切换本机使用的 Direct 服务器"},
			{Name: "--service", EN: "keep it on across reboots", ZH: "安装为后台服务，开机自动启动"},
			{Name: "--force", EN: "with --service and cloudflare: repair the existing ingress and DNS, keeping the address", ZH: "配合 --service 和 cloudflare：修复原隧道的入口和 DNS，保留地址", Requires: []string{"--service"}},
			{Name: "--recover", EN: "with --service and cloudflare: restore the connection, replacing only a confirmed missing tunnel", ZH: "配合 --service 和 cloudflare：恢复连接，仅在确认原隧道已不存在时重建", Requires: []string{"--service"}},
		},
		DetailEN: "Prints a public URL, a token, and a QR the phone can scan. Standard is a stable hosted address you pair once; self is Direct over 443, self-hosted or unlocked.",
		DetailZH: "输出远程访问地址、访问令牌和手机二维码。Standard 使用固定托管地址；self 使用 443 端口的 Direct，可自托管或通过兑换码开通。",
	},
	{
		Name: "pair", Args: "[list|revoke <id>]", Group: "remote", Writes: true,
		EN:       "pair your own devices, list or revoke them",
		ZH:       "配对设备，查看或撤销配对",
		DetailEN: "One one-time code, three ways to use it: scan it on the phone, open it in a browser, or paste the printed `gtmux attach` line into another computer's terminal. Someone else's device goes through `share` instead, which is scoped.",
		DetailZH: "生成一次性配对码：可用手机扫码、浏览器打开，或在另一台电脑执行输出中的 `gtmux attach` 命令。仅向他人分享指定窗格时使用 `share`。",
	},
	{
		Name: "attach", Args: "<target> [%N]", Group: "remote", Writes: true,
		EN: "open a remote pane in this terminal",
		ZH: "在当前终端连接远程窗格",
		Flags: []cmdFlag{
			{Name: "--read-only", EN: "watch without typing", ZH: "只读连接，不发送输入"},
			{Name: "--token TOKEN", EN: "when the target is a bare host", ZH: "目标未指定端口时使用的端口"},
			{Name: "--code CODE", EN: "a share link's short code; refused like the link itself (open the link in a browser)", ZH: "分享链接的短码；和链接本身一样会被拒绝（请在浏览器里打开链接）"},
		},
		DetailEN: "Your terminal becomes the remote pane, raw, over a WebSocket. A pair link (…/#c=) enrolls this terminal as your own device and is kept for that host, so later just `gtmux attach <host>`. A share link (…#code=, or --code when it was read out to you) is refused: a terminal would reach the whole tmux session, not only the shared panes; open the link in a browser instead. Ctrl-] detaches.",
		DetailZH: "通过 WebSocket 在当前终端操作远程窗格。配对链接（…/#c=）会将此终端登记为本人设备，并保存主机信息，之后可用 `gtmux attach <host>` 连接。终端连接可访问整个 tmux 会话，因此不接受仅授权单个窗格的分享链接（…#code= 或 --code）；分享链接请用浏览器打开。按 Ctrl-] 退出。",
	},
	{
		Name: "devices", Args: "[revoke <id>]", Group: "remote", Writes: true,
		EN: "the devices that can reach this host",
		ZH: "查看已登记的连接设备",
		Flags: []cmdFlag{
			{Name: "--push", EN: "inspect the push tokens", ZH: "显示推送令牌"},
			{Name: "--forget-push <what>", EN: "clear them", ZH: "清除推送令牌", Values: []string{"<id>", "orphans", "all"}},
		},
	},
	{
		Name: "awake", Args: "[on|off]", Group: "remote", Writes: true,
		EN:       "keep this Mac working with the lid closed",
		ZH:       "管理合盖后保持运行的设置",
		Flags:    []cmdFlag{{Name: "--json", EN: "the current state as data", ZH: "以 JSON 输出当前状态"}},
		DetailEN: "So serve, the tunnel and the phone keep answering after you shut the lid. `on` asks for your admin password once and checks it took effect; `off` needs no password while the guard `on` installed is there.",
		DetailZH: "保持合盖后的本地服务和隧道运行。`on` 需要管理员密码并会验证设置结果；已安装对应守护程序时，`off` 无需再次输入密码。",
	},

	{
		Name: "doctor", Args: "[--json | --fix | --bundle]", Group: "setup", Writes: true,
		EN: "check this host, then set up what is missing",
		ZH: "检查本机环境和功能配置",
		Flags: []cmdFlag{
			{Name: "--json", EN: "versioned read-only report; never prompts or repairs", ZH: "输出带版本的只读报告，不询问、不修复"},
			{Name: "--no-prompt", EN: "read-only checks even in a terminal", ZH: "在终端中也仅检查，不询问是否修复"},
			{Name: "--terminal", EN: "open interactive repairs in the configured terminal; no --yes", ZH: "在配置的终端中逐项确认修复，不可与 --yes 一起使用", Requires: []string{"--fix"}},
			{Name: "--progress", EN: "show each check stage on stderr (also shown automatically in a terminal)", ZH: "在标准错误输出显示检查进度（终端中默认显示）"},
			{Name: "--fix", EN: "set the rest up, explaining and asking before each change", ZH: "逐项确认并修复检查发现的问题"},
			{Name: "--yes", EN: "apply every step without asking", ZH: "确认执行全部可用修复", Requires: []string{"--fix"}},
			{Name: "--bundle [path]", EN: "pack logs, status, launchd output, this report and versions into one file for a bug report; tokens replaced, nothing sent", ZH: "把日志、状态、launchd 输出、这份报告和版本号打成一个文件，用于报告问题；token 已替换，不会发送"},
			{Name: "--with-events", EN: "also pack the event journal, which holds prompt heads", ZH: "连事件流一起打包，里面有提示词开头", Requires: []string{"--bundle [path]"}},
		},
		DetailEN: "Grouped by what it affects: tmux, restore, the terminal, agents and notifications. This is the way in: run it first, run it again whenever something stops working.",
		DetailZH: "按 tmux、会话恢复、终端、助手和通知分类检查。首次配置或排查异常时可先运行此命令。",
	},
	{
		Name: "install", Args: "[hooks|app|cloudflared]", Group: "setup", Writes: true,
		EN: "install agent hooks or the menu-bar app",
		ZH: "安装助手 hook 或菜单栏应用",
		Flags: []cmdFlag{
			{Name: "cloudflared [--yes]", EN: "install or update the Standard tunnel client through its original channel", ZH: "通过原渠道安装或更新 Standard 隧道客户端"},
			{Name: "refresh-owned --agent dsh", EN: " refresh an existing gtmux bridge only; never installs a client or enables a profile", ZH: "仅更新已有 gtmux 连接组件，不安装客户端或启用 profile"},
			{Name: "hooks [--yes]", EN: "register the agent hooks", ZH: "注册 agent hook"},
			{Name: "--agent <a>", EN: "the agent to configure", ZH: "要配置的助手",
				Values: append([]string{"auto"}, installableHookAgentKeys()...)},
			{Name: "app", EN: "the menu-bar app, which delivers desktop notifications", ZH: "安装菜单栏应用，用于桌面通知"},
		},
		DetailEN: "No target and it asks. Hooks default to Claude; --agent auto selects supported agent CLIs found in PATH or known install locations; retained configuration alone does not select an agent. --agent dsh installs the bundled native Cordis module and prints the next Web launch command. `doctor --fix` repairs the same files.",
		DetailZH: "未指定组件时会询问安装内容。hooks 默认配置 Claude；--agent auto 根据 PATH 和常见安装位置识别助手，不会仅凭旧配置目录选择。--agent dsh 安装随附 Cordis 模块，并输出下次 Web 启动命令。`doctor --fix` 可修复相同配置。",
	},
	{
		Name: "uninstall", Args: "[hooks|app]", Group: "setup", Writes: true,
		EN:       "take back out what gtmux installed",
		ZH:       "卸载 gtmux 安装的组件",
		DetailEN: "Removing hooks stops their direct state reports; tmux agents can still be detected with fallback state. Removing the app stops desktop notifications. No target and it asks.",
		DetailZH: "卸载 hook 后不再接收直接状态报告，仍可识别 tmux 中的助手并推断状态；卸载应用后停止桌面通知。未指定组件时会询问。",
	},
	{
		Name: "update", Args: "[--check]", Group: "setup", Writes: true,
		EN: "update gtmux, the CLI and the menu-bar app",
		ZH: "更新命令行工具和菜单栏应用",
		Flags: []cmdFlag{
			{Name: "--check", EN: "report what is available and change nothing", ZH: "仅检查新版本，不更新"},
			{Name: "--cli-only", EN: "leave the app alone", ZH: "仅更新命令行工具"},
		},
	},
	{
		Name: "whatsnew", Args: "[--since v] [--json]", Group: "setup",
		EN:       "read what changed, per release",
		ZH:       "查看各版本更新内容",
		DetailEN: "`update` prints a summary. Read full notes within the latest 30 releases; --all starts at 0.0.0, --until caps the included version, and --json returns bilingual authored notes (schema 1).",
		DetailZH: "读取最近 30 个发行版中的完整说明；--all 从 0.0.0 开始，--until 限定最高版本，--json 返回双语更新说明（schema 1）。",
	},
	{
		Name: "app", Group: "setup", Writes: true,
		EN:       "open the menu-bar app",
		ZH:       "打开菜单栏应用",
		DetailEN: "The status dot appears in the top-right menu bar. Also spelled `menubar`. Install it with the curl installer or macapp/build.sh.",
		DetailZH: "启动后可在 macOS 菜单栏查看状态。命令别名为 `menubar`；安装可使用安装脚本或 macapp/build.sh。",
	},

	{Name: "spawn", Group: "more", Writes: true, OwnHelp: true,
		EN: "launch an agent and hand it a task", ZH: "启动助手并分配任务"},
	{Name: "relay", Group: "more", Writes: true, OwnHelp: true,
		EN: "coordinate with HQ through requests and an inbox", ZH: "向 HQ 汇报、提问并读取收件箱",
		DetailEN: "Use report/ask to save a request, inbox to list your unread replies, show to fetch one, or wait --timeout 30 after a blocking ask. Codex full replies are never queued as human turns; only a fresh, coalesced active-turn hint is attempted. Hint accepted is not read. Run relay --help for verbs and flags.",
		DetailZH: "用 report/ask 记录请求，inbox 列出未读回复，show 读取指定回复；阻塞提问后可 wait --timeout 30。Codex 完整回执不作为用户轮次排队，只尝试新鲜且按运行轮次合并的简短提醒。提醒被接受不等于已读。具体子命令和参数见 relay --help。"},
	{Name: "tasks", Group: "more", OwnHelp: true,
		EN: "the dispatch and needs-you ledger", ZH: "查看任务记录和待处理事项"},
	{Name: "reap", Group: "more", Writes: true, OwnHelp: true,
		EN: "reclaim a dispatch that finished", ZH: "回收已结束任务的运行资源"},
	{Name: "send", Group: "more", Writes: true, OwnHelp: true,
		EN: "type a message into a pane", ZH: "向指定窗格发送消息"},
	{Name: "events", Group: "more", OwnHelp: true,
		EN: "the event stream HQ reads", ZH: "读取会话和任务事件"},
	{Name: "share", Group: "more", Writes: true, OwnHelp: true,
		EN: "share one pane with someone else", ZH: "分享指定窗格"},
	{Name: "status", Group: "more", OwnHelp: true,
		EN: "one line, for scripts and prompts", ZH: "一行输出，给脚本和提示符用"},
	{Name: "config", Group: "more", Writes: true, OwnHelp: true,
		EN: "read and change gtmux's settings", ZH: "查看或修改 gtmux 设置"},
	{Name: "quiet", Group: "more", Writes: true, OwnHelp: true,
		EN: "how loud HQ is allowed to be", ZH: "设置 HQ 提醒频率"},
	{Name: "resource", Group: "more", OwnHelp: true,
		EN: "disk, memory and CPU, per agent", ZH: "查看各助手的资源用量"},
	{
		Name: "hook", Group: "setup", Writes: true, Internal: true,
		EN:       "run by an agent as a hook, reads stdin",
		ZH:       "由 agent 作为 hook 调用，读 stdin",
		DetailEN: "gtmux runs this, you do not. It writes the pane's state and fires the notification.",
		DetailZH: "供 gtmux 内部调用，用于更新窗格状态和发送通知，无需手动执行。",
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

// HQ owns its catalog so its leaf command and the root help cannot drift.
func hqHelpCommand() command {
	c := command{Name: "hq", Args: "[options]", Group: "hq", Writes: true,
		EN: "open HQ to follow your agents and report progress", ZH: "打开 HQ，管理任务和待处理事项"}
	for _, f := range hq.HelpFlags() {
		c.Flags = append(c.Flags, cmdFlag{Name: f.Name, EN: f.EN, ZH: f.ZH, Values: f.Values, Requires: f.Requires})
	}
	return c
}
