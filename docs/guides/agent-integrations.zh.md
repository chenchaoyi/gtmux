---
title: 各 Agent 如何接入 gtmux
description: 按 agent 讲清接入实现、安装方式和支持边界。
order: 6
---

[English](agent-integrations.md) · **中文**

这是 gtmux 各 agent 接入方案的统一入口，以当前仓库实现为基准，
于 **2026-10-19** 对照源码核对。“支持”要分开看：能识别进程、接收事件、读取对话、
控制会话，是不同的能力。下面覆盖注册表中的全部 **14 个 agent**，也列出仅做了部分接入的项目。

头像资料弹窗的模型、版本、套餐、模式、推理选项及执行链，另见
[逐 Agent 信息设计](../design/agent-profiles.zh.md)。该文区分现有采集和待实现来源，不代表资料接口已经交付。

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

状态判断的设计原则是：能绑定到当前会话且可信的原生 hook 优先。其他证据用于补充 hook
没有提供的信息，或处理不支持 hook、已确认的上游故障，不能悄悄覆盖可靠的原生生命周期事件。
例如 Codex 的权限 hook 早于自动审查结果，因此适配器需要确认实际出现的审批菜单，才报告等待用户。
没有证据代表未知，不代表已完成。下面的 dst 链路通过独立原生事件状态落实这一优先级；
其他适配器仍保留各节说明的恢复机制。

## 通过其他可执行程序启动的 Codex

启动器不同，不会改变 harness 的身份。包装器在 tmux pane 里启动真正的交互式 Codex 时，
可以复用普通 Codex 的输入、草稿保护、hook、对话绑定和投递回执。现有启动器记录会把
启动命令与 Codex 身份分开保存，不会把某个内部包装器品牌注册成一种新 agent。

仅被雷达识别还不够。共享模块 `internal/codexterminal` 会在两秒总预算内读取两次进程快照：
须有且只有一个 Codex 子进程使用这个 pane 的控制终端和前台进程组，且标准输入确实连着
同一个终端。macOS 通过 `lsof` 只读标准输入文件描述符，Linux 读取 `/proc/<pid>/fd/0`。
进程快照只取进程名与出生时间，不取完整参数。模块会复查祖先进程身份、当前 pane 和
标准输入；采样间新增了另一个可能的 Codex 客户端，也会拒绝判断。随后仍按原有会话 ID、
目录与唯一性规则路由 hook；消息绑定凭据还会固定客户端和 pane 根进程的出生时间、终端
及启动器。恢复命令沿用身份 hook 使用的通用启动器规则；启动器不兼容原有恢复参数时，仍需要自己的启动配方或适配器。出生时间只有系统提供的秒级精度，因此会话与消息凭据的精确核验仍须保留。

包装器通过管道调用的 app-server、脱离终端的工作进程或后台 Codex，不算交互式终端连接。
自行控制这类后端的前端需要专用、能指定目标对话的输入适配器，类似 dst 对 DeepSeek Harness
的适配。缺少进程或终端连接证据的旧外部记录仍保留为外部记录；不能因为历史可读、目录相同
或启动宿主名字相同，就把它接到当前 pane。读取终端元数据无需发送测试消息，但元数据核验
不代表已经验收了完整模型回合或消息投递。

## 外部对话从哪里启动

Agent 身份仍是 harness 本身。例如 `Codex · sample-client` 表示曾观察到这个客户端
承载 Codex，不是新增了一种 agent。`launch_host` 是**hook 当时看到的启动宿主**，
不能确认现在由哪个前端接收输入。没有来源证据且控制端未确认时，行上显示
`Codex · 外部会话`；详情保留“当前输入客户端未确认”，有宿主证据时另列记录时间。

提取由 Go 核心的 `internal/launchhost` 完成，`internal/hook` 调用，各界面不另做识别：

1. 只在 `SessionStart`、`UserPromptSubmit` 或显式 `Resumed` hook 上，从 hook 的父进程
   开始观察。Codex 要先采集再把 worker 脱离父进程；脱离后祖先关系会丢失。worker 通过
   内部环境变量接收有大小、时效限制的快照，不重新猜测原来的父进程。
2. 最多查八层，共用 350 毫秒期限。每次 `ps` 只读父 PID、进程启动时间和可执行命令名
   `comm`，不读完整参数；单条命令标准输出上限 4 KiB。只沿这条祖先链走，不遍历整机进程，
   也不扫描打开的文件。
3. 按现有 agent 注册表匹配可执行命令，略过普通 shell 和启动跳板。找到 agent 后，macOS
   优先取外层 `.app` 的 `CFBundleDisplayName`，其次 `CFBundleName`，再退到 app 文件名；
   plist 必须是普通文件且不超过 256 KiB。没有 app 名时取可执行文件 basename；Linux
   使用 basename。遇到 `node`、`python` 等通用运行时、tmux 或系统服务边界、无效名称、
   父进程消失、循环或超过层数，就保留未知。运行时位于可识别的 macOS app 内时可用 app 名。
4. 接受前重读整条已采样链；父 PID、命令名或启动时间有变化就丢弃。PID 和启动时间只留在
   私有 native 记录；CLI JSON 和 HTTP 只输出 `{name, kind, observed_at}`，kind 为
   `app` 或 `process`，时间为 Unix 秒，不输出可执行路径或参数。

其他 hook 保留原来的宿主和记录时间；新的身份 hook 会替换观察，新的来源未知时也会清空旧值。
旧记录不补猜，等下一次自然发生的身份 hook。仅从历史文件发现、没有 hook 的对话，或藏在无法
辨认的解释器后面的进程，可能一直没有来源。此功能不另装来源识别插件。

进程启动时间只有系统提供的秒级精度；进程名和 app 元数据也可以被所有者改动，只能作为历史显示线索。它们不证明当前控制端、存活、
窗格绑定，也不授予发送、跟进或接管权限；多个对话可以共用一个宿主进程。创建来源、宿主观察、
当前控制权分别保留，终端、菜单栏、手机、iPad 和 Web 都消费同一份核心结果。

## 哪些外部 Codex 对话继续显示

实时总览不把保存的历史当成客户端仍然打开的证明。Codex 观察记录若已完成或处于空闲、
**没有记录进程 PID**，且**未开启 HQ 跟进**，就退出雷达列表。观察记录、对话文件和授权
边界均保留；后续收到新的工作中／等待输入 hook，或用户主动开启 HQ 跟进时，可按原有
条件再次显示。这不代表确认前端已经关闭。记录了进程的空闲会话继续按存活检查处理；
进程元数据暂时无法读取时，仍适用原有的 12 小时宽限。DSH Web 从保存记录发现会话的
机制不变。

Codex 每次生命周期 hook 还会在工作进程转入后台前，单独采集原 agent 的 PID、可执行
文件名和启动时间；最多读取 8 层祖先、总时限 350 毫秒，每条命令输出不超过 4 KiB，并
重新核对进程链。此采集不依赖能否识别启动宿主。传给后台工作的内部数据超过 60 秒即
失效。后续检查会核对精确启动时间：PID 已被复用则移除，启动时间无法读取则使用未知
进程的宽限。旧的无 PID 记录不会按目录、标题或宿主名称猜配进程。这里证明的只是后端
进程存活，不证明前端独占控制，也不授予输入权限。启动时间只有秒级精度，同一秒内
同名进程复用 PID 无法区分；采样统一用 UTC，避免时区变化被误判为换了进程。

Mac 菜单栏的“外部会话”和“不在 tmux”均可点击标题展开／收起。收起后保留标题和数量，
不影响其他分组，重新打开菜单栏仍记住选择。折叠只改变展示，跟进仍按每条对话设置。

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

Doctor 区分文件已安装、实时绑定和输入权限。适配器支持普通 `dst --backend dsh`、终端派发与恢复，以及 HQ 逐条读取公开消息并明确标记审阅。Web 继续外部只读，仍无 Web adopt 和一次性执行；较早的 Web 或后台区间不自动纳管。知识写入 `$DSH_HOME/AGENTS.md`。见[终端设计与验证边界](../design/dsh-terminal-clients.zh.md)。

实现：[安装、上游契约与官方图标](deepseek-harness.zh.md)、
[插件源码](../../internal/app/dsh_plugin.mjs)、[文件读取器](../../internal/dsh/log.go)。

已验证且接入原生生命周期事件的终端客户端，会单独保存一份原生事件状态，绑定当前
harness、数据目录、会话、pane 和客户端进程实例。开始、提问、恢复和结束 hook 决定状态；
屏幕/CPU 变化及通用 active/finished 缓存不能覆盖它。“完成时间”缓存也可能由雷达推断生成，
因此不能证明原生回合已结束。

同一个 DSH 会话的持久化历史若出现更晚的明确回合边界，可以补救漏掉的 hook。
历史的默认状态、不相关会话和更旧的边界不能覆盖原生事件。读取复用现有增量 DSH 索引，
不扫描全局事件日志。两种证据都没有时保留中性状态（`running`），不编造完成时间。
旧会话需要自然发生的 hook 或明确历史边界，不猜测补填，也不需要额外安装插件。
原生提问在界面重绘时保持等待，直到原生恢复/结束事件或更晚的明确回合边界出现。
这项能力由适配器声明，换社区客户端时 harness 身份仍为 `dsh`；其他接入保留各自文档说明的回退。
已跟踪但仍卡在输入框中的未投递目标，继续使用独立的投递等待判断。


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


### DSH 用量与账户支持边界

DSH v4 用量读取原生持久化的 `assistant/message`、`assistant/attempt`，包含压缩流中最后一个 usage 样本。同一 turn/step 的修订替换之前的样本；`llm/retry-started` 之后另计一次尝试。输入、缓存读取/写入和输出分别计数，推理 token 已包含在输出中，不再叠加。会话及本地日期统计复用已有的增量索引，支持原始 JSONL 与校验后的 Zstandard；无需插件，也不调用模型。上下文数值仅表示最近一次请求的输入/缓存压力，分母必须有原生 `contextWindow` 记录，不还原压缩后的实时上下文。缺少或不支持的记录表示未知，不表示没有花费。卸载 agent 或移走日志后仍保留历史日统计。

DeepSeek Account 余额通过现有 gtmux 插件中的原生 `deepseekAccount` Cordis 服务读取，不需要额外 npm 插件，也不走 Codex 兼容 hook。`gtmux install` 更新 gtmux 自己拥有的文件；`gtmux doctor` 能发现旧桥接，`gtmux doctor --fix` 可以修复。正在运行的 Web/TUI 需要在下次正常启动时加载更新；读取用量不会替用户重启。观察器只用原生账户标识和 `getBalance`，不调用取凭证的 `resolveToken`、`getPlatformSession`，不登录、不请求模型、不确认领取赠送。离开原生服务的只有账户标识摘要、币种、充值/赠送钱包的原始十进制金额及观测时间。缺少可核对的账户标识或服务不支持时不展示余额。

账户切换后丢弃旧请求；同一 DSH 目录内 Web/TUI 对同一账户的观测合并显示。单次读取有 8 秒展示期限，但原生服务无取消参数，所以超时后仍保留一个在途位置，直到原调用结束，避免堆叠重试。每个已加载桥接至多每 5 分钟刷新一次，15 分钟后观测过期。这是钱包金额，不是订阅百分比或已付费套餐。API Key 与自定义 Provider 还没有经过核实的余额适配器，会明确说明暂不支持；不能从 DSH 或模型名猜计费方。

来源列表根据注册表的安装探测和实际会话、历史及账户数据动态生成。安装只表示本机可用，不证明订阅情况；卸载后历史用量仍保留。Claude `/usage` 保留缓存观测时间，Codex 保留原始额度记录时间和当时记录的套餐，后者不是实时账单验证。过期读数灰显，不参与当前额度摘要和告警。CLI、手机/iPad 与菜单栏复用同一份 API；内嵌 Web 当前没有额度页面。

依据：[原生 token 投影](https://github.com/deepseek-ai/deepseek-harness/blob/687f282e8625c6b8d11ff58211a90524be2e4317/packages/llm/token-meter/src/usage-projection.ts)、[原生账户契约](https://github.com/deepseek-ai/deepseek-harness/blob/687f282e8625c6b8d11ff58211a90524be2e4317/packages/credentials/deepseek-account/src/index.ts)。核对版本为本机 DSH 0.2.0-rc.2 和固定的上游提交；合成账户测试不代表已读取真实余额，也不代表账单总额。

当前 DSH token 适配器覆盖已有索引里的根会话结算，单次发现至多取最近修改的 128 份文件；不汇总子 agent 会话，也不校验服务商账单。未观测到或更旧的文件不代表用量为零。
