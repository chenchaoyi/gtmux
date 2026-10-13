package app

import (
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"strings"
)

type doctorPurpose struct {
	keyEN, keyZH         string
	titleEN, titleZH     string
	purposeEN, purposeZH string
}

// Report copy explains the measured prerequisite. It does not change checks,
// statuses or repair decisions; the terminal retains its technical labels.
var doctorPurposes = []doctorPurpose{
	{"cli", "命令行", "Command-line tool", "命令行工具", "Reports the version and location of the gtmux command running these checks.", "确认执行本次检查的 gtmux 命令行版本及位置。"},
	{"app", "app", "Menu-bar version", "菜单栏版本", "Checks whether the installed menu-bar app and command-line tool have the same version.", "检查菜单栏应用与命令行工具的版本是否一致。"},
	{"config", "配置", "Configuration file", "配置文件", "Checks the JSON syntax of the gtmux configuration file.", "检查 gtmux 配置文件的 JSON 格式是否有效。"},
	{"tmux", "tmux", "tmux", "tmux", "Checks the installed tmux version required to manage terminal sessions.", "检查 tmux 是否已安装，以及版本是否支持会话管理。"},
	{"locale", "字符集", "Character encoding", "字符编码", "Checks UTF-8 support in the tmux environment for names and symbols.", "检查 tmux 环境是否使用 UTF-8，以正确显示中文名称和符号。"},
	{"set-titles", "set-titles", "Terminal tab titles", "终端标签页标题", "Checks whether tmux publishes session and window names so gtmux can locate terminal tabs.", "检查 tmux 是否将会话和窗口名称传给终端，便于定位标签页。"},
	{"window-name source", "窗口名来源", "Window naming", "窗口命名", "Checks the format used for automatic tmux window names; custom formats are preserved.", "检查 tmux 自动命名窗口的格式；保留自定义格式。"},
	{"pane ids in tab titles", "标签页标题带 pane id", "Pane identifiers", "窗格标识", "Checks whether window names include pane IDs, so panes can be distinguished when switching tabs.", "检查窗口名称是否包含窗格编号，便于切换时区分不同窗格。"},
	{"pane titles", "pane 标题", "Pane names", "窗格名称", "Checks whether ordinary panes have a meaningful process title instead of only a shell name.", "检查普通窗格是否显示当前进程名称，而不只是 shell 名称。"},
	{"clickable links", "可点击的链接", "Terminal links", "终端链接", "Checks whether tmux allows terminal hyperlinks to be opened.", "检查 tmux 是否允许打开终端中显示的链接。"},
	{"history-limit", "history-limit", "Terminal history", "终端历史长度", "Checks the tmux scrollback limit used to retain terminal output.", "检查 tmux 可保留的终端输出行数。"},
	{"TPM", "TPM", "Plugin manager", "插件管理器", "Checks whether TPM is installed to manage the session recovery plugins.", "检查 TPM 是否已安装，用于管理会话恢复插件。"},
	{"tmux-resurrect", "tmux-resurrect", "Session recovery", "会话恢复插件", "Checks the plugin that saves and restores tmux window and pane layouts.", "检查用于保存和恢复 tmux 窗口及窗格布局的插件。"},
	{"tmux-continuum", "tmux-continuum", "Automatic save plugin", "自动保存插件", "Checks the plugin that periodically saves tmux layouts and supports automatic recovery.", "检查用于定期保存 tmux 布局和自动恢复的插件。"},
	{"capture-pane", "capture-pane", "Save terminal contents", "保存终端内容", "Checks whether session backups include pane contents as well as layouts.", "检查会话备份是否同时保存窗格内容和布局。"},
	{"auto-restore", "auto-restore", "Restore on startup", "启动时恢复", "Checks whether continuum restores saved sessions when tmux starts.", "检查 tmux 启动时是否自动恢复已保存的会话。"},
	{"resurrect autosave", "resurrect 自动保存", "Automatic session saves", "会话自动保存", "Checks the save trigger and recent backup activity, rather than only plugin installation.", "检查自动保存触发器和最近的备份情况，而不只检查插件是否安装。"},
	{"host", "宿主终端", "Current terminal", "当前终端", "Checks the terminal hosting tmux and the actions its gtmux driver supports.", "识别承载 tmux 的终端，并检查可用的跳转和会话操作。"},
	{"other terminals", "其它终端", "Other installed terminals", "其他已安装终端", "Lists other installed terminals and whether gtmux can control or only detect them.", "列出其他已安装终端，以及 gtmux 能控制还是只能识别它们。"},
	{"Claude Code renderer", "Claude Code 渲染器", "Claude Code display mode", "Claude Code 显示模式", "Checks the configured renderer and existing fullscreen panes that limit terminal history.", "检查 Claude Code 的显示配置，以及可能限制终端历史的全屏窗格。"},
	{"chat binding", "会话绑定", "Conversation binding", "对话绑定", "Checks whether pane bindings still refer to the current agent conversation, for reading Chat history.", "检查窗格是否绑定到当前 agent 对话，以正确读取对话历史。"},
	{"hook traffic", "hook 通路", "Agent state updates", "Agent 状态更新", "Checks for active agent turns whose installed hooks have stopped sending state updates.", "检查正在工作的 agent 是否长时间未通过 hook 上报状态。"},
	{"installed", "安装", "App installation", "应用安装", "Checks whether the menu-bar app needed for desktop notifications is installed.", "检查负责桌面通知的菜单栏应用是否已安装。"},
	{"path", "路径", "App location", "应用位置", "Reports the installed menu-bar bundle that gtmux uses.", "确认 gtmux 使用的菜单栏应用安装位置。"},
	{"up to date", "是否最新", "App compatibility", "应用版本兼容", "Checks whether the installed menu-bar app needs updating to match the command-line tool.", "检查菜单栏应用是否需要更新，以匹配命令行工具版本。"},
	{"cloudflared", "cloudflared", "Standard tunnel client", "标准隧道客户端", "Checks the optional cloudflared client used by the standard tunnel.", "检查标准隧道使用的 cloudflared 客户端；使用标准隧道时需要安装。"},
	{"sleep setting", "睡眠设置", "Mac sleep settings", "Mac 睡眠设置", "Checks the actual sleep setting and whether gtmux owns the change.", "检查 Mac 实际的睡眠设置，以及该设置是否由 gtmux 管理。"},
	{"serve", "服务", "Local server", "本机服务", "Checks whether the gtmux server needed by the phone and browser is responding.", "检查供手机和浏览器连接的 gtmux 本机服务是否响应。"},
	{"tunnel", "隧道", "Remote connection", "远程连接", "Checks the running tunnel used to reach this Mac outside the local network.", "检查用于从外部网络连接这台 Mac 的隧道运行状态。"},
	{"lock-screen card", "锁屏卡片", "Live Activity updates", "实时活动更新", "Checks whether the running server still has the tokens needed to update the phone's Live Activity.", "检查运行中的服务是否仍持有更新手机实时活动所需的 token。"},
	{"HQ records", "HQ 档案", "HQ archive storage", "HQ 档案存储", "Reports HQ archive size, local snapshots and export status.", "检查 HQ 档案占用、本地快照及导出情况。"},
	{"gtmux disk usage", "gtmux 磁盘占用", "gtmux storage", "gtmux 存储占用", "Measures the main gtmux data directories and identifies large consumers.", "统计 gtmux 主要数据目录的占用，定位较大的数据来源。"},
	{"phone uploads", "手机上传", "Uploaded files", "上传文件", "Measures files uploaded to this Mac from the phone or send commands.", "统计手机及发送命令上传到这台 Mac 的文件数量和占用。"},
	{"log store", "日志库", "Log retention", "日志保留", "Checks the diagnostic log store against its age and size retention rules.", "检查诊断日志是否符合按时间和体积保留的规则。"},
	{"recording", "记录写入", "Log writes", "日志写入", "Checks whether diagnostics, events, sequence counters and the existing knowledge ledger are writable.", "检查诊断日志、事件流、序号计数器及现有知识台账是否可写入。"},
	{"runaway writer", "刷屏的组件", "Repeated log activity", "重复日志活动", "Checks runaway-log warnings recorded in the last seven days.", "检查最近 7 天是否出现日志异常增长告警。"},
	{"recent errors", "近期报错", "Recent errors", "近期错误", "Counts diagnostic errors from the last 24 hours and identifies the affected components.", "统计最近 24 小时的诊断错误并指出相关组件。"},
	{"file modes", "文件权限", "Data file permissions", "数据文件权限", "Checks whether private gtmux files can be read by other users on this Mac.", "检查 gtmux 私有文件是否可能被这台 Mac 的其他用户读取。"},
	{"other stores", "其他数据", "Other retained data", "其他保留数据", "Checks retained events and other state files for excessive growth.", "检查保留的事件及其他状态文件是否占用过多空间。"},
	{"credential backups", "凭证备份", "Old credential backups", "旧凭证备份", "Finds credential copies left by earlier migrations, without deleting them during this check.", "查找早期迁移遗留的凭证副本，本次检查不会删除它们。"},
	{"home", "家目录", "HQ working directory", "HQ 工作目录", "Checks whether the HQ working directory exists.", "检查 HQ 工作目录是否存在。"},
	{"board", "看板", "Situation board", "态势板", "Checks the board's update time and whether its cells exceed the reading budget.", "检查态势板更新时间，以及单元格内容是否过长。"},
	{"knowledge base", "知识库", "Knowledge base", "知识库", "Counts knowledge topics and records pending distillation.", "统计知识主题数量，以及等待整理的知识条目。"},
	{"event consumption", "事件消费", "HQ event follow-up", "HQ 事件跟进", "Checks whether HQ has consumed recent events or is falling behind the event stream.", "检查 HQ 是否已读取近期事件，以及是否存在积压。"},
	{"HQ conversation health", "HQ 对话健康", "HQ conversation capacity", "HQ 对话容量", "Checks HQ context usage, conversation age and turn count against rotation thresholds.", "检查 HQ 上下文用量、对话时长和轮数是否达到轮换阈值。"},
	{"knowledge distill", "知识蒸馏", "Knowledge distillation", "知识蒸馏", "Checks whether HQ is reviewing accumulated knowledge on its configured schedule.", "检查 HQ 是否按配置周期整理积累的知识。"},
	{"HQ self-check", "HQ 自检", "HQ self-check", "HQ 自检", "Checks whether HQ's scheduled review of its own records is overdue.", "检查 HQ 对自身记录的定期审查是否逾期。"},
	{"knowledge promotions", "知识晋升队列", "Pending knowledge distribution", "待落实的知识规则", "Checks knowledge promoted into instruction briefs that still need to be applied and recorded.", "检查已整理为指令规则、但尚未落实并登记完成的知识。"},
	{"knowledge sync", "知识分发", "Knowledge synchronization", "知识同步", "Checks whether installed agents have the current machine knowledge in their instruction files.", "检查已安装 agent 的指令文件是否包含最新的本机知识。"},
}

func doctorReportCopy(label string) (title, purpose string) {
	for _, copy := range doctorPurposes {
		if label == copy.keyEN || label == copy.keyZH {
			return i18n.Tr(copy.titleEN, copy.titleZH), i18n.Tr(copy.purposeEN, copy.purposeZH)
		}
	}
	if strings.HasSuffix(label, " hook") {
		agent := strings.TrimSuffix(label, " hook")
		return i18n.Tr(agent+" state hook", agent+" 状态上报"), i18n.Tr("Checks the hook configuration that connects "+agent+" state changes to gtmux.", "检查 "+agent+" 向 gtmux 上报状态变化所需的 hook 配置。")
	}
	return label, ""
}
