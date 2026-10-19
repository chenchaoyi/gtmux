---
title: 各 Agent 如何接入 gtmux
description: 按 agent 讲清接入实现、安装方式和支持边界。
order: 6
---

[English](agent-integrations.md) · **中文**

这是 gtmux 各 agent 接入方案的统一入口，以 **v1.0.128** 已交付实现为基准，
于 **2026-10-19** 对照源码核对。“支持”要分开看：能识别进程、接收事件、读取对话、
控制会话，是不同的能力。下面覆盖注册表中的全部 **14 个 agent**，也列出仅做了部分接入的项目。

## 共用架构

**Agent key** 是选择适配器的名字，例如 `codex`。**Hook** 是 agent 在特定事件发生时
执行的回调。**桥接插件**订阅 agent 的原生扩展接口，再把事件转给 gtmux。
**对话读取器**只解析已保存的对话，不操作 agent，也不调用模型。

数据从三条路径汇入同一个 Go 核心：

1. **Tmux pane。** 雷达从前台命令或进程树识别 agent，结合标题、屏幕变化、CPU 采样和
   hook 状态判断它在做什么。屏幕和 CPU 提供推测，不能当成结构化事件。终端、菜单栏、
   手机、iPad 和 Web 都消费核心的结果，各端不各写一套 agent 适配器。
2. **Agent 事件。** 命令 hook 或插件调用 `gtmux hook --agent KEY EVENT`，分类器把原生
   事件转换为提示提交、工作、等待、恢复、成功、中断或失败，供状态、通知和 HQ 使用。
   工具开始执行不一定表示正在等人批准，中断也不能记成成功完成。
3. **已保存的对话。** 读取器把经过核实的会话身份映射到本地文件。Claude、Codex、
   OpenCode、Kimi、DSH 有结构化内容读取器。其中 Codex 和 DSH 还有专门的外部对话适配器，
   不依赖 tmux 或已加载的 hook 就能发现历史；其他 tmux 外会话的观测依赖 hook 上报。

交互式 **tmux 会话**的输入始终走 pane：先检查输入框，再粘贴、提交。匹配到提示提交事件
可以确认消息已到达，否则回退到屏幕证据。**收到消息不等于任务成功。** 启动就绪判断同样
只用正向事件证据提前确认，没有事件时继续检查屏幕。没有内容读取器就不提供结构化对话及
`goal`/`last`，终端抓屏仍是单独的视图；没有一次性执行适配器就拒绝 `spawn --oneshot`。

外部历史可读，不代表有输入通道。Codex、DSH 的跟随授权绑定 agent、规范化数据目录和会话
三者；HQ 观测、通知、将未来内容保留到知识库各有授权，均不授予接管权限。访客分享链接
不能读取这些外部历史。

## 能力对照

“进程识别”指内置命令匹配规则，不涵盖所有标题推测或用户自定义规则。“有 hook”表示有
安装器，仍需 agent 真正加载并发出事件。“恢复”表示注册了恢复命令，还要有有效会话记录
并通过可用性检查，不表示可以接管一个外部客户端。

| Agent key | 内置进程识别 | 事件接入 | 结构化对话 | 恢复 / 一次性执行 |
| --- | --- | --- | --- | --- |
| `claude` | `claude` | 命令 hook | Claude JSONL | 可恢复；支持一次性执行 |
| `codex` | `codex` | 命令 hook | Rollout JSONL；外部对话适配器 | tmux 内可恢复；支持一次性执行；外部 Move 不可用 |
| `gemini` | `gemini` | 命令 hook | 无 | 可恢复；无一次性执行 |
| `cursor` | `cursor-agent`、`cursor` | 命令 hook | 无 | 可恢复；无一次性执行 |
| `opencode` | `opencode` | 原生 JS 插件 | gtmux 自行记录的 JSONL | 可恢复；无一次性执行 |
| `kimi` | `kimi`、`kimi-code` | 命令 hook | Kimi wire 日志 | 可恢复；无一次性执行 |
| `copilot` | 无 | 命令 hook | 无 | 有恢复命令；无一次性执行 |
| `kiro` | 无 | V3 全局 hook / 旧版 agent profile | 无 | 有恢复命令；无一次性执行 |
| `dsh` | 核验终端客户端绑定；安装探测 `dsh`/`dst` | DSH 原生组件 | v4 文件；共享 pane 与外部读取器 | dst 原生输入、恢复；Web 只读；无一次性执行 |
| `hermes-agent` | 无 | 仅事件分类器，无安装器 | 无 | 仅有恢复命令 |
| `grok` | 无 | 无 | 无 | 仅有恢复命令 |
| `aider` | `aider` | 无 | 无 | 均无 |
| `crush` | `crush` | 无 | 无 | 均无 |
| `amp` | `amp` | 无 | 无 | 均无 |

## Claude Code — `claude`

**发现与事件。** 雷达匹配 `claude` 及其进程树。安装器向 `~/.claude/settings.json`
合并 gtmux 回调，保留其他 hook。提示提交、权限、提问、工具、压缩、会话和 Stop 事件
共同驱动状态；`StopFailure` 表示失败，MCP elicitation 回调表示询问。当前没有安装顶层
Interrupt 事件，Stop 也不覆盖所有用户中断，因此仍需要屏幕和对话记录补充判断。

**内容与操作。** 读取 `~/.claude/projects/<project>/<session>.jsonl`，区分用户提示和
工具结果，跳过子会话、元消息及压缩后的续接摘要。用量来自 Claude 的消息用量字段。
交互输入走 tmux，恢复命令是 `claude --resume ID`。一次性执行使用
`claude -p … --output-format stream-json --verbose`，从结构化输出和退出状态判断结果。
机器知识通过 `~/.claude/CLAUDE.md` 中的托管区块分发。

实现：[安装器](../../internal/app/hooks.go)、[读取器](../../internal/transcript/claude.go)、
[一次性执行](../../internal/driver/headless.go)。

## Codex CLI 与桌面历史 — `codex`

**发现与事件。** 雷达识别 CLI 进程。安装器写入 `$CODEX_HOME/hooks.json` 中的 gtmux
条目，并在 `config.toml` 启用 `features.hooks`，保留原有 `notify`。默认目录是
`~/.codex`。回调同步执行且有超时；更新后可能需要在 Codex `/hooks` 中审阅信任。
专用分类器处理 Interrupt，并区分提示、权限、工具、会话和压缩事件。

共享 app-server 的回调可能继承别的客户端的 `TMUX_PANE`，也可能没有身份字段。
Gtmux 以会话与 rollout 绑定为依据，不猜一个 pane 来标记等待或结束。权限回调可能早于
自动审批，真正等待用户需要稳定可见的批准菜单作为证据。绑定的 rollout 可以补上漏发的
完成或中断，同时防止误结束下一轮。

**内容与操作。** 读取器合并 `$CODEX_HOME/sessions`、`archived_sessions` 中属于该会话的
已核实 rollout JSONL，过滤注入上下文，保留公开对话。用量单独读取 token 记录。
外部对话适配器直接读本地历史，包括桌面端创建的对话，不先恢复会话。
创建来源不能证明现在由哪个客户端控制；**外部 Codex 的 Move、adopt 和直接输入均不可用**。
Tmux 内恢复使用 `codex resume ID`，一次性执行使用 `codex exec --json`。
Gtmux 自己直接启动交互式 Codex 时，在未显式选择其他模式的情况下加入 `--no-alt-screen`
保留滚动历史。知识分发写 `$CODEX_HOME/AGENTS.md`；用户的 `AGENTS.override.md` 可以优先。

实现：[Codex 专页](../design/CODEX.zh.md)、[客户端身份与控制权](../design/codex-client-lifecycle.zh.md)、
[rollout 读取器](../../internal/transcript/codex_reader.go)。

## Gemini CLI — `gemini`

雷达匹配 `gemini`。安装器向 `~/.gemini/settings.json` 合并命令 hook，使用
BeforeAgent/AfterAgent、BeforeTool/AfterTool、会话、Notification、PreCompress 事件。
只有 `notification_type=ToolPermission` 的 Notification 表示权限等待，普通工具开始不算；
AfterTool 可以解除等待。没有专门安装的中断或最终 API 失败事件，没有对话读取器、
用量解析器或全局知识载体。输入和备用投递验证走 tmux，恢复命令为 `gemini --resume ID`。
尚未接入一次性执行。

实现：[hook 契约与边界](../design/agent-hooks.zh.md)。

## Cursor CLI — `cursor`

雷达识别 `cursor-agent` 和 `cursor`；gtmux key 也接受别名 `cursor-agent`。
安装器使用 Cursor 的扁平 JSON 格式，写入 `$CURSOR_HOME/hooks.json`，默认是
`~/.cursor/hooks.json`。`beforeSubmitPrompt` 提供投递回执，`stop.status` 区分
completed、aborted、error；Shell/MCP、工具结果、会话和压缩事件补充生命周期。
回复文本本身不算完成。桥接不实现 Cursor 的权限决策回调，真实等待依靠屏幕感知。
没有结构化对话、用量、知识载体和一次性执行适配器。输入走 tmux，恢复使用
`cursor-agent --resume ID`。

实现：[安装格式与事件绑定](../../internal/app/agent_hooks.go)。

## OpenCode — `opencode`

雷达匹配 `opencode`。接入采用 **gtmux 编写的 JavaScript 插件**，使用 OpenCode 原生
插件 API。安装到 `~/.config/opencode/plugins/gtmux.js`，由 OpenCode 启动时加载；
只迁移 gtmux 自己旧的 `plugin/gtmux.js`，不改其他插件。这条路径不使用 JSON 命令 hook。

插件订阅 `event`、`chat.message`，用传入的 SDK client 确认会话父子关系，忽略子会话及
身份不明的会话。Busy 表示开始工作，权限或提问结果表示恢复。中断保留为中断，错误保留为
失败，后续 idle 不会把它们改成成功。子进程的 stdin 使用管道或 `/dev/null`，不读取终端。

这份适配器把**之后观察到的**用户提示和最终回复写进 **gtmux 自己的**
`~/.local/share/gtmux/octrans/<session>.jsonl`，读取器再消费它；不导入 OpenCode 全部既有
历史，也不查询它的存储数据库。插件没上报，就不会产生这份内容记录。恢复命令为
`opencode --session ID`，输入走 tmux，没有用量或一次性执行适配器。
知识写入 `~/.config/opencode/AGENTS.md`。

实现：[插件生成器](../../internal/app/agent_hooks.go)、
[gtmux 自有对话记录](../../internal/transcript/opencode.go)。

## Kimi Code — `kimi`

雷达同时识别启动器 `kimi` 和实际进程 `kimi-code`。安装器在
`$KIMI_CODE_HOME/config.toml` 维护带边界标记的 `[[hooks]]` TOML 区块，默认目录
`~/.kimi-code`，保留区块外原始字节。原生提示、TurnStarted、权限请求与结果、工具、
压缩、会话、StopFailure 和 Interrupt 各按语义分类。排队中的提示不算开始工作，
权限等待由 PermissionResult 解除，不由无关工具结果解除。

读取器先查 `session_index.jsonl`，遵守删除记录，再读取
`sessions/<workspace>/<session>/agents/main/wire.jsonl`。支持实际观察到的消息来源字段
形状，区分用户输入和注入内容，从助手内容片段还原回复，避免重复显示部分消息。
输入走 tmux，恢复使用 `kimi --session ID`。没有接入用量和一次性执行，即使上游本身提供
这些功能也不能当作 gtmux 已支持。知识写入 `$KIMI_CODE_HOME/AGENTS.md`。

实现：[TOML 安装器](../../internal/app/kimi_hooks.go)、[wire 读取器](../../internal/transcript/kimi.go)。

## GitHub Copilot CLI — `copilot`

没有内置进程命令识别规则。安装器维护专用 `$COPILOT_HOME/hooks/gtmux.json`，默认
`~/.copilot/hooks/gtmux.json`，使用 version 1、PascalCase 事件和
`bash`/`timeoutSec` 命令条目。提示、停止、会话、工具、压缩和错误回调接入 gtmux hook；
有 hook 接入不等于不装 hook 也能识别进程。

PermissionRequest 发生在策略和自动批准之前，仅记录为事件，不能据此认定正在等人。
ErrorOccurred 只有显式 `recoverable=false` 才把回合记为失败。
没有专用用户中断事件，也没有对话、用量、知识载体或一次性执行适配器。
注册了 `copilot --resume ID` 恢复命令，前提是已有可用会话记录，不保证自动发现会话。

实现：[hook 契约与边界](../design/agent-hooks.zh.md)。

## Kiro CLI — `kiro`

没有内置进程命令识别规则。安装器先看已有配置，再通过有超时的版本探测选择 hook 引擎。
V3 使用全局 `$KIRO_HOME/hooks/gtmux.json`，默认 `~/.kiro`，格式为
`version=v1` 与 trigger/action/timeout/enabled。旧版 CLI 2.x 使用
`$KIRO_HOME/agents/gtmux.json`，必须用 `kiro-cli chat --agent gtmux` 选择该 profile。
无法判断引擎时保留旧版兼容方案；迁移到 V3 时只清理 gtmux 自己重复的旧绑定。

SessionStart、提示、工具前后和 Stop 事件接入桥接。工具开始执行不代表正在等用户。
当前契约没有已安装的 Interrupt、SessionEnd 或 StopFailure；没有内容、用量、知识载体
和一次性执行适配器。注册的恢复命令是 `kiro-cli chat --resume-id ID`，仍需有效映射，
单有恢复命令不构成完整的会话发现能力。

实现：[hook 契约](../design/agent-hooks.zh.md)、[分版本安装器](../../internal/app/kiro_hooks.go)。

## DeepSeek Harness — `dsh`

**先读历史。** 外部对话适配器读取 `$DSH_HOME/sessions`，默认 `~/.dsh/sessions`，
针对 DSH **0.2.0-rc.2** 的 v4 文件格式，支持原始 JSONL 和带校验的连续 Zstandard 帧。
核实数据目录与会话身份，排除子会话，读取公开用户文本、助手文本和工具步骤；排除思考内容、
原始工具结果、注入指令和重放、压缩副本。未写完的尾部等下一轮；未知代际、损坏数据或身份
歧义直接拒绝。发现列表取最近 128 个根对话，精确 ID 仍可读更早的历史。
读取不调用 DSH HTTP API、不读取凭证、不获取写入租约，**不需要插件**。

**再接未来事件。** Gtmux 随二进制打包的是**我们编写的事件桥接脚本**，没有内置 Cordis
框架；Cordis 扩展运行时由 DSH 提供。安装器写出 `$DSH_HOME/gtmux/gtmux.mjs` 和
`gtmux.cordis.yml`，不用另外装 npm 包。下次主动启动 Web 时，使用安装器打印的
`dsh web --patch '…/gtmux/gtmux.cordis.yml'`。安装不会重启现有工作；已有 dst profile 会管理一段自有原生组件配置，其他设置保留。

脚本使用 DSH 自己的 `session/event`、`user-questions/request`，**不走 Codex 兼容 hook**。
它跟踪根会话提示、回合结果和未回答的问题，将问题原样交给 DSH，不代答、不批准。
`approval/asked` 不能证明正在等用户处理，批准卡片检测尚未实现。队列和子进程有边界，
stdin 使用管道。终端适配器核对数据目录、前台会话、进程祖先和 tmux 代际，再开放受管 pane；后台挂载本身不授予输入能力。原生命令拿到真实目标 agent，校验一次性请求并返回对应回执，保留草稿、菜单和会话切换保护。各端仍显示 DeepSeek Harness 和官方图标，dst 只是内部客户端适配器。

Doctor 区分文件已安装、实时绑定和输入权限。适配器支持普通 `dst --backend dsh`、终端派发与恢复，以及 HQ 逐条读取公开消息并明确标记审阅。Web 继续外部只读，仍无 Web adopt、用量和一次性执行；较早的 Web 或后台区间不自动纳管。知识写入 `$DSH_HOME/AGENTS.md`。见[终端设计与验证边界](../design/dsh-terminal-clients.zh.md)。

实现：[安装、上游契约与官方图标](deepseek-harness.zh.md)、
[插件源码](../../internal/app/dsh_plugin.mjs)、[文件读取器](../../internal/dsh/log.go)。

## Hermes — `hermes-agent`

这是**部分基础接入**。注册表有显示身份、`hermes --resume ID` 恢复命令，以及会话、
LLM、工具和审批回调的专用事件语义。没有内置进程规则、hook 安装器或回执、就绪驱动，
也没有对话、外部对话适配器、用量、知识载体和一次性执行。分类器能识别事件，不代表
已安装一个会发出这些事件的桥接。`--agent auto` 不会安装 Hermes hook。

实现：[注册表](../../internal/agents/registry.go)、[事件分类器](../../internal/hook/classify.go)。

## Grok — `grok`

只注册了显示身份和 `grok -r ID` 恢复命令，没有内置进程规则、hook 安装器、专用事件
分类器、内容读取器、外部对话适配器、用量、知识载体或一次性执行。
有既存记录且满足条件时可使用恢复命令，不代表会自动完成 Grok 接入。

实现：[注册表](../../internal/agents/registry.go)。

## Aider — `aider`

雷达识别 `aider` 进程树并归集进程资源，通过共用的标题、屏幕和 CPU 采样判断状态。
识别到的 tmux pane 可跳转、输入，用屏幕验证投递。没有 gtmux hook 安装器、结构化
对话、token 用量读取器、恢复命令、知识载体和一次性执行。当前支持的是进程可见与 pane 操作。

实现：[注册表](../../internal/agents/registry.go)。

## Crush — `crush`

雷达识别 `crush` 并归集进程资源，使用共用的 tmux 感知、跳转、输入和屏幕投递验证。
没有注册命令 hook 或插件、结构化对话、token 用量读取器、恢复命令、知识载体和一次性执行。
这种仅识别进程的接入不需要安装 hook。

实现：[注册表](../../internal/agents/registry.go)。

## Amp — `amp`

雷达识别 `amp` 并归集进程资源，使用共用的 tmux 状态、跳转和输入路径。
没有 gtmux hook 安装器、对话读取器、token 用量读取器、恢复命令、知识载体和一次性执行。
上游有某个接口，不等于 gtmux 已经接上。

实现：[注册表](../../internal/agents/registry.go)。

## 安装、诊断与实现分工

有安装器的 9 个 agent，可以用上面的准确 key 选择：

```sh
gtmux install hooks --agent codex
gtmux install hooks --agent auto
gtmux doctor
gtmux doctor --fix
gtmux uninstall hooks --agent codex
```

`auto` 根据支持的配置目录或可执行文件识别，只选择有安装器的 agent，跳过能够识别的
hook 禁用配置。不传 `--agent` 仍默认 Claude。Doctor 检查配置，`--fix` 调用对应 agent
自己的安装器，保留其他 hook，归属不清时拒绝覆盖。**配置正确不等于运行中已加载：**
agent 需要实际运行新 hook，Codex 可能还需审阅信任，DSH 要显式加载覆盖文件，安装不表示
自动重启 agent。桌面通知还需要菜单栏 app 和系统通知权限；手机通知需要配对、推送注册
及相应授权。

| 实现位置 | 负责什么 |
| --- | --- |
| [Agent 注册表](../../internal/agents/registry.go) | Key、别名、进程与安装探测、恢复命令、能力声明 |
| [通用 hook 安装器](../../internal/app/agent_hooks.go)、[Claude](../../internal/app/hooks.go)、[Kimi](../../internal/app/kimi_hooks.go)、[DSH](../../internal/app/dsh_hooks.go) | 写哪些文件、配置格式、归属迁移 |
| [事件分类器](../../internal/hook/classify.go) | 原生事件的含义，包括等待与取消证据的边界 |
| [雷达](../../internal/radar/agents.go)、[能力驱动](../../internal/driver/driver.go) | 进程与状态感知、回执、就绪、内容和一次性执行接线 |
| [对话读取器](../../internal/transcript/transcript.go)、[外部对话适配器](../../internal/conversation/provider.go) | 内容解析及数据目录、会话身份核实 |
| [新增 agent 清单](../design/agent-onboarding.zh.md)、[hook 契约](../design/agent-hooks.zh.md) | 后续如何接入，以及终端 agent 的上游事件契约 |

以上能力是对照源码整理的，不表示对每个上游最新版本、终端、桌面客户端和设备都做过实机
验收。注册表与回归测试核对接线和合成契约，真实 hook 上报与运行验收是另外的证据。
接入文件、能力或边界变化时，应同时更新本文及英文版。
