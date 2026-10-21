# 参与者资料：逐 Agent 的信息与来源

[English](agent-profiles.md) · **中文**

**已实现资料采集与手机端展示**，来源核对日期为 2026-10-20。
覆盖[注册表](../../internal/agents/registry.go)中的 14 个 Agent；丰富字段由各自采集器提供。
接入现状见[统一接入说明](../guides/agent-integrations.zh.md)。下表也列出后续候选来源，
不会把上游支持的能力都算成本次已实现。

## 已交付的采集范围

共享投影在 `internal/agentprofile`。原生日志提供 Claude 的模型/版本；Codex 的版本、
供应商、模型、effort、协作模式、审批/沙箱策略及本会话当时记录的套餐；DSH 请求配置与
计划事件；Kimi 主 Agent 请求、计划与权限事件。仅当前会话的 hook 快照补充 Claude
模型/effort/权限、Cursor 模型/版本/可选参数、Gemini 模型/思考等级与预算，以及 OpenCode
根会话的模型/供应商/角色/变体/明确推理参数。

Claude 的可选 PostModelSwitch 要求 CLI >=2.1.251，Gemini 的 BeforeModel/AfterModel
要求 >=0.35.3。仅安装流程探测已知 CLI 版本，旧版或版本不明时跳过；doctor 不把资料观察器
当成必需生命周期 hook。不替换用户 status line。已有安装通过正常的 hook 安装/修复流程
更新；点击头像不会自动安装任何东西。

手机端按需读取 `/api/participant-profile`，不加入 transcript 轮询；仅 owner 和配对设备可读。
历史请求须定位到已核验源会话中唯一的一轮；未知或重复时间拒绝关联。资料截止本轮终止事件
或下一条用户输入，历史资料不使用当前 hook 快照、运行进程链。其他发送者只保留自己的身份，
不借当前 pane。Hook 快照不充当历史存档，因此旧 OpenCode/Cursor/Gemini 消息细节可能未记录。

JSONL 缓存最多 16 个文件，每文件 1024 个元数据点；冷读尾部 2 MiB 加首条头记录，单条限
1 MiB，空闲十分钟淘汰。追加复用字节位置，替换/截断重建；超限标为不完整，I/O 失败另标不可读。
DSH Zstandard 复用帧游标：冷读可能从目标会话文件开头解码，后续只读追加帧；不能说成压缩文件
也严格限读 2 MiB。它不扫描全局事件日志。小型脱敏快照保存在 `participant-profiles/`，
按 Agent/home/会话隔离，目录 0700、文件 0600。

其余七种接入保留注册表身份与可观察到的进程链。本轮未接账户查询、不执行配置、不发模型请求，
也不为缺少采集器的 Agent 猜模型或套餐。

## 展示原则

沿用已经选定的深色居中资料窗，**不把 Codex 示例中的六个字段变成所有 Agent 的固定模板**。
标题是 harness 身份，下面可列实际客户端或角色；资料按可用项顺序流动排列，可用项按两列排列，大字号改为单列。

- 主要信息：对应本轮的模型、版本、模式、推理选项；有依据时增加供应商与账户套餐。
- Agent 特有信息：例如 OpenCode 的角色和模型变体、Aider 的主模型与编辑模型。
- 下方“执行链”默认收起；展开查看已记录的程序、路径和脱敏参数，长命令横向滚动。
- “来源与记录时间”折叠在详情里，不让每个格子充满技术说明。
- 缺失字段不占格；资料为空时统一说明未记录，读取失败另行说明，不铺满灰色占位。整个资料窗不新增工作状态或能力评分。

账户套餐、任务模式、权限策略和推理选项分别保存。不能把 `Pro`、`Plan`、`yolo`、`high`
合成一个“Plan”，也不能把思考开启、token 预算、模型变体都翻译成相同的 effort 等级。
供应商的原值保留；只有明确的 effort 字段才使用“推理强度”标签。

点某条消息的头像，资料默认说明**这条消息当时的参与者**。当前会话资料可另列为“当前运行”，
不能用今天的安装版本、当前模型或当前 pane 替换过去的事实。关闭弹窗保留阅读位置和草稿。
人的资料保持“人类”标题、小字“碳基决策者 · 自封”及《沙丘》引文，不增加调侃段落。

资料窗读取期间保留稳定的占位高度，先显示明确的静态加载提示，返回后按资料多少收紧；未完成时不闪出“未记录”，也不残留上一目标的事实。读取失败可重试，成功但没有资料则说明未记录。Agent 身份头部更紧凑，来源与时间按灰色键值行排列。关闭按钮保留独立空间，滚动内容不会从它下面穿过。

旧消息事实与当前启动链分开展示。只有消息和当前 Agent 的源会话 ID 相同，用户点开执行链后才额外读取一次当前资料，并照常由服务端核验绑定；界面只取这份响应的链和观察时间，不把当前模型、套餐或推理强度填入旧消息。响应的会话与 Agent 也必须匹配消息资料。顶部当前资料复用已有响应；拼接的旧会话或其他发送者不会借用另一运行实例，没有记录时明确说明。启动链是最近一次记录的进程关系，不保证当前控制权，也不冒充消息当时的进程。

tmux 雷达行以 pane 定位，按既有契约不提供 `session_id`；缺少该字段不代表启动链没有采集。
消息详情可带消息的源会话 ID 和 pane ID 请求观察记录，由服务端核对 pane 的当前绑定，
客户端再核对响应的会话与 Agent。pane 关闭或绑定改变时明确提示读取失败，不借用另一运行实例。
外部会话仍须在请求前具有明确匹配的源会话 ID。

DSH Web/dst 共用的原生桥接直接启动 hook 子进程，不带已脱离原进程的 `--detached` 标记，
让原生输入、恢复事件能记录真实父进程链。采集保持有界，不阻塞 DSH；Web 的祖先进程关系
不授予 pane 控制权。更新 gtmux 自有插件文件不会让运行中的 harness 自动重载；
过去未采到的记录不能用后来的进程树补写。

## 既有接入的边界

雷达 `agentJSON` 仍负责身份和生命周期，资料由新接口单独组合。Codex 全局额度概览不参与
资料归属判断；本会话记下的套餐只证明当时记录，不是实时账户/计费核验。OpenCode 既有
`octrans` 仍只保存文本，不能还原从未留存的历史资料。下表包含已交付来源及后续候选，
具体采集范围以上文为准。

## 14 个 Agent 的字段设计

| Agent | 模型与供应商的可接来源 | 推理与模式的可接来源 | 套餐边界 | 资料窗的差异 |
| --- | --- | --- | --- | --- |
| `claude` | `SessionStart.model`、`PostModelSwitch.to_model`；对应助手消息中的模型可补充本轮记录 | 原生 hook 的 `effort.level`、`permission_mode`；`plan` 可表示计划模式，其他值仍按权限策略显示 | 通用 hook 不提供套餐。账户状态读取要单独核验并绑定对应账户；有额度不等于知道 Pro/Max | 模型、推理强度、任务模式、权限策略；版本有本轮记录或运行快照时显示 |
| `codex` | 对应 rollout 的 `session_meta.model_provider`、`turn_context.model` | `turn_context.effort`、`collaboration_mode`；审批与沙箱单列 | 本会话/账户有对应 `rate_limits.plan_type` 才显示；不能从全局额度借值 | 模型、推理强度、协作模式、权限；CLI、桌面与包装器保留同一 Codex 身份 |
| `dsh` | v4 `request/header` 的 `data.header.config.provider/model` | 同一 config 的 `reasoningEffort`；`plan/mode` 的 `active`，以及已确认的原生 plan 投影 | 不假设有“DeepSeek Pro”；供应商或登录方式不证明套餐 | DeepSeek Harness 为标题，dst/Web 是使用方式；harness 与客户端版本分开 |
| `opencode` | 原生插件 `chat.message` 的 `model.providerID/modelID`；`chat.params` 的模型、供应商 | `agent` 是角色，`variant` 是模型变体；仅明确的供应商 reasoning 选项可列推理强度 | OpenCode 可以连接不同供应商，品牌、OAuth 或模型名不证明账户套餐 | 角色（Build/Plan/自定义）、供应商、模型、模型变体；不把所有变体叫 high/low |
| `kimi` | 主 Agent 的 wire `llm.request.provider/model/modelAlias` | `thinkingEffort`；`plan_mode.enter/exit/cancel`；权限另取 `permission.set_mode` | wire 请求不提供套餐。官方账户查询属于另一路径，未接入时不显示猜测值 | 模型及别名、推理选项、计划模式、权限；过滤主 Agent 之外的记录 |
| `gemini` | 版本门禁下的原生 `BeforeModel/AfterModel.llm_request.model` 观察 | `llm_request.config` 中经版本核验的 thinking 选项；现有通用 hook 没有计划模式字段 | 登录方式、Gemini 模型或 Google 账户都不证明订阅层级 | 模型、思考等级或预算按原语义展示；计划模式缺证时不按默认配置补猜 |
| `cursor` | 原生 hook 的 `model_id`，旧版可退到 `model` | 可选 `model_params` 的 effort/thinking/context，各按参数类型展示；计划模式需独立证据 | `user_email` 不是套餐，也不进入资料 DTO | 显示“Cursor 版本”（`cursor_version` 的原义），不能冒充独立 CLI/harness 版本 |
| `copilot` | 现有命令 hook 契约未给出模型；启动参数有记录时仅显示“启动时选择” | 同样不从工具事件推断 Plan 或 effort | GitHub 登录不等于知道 Free/Pro/Business 等层级 | 先显示已绑定身份、运行方式、已观察版本和启动记录；缺少已核实请求来源时不填模型格 |
| `kiro` | Agent profile 中的 `model` 仅是配置；现有 hook 无已核实的实际请求模型 | profile 名称不等于计划模式，工具权限不等于 effort | 不从 Kiro 品牌或登录推断套餐 | 有来源时显示配置 profile；CLI 2.x 与 3.x 的 hook 引擎分别适配，不读取 IDE 数据冒充 CLI |
| `hermes-agent` | 当前只有部分事件分类与恢复命令，没有安装器、内容或模型读取器 | 暂无已接入的模式/推理来源 | 无已接入来源 | 只展示实际观测的身份与运行事实；不因为同名进程就宣称完整资料可读 |
| `grok` | 当前只有恢复命令，未锁定可读模型协议 | 暂无已接入来源 | 无已接入来源 | 不将某个同名社区 CLI 或 xAI 模型自动选作协议；只显示已记录身份与运行事实 |
| `aider` | 当前只有进程识别；未来可记录显式 `--model`、`--editor-model`、`--weak-model` 启动选择 | `code/ask/architect/help` 是 Aider 的聊天模式；编辑格式另列，不等同 Plan | 供应商 API 或本地模型与订阅层级分别看待 | 预留主模型、编辑模型、辅助模型和编辑格式；当前无实时读取器，不声称知道会话内 `/model` 的变化 |
| `crush` | 当前只有进程识别；large/small 模型槽位、供应商配置不是当前会话调用证明 | 配置可能有 thinking/reasoning；当前无观察器 | 不能把 Hyper 订阅或其他 API 来源强加给所有 Crush 会话 | 当前保持基础资料；未来适配要区分 large/small 槽位，不执行 `crushrc` 来“读取配置” |
| `amp` | 当前只有进程识别；上游按模式路由模型，可能随服务更新 | 原生运行模式需要专用来源，不从模式名称反推固定模型或 effort | 连接了某家模型订阅不证明 Amp 账户套餐 | 优先展示被观察到的模式；缺少实际请求证据时不列硬编码“模式 → 模型”映射 |

### Claude Code

优先扩展已有原生 hook 的字段提取，使用 session/prompt/agent 身份约束。
`effort.level` 是 hook 当时的取值，`SessionStart.model` 可能缺失且不会追踪之后的每次切换；
需要核验并增加 `PostModelSwitch` 观察，不能将 `PreModelSwitch` 的意图视为已经切换。
助手消息模型与会话选择不同的时候，保留本轮模型，不用选择值覆盖回复记录。

官方 status line 还有 `version`、模型和 effort，但不为资料功能覆盖用户现有 status line。
若未来增加兼容采集，必须透明组合原脚本；当前设计的 hook 路径不依赖这种修改。
账户套餐暂不承诺自动获得：即使官方账户状态能报告当前套餐，也须确认该会话使用同一个
账户与付费路径，不能把 API/第三方网关会话标成当前登录账户的 Max。

### Codex

版本来自**包含目标消息的物理 rollout 段**的 `session_meta.cli_version`。
同一个逻辑会话可能恢复、换版本并继续到新文件，不统一套用最早创建版本。
模型、effort、协作模式绑定 turn；审批策略、沙箱与协作模式分别展示。
`rate_limits.plan_type` 是账户额度记录，不是协作模式；缺少账户/会话对应证据时不显示套餐。
通过桌面或其他 binary 承载不改变 `codex` key，也不因此获得输入权限。

### DeepSeek Harness

复用已有 v4 JSONL/Zstandard 增量读取与原生 `session/event`，只提取上述白名单字段，
不读取请求正文、系统提示或思考。`request/header` 是请求记录，默认值未写出时不能猜一个
reasoningEffort；`adapterDefaults` 标志有依据时可以说明取值来自适配器默认。

计划模式能力未装配时没有 plan 投影，不能将其解释为“标准模式”。有能力与完整原生投影
时才使用其 `active/pending` 语义；只有零散历史事件时保留未记录。切换尚待生效可以在
资料行注记“切换中”，不增加雷达状态。

dst 显示在“终端客户端”行，另列客户端版本；harness 版本只认该进程实际加载的软件包或
运行记录。全局 `dsh` 和 dst 自带的后端依赖可能不同，不能拿全局 npm 包版本代替后端版本。
Web 历史没有活进程链时只显示已经保存的运行来源，不为查资料启动、挂载或写入会话。
DSH header 的 `version=4` 是**日志格式**，不能显示成 harness 版本。

### OpenCode 与 Kimi

OpenCode 复用已有原生插件，在已核实的根会话 `chat.message` / `chat.params` 中增加独立的
元数据观察。只记录字段，不修改输入、参数或权限；隔离 title/compaction/子 Agent 请求。
`variant` 是选择的模型变体，不证明最终 API 接受了等价 effort；有经核实的供应商字段才
另显示推理参数。旧 `octrans` 文本不能反推丢失的历史元数据，也不为此导入全部上游数据库。
版本需来自实际服务端运行，不借插件 SDK 版本或另一套 PATH 上的 CLI。

Kimi 对 `agents/main/wire.jsonl` 增量解码指定类型，排除 compaction 请求与子 Agent。
`llm.request` 表示最近请求；`profile.bind/config.update` 表示选择，可能比下一次请求早。
有选择记录时可明确区分“当前选择”与“最近请求”，没有新事件时不把旧请求值称为实时选择。
计划正文的 `plan.revision` 不等于进入计划模式；必须按 enter/exit/cancel 语义恢复状态。
wire 开头的 `protocol_version` 是日志协议版本，也不能充当 Kimi CLI 版本。

### 只有部分接入的 Agent

Gemini 的新模型观察需版本校验后通过安装器接入；只读白名单，不序列化整个 `llm_request`
的消息与配置。BeforeModel 是观察当时的请求提案，后续 hook 可改变它，不能标成服务器确认。
Cursor 的新可选字段也先做版本契约测试，旧版缺字段保持可用。
Copilot、Kiro、Hermes、Grok 不因上游 SDK 提供别的接口，就宣称当前命令 hook 已提供这些值。

Aider、Crush、Amp 当前不增加偷偷操作 TUI 的信息采集：不发送 `/model`、`/status`，不启动
模型回合，不求 agent 自报版本。显式启动参数只作为启动快照；默认配置和运行中选择变化
没有绑定观察时不推断。Crush 新配置是可执行脚本，资料读取绝不执行它。

### 各类资料窗的具体排布

共同骨架保持“头像与名称 → 主要资料 → 执行链 → 来源详情”，主要区优先放 4–6 个有用字段，
额外资料在同一滚动窗内接续，不缩小字号来塞满。不新增品牌专属页面或新导航。

| 资料窗 | 主要资料的优先顺序 | 接续资料与省略规则 |
| --- | --- | --- |
| Claude Code | 本轮模型、版本、推理强度、任务模式；有对应证据才列套餐 | 权限策略另列；只取得启动模型时注明启动记录，不把它称为本轮响应模型 |
| Codex | 本轮模型、版本、推理强度、协作模式、已绑定套餐、运行方式 | 审批/沙箱放接续资料；桌面客户端名称与启动宿主保持不同含义 |
| DeepSeek Harness | 模型、供应商、推理选项、计划模式、harness 版本、运行方式 | “终端客户端 dst”及其版本另列；Web 没有终端客户端行；计划能力缺证不填“标准” |
| OpenCode | 当前角色、模型、供应商、模型变体、服务端版本、运行方式 | 自定义角色保留原名；只有明确 effort 才增列推理强度；无套餐证据不放占位格 |
| Kimi Code | 本轮模型、模型别名、推理选项、计划模式、版本、运行方式 | 供应商、profile 与权限放接续资料；当前选择有新记录时与最近请求分开，不并列冲突值而不解释 |
| Gemini / Cursor | 模型、该产品实际提供的推理参数、已观察版本、运行方式 | Gemini 保留思考等级/预算的差别；Cursor 保留 thinking/context/effort 参数的差别 |
| 其余部分接入 | 已确认的名称、运行方式、版本或明确启动选择 | 原生请求字段未接入时弹窗自然变短；Aider 多模型、Crush 槽位、Amp 模式保留各自标签，缺少观测不显示虚构值 |

这些是字段优先级，不是示例值。实际只有两项有证据时就展示两项，资料少不表示异常或“控制端未确认”。

## 共享实现契约

这是一套共享资料模型加逐 Agent 解码器，不在手机、菜单栏、Web 各写一组品牌判断。

1. 身份从注册表派生。资料能力只声明对应采集器和字段，新增 Agent 默认只有基础信息，
   不继承 Codex 的模型、套餐或模式能力。专用数据解释留在各自适配器。
2. 每条事实携带 Agent、规范化 home 的不透明标识、会话、参与者、可选 turn/消息范围、来源、
   记录时间与有效区间。账户事实另有内部账户范围；`cwd`、显示名和 pane 单独都不足以绑定。
3. 内部区分支持、缺证、不适用、读取失败与过期；界面按上面的简化规则展示。读失败不写成
   “不支持”；也不把“缺少 effort”写成“关闭思考”。先按事实范围选取，再比较时效和来源。
4. 生命周期延续“可信原生 hook 优先”。资料按字段选最贴近事实的原生来源：本轮响应记录、
   已生效模式事件、请求参数、启动快照各有含义，不设一个把所有日志压在 hook 之后的总优先级。
5. Hook/插件只保存有界、脱敏的字段；日志复用增量游标，按文件身份、截断、替换清缓存。
   不为每次点击或两秒轮询全量解码历史或全局 events，也不在每个气泡渲染时执行 `--version`。
6. 新的只读资料接口按已有会话访问范围授权。执行路径、命令和账户事实只随已有 owner 权限
   返回；访客不能通过头像旁路获取它们。不扩展现有访问策略。

## 执行链与版本

现有 [`launchhost`](../../internal/launchhost/launchhost.go) 只输出宿主名称、种类和观察时间；
它故意不输出完整 argv。执行链由独立、有界的原生 hook 祖先进程采样提供。

- 优先使用 gtmux 自己保存的启动配方，或在可信原生 hook 当时采样已绑定进程的祖先链。
  PID、启动时间、父子关系要前后复核，进程退出、复用或边界不清时保留未知。
- 展示的是可证的父子执行关系。Node/Python 解释器与脚本可以分开呈现；包装器退出后丢失的
  `exec` 前命令、远程客户端、整个桌面宿主不补猜。多会话共享服务进程也不证明独占控制。
- 路径与参数分开处理。扁平 `ps` 参数无法区分 flag 与提示词，所以当前只保留程序名/路径及已知 Node DSH/dst 的入口脚本，
  不重建模型/模式/effort 参数。任意位置参数、
  提示词、`node -e`、shell `-c`、环境赋值、认证参数、URL 凭据都不原样持久化。
  不先保存完整 argv 再仅在前端遮挡；复制也只复制已展示的脱敏内容，说明它不是可重放命令。
- 版本先取目标消息/运行中明确报告的值；否则在已绑定的运行实例上核实实际可执行产物或
  加载包。PATH 上另一套版本、SDK 版本、协议版本、旧创建版本都不是替代值。
  没有已验证的读取路径就显示未记录，不执行未知包装器来探测版本。

## 实现验收

每个已声明字段都需要该 Agent 自己的脱敏契约样例。至少覆盖：缺字段与旧版、模型切换、
默认值缺失、同会话跨版本恢复、主/子 Agent 隔离、账户切换、共享宿主、历史头像、进程复用、
截断与替换、敏感 argv、无权限访客，以及只识别进程但没有内容读取器的 Agent。
再核验手机/iPad 小屏与大字号的重排、横向命令查看、44pt 点击区域、VoiceOver 和关闭后的
阅读位置。源码、作者构造样例、独立审查和真实模型/实体设备验收分别报告。

本轮以合成契约、历史边界、来源绑定、隐私与 UI 交互回归验证实现。真实模型回合、固定契约外的
第三方版本、实体设备和 VoiceOver 语音验收另报；作者自测不等于独立 review。实现本身不会
把新 hook 应用到用户配置。

## 核对资料

- Claude：[原生 hook 字段](https://code.claude.com/docs/en/hooks#common-input-fields)、[status line](https://code.claude.com/docs/en/statusline#available-data)、[账户状态命令](https://code.claude.com/docs/en/cli-reference)。字段随版本变化；此稿不声称旧版全部具有这些字段。
- Codex：[0.163.0 protocol](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/protocol/src/protocol.rs) 的 `SessionMeta` 与 `TurnContextItem`；gtmux 的 [rollout 读取器](../../internal/transcript/codex_reader.go)和[额度读取器](../../internal/limits/codex.go)。
- DSH：[0.2.0-rc.2 请求配置](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/llm/llm/src/call-config.ts)、[会话类型](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/core/session/src/types.ts)、[计划模式](https://github.com/deepseek-ai/deepseek-harness/tree/dsh-v0.2.0-rc.2/packages/plan/plan-mode)。同时只读核对了本机同版本包与 dst 0.16.1 包元数据；安装版本不证明某条会话的运行版本。
- OpenCode：[1.18.11 插件类型](https://github.com/anomalyco/opencode/blob/v1.18.11/packages/plugin/src/index.ts)、[Build/Plan 角色](https://opencode.ai/docs/agents/)、[现有桥接](../../internal/app/agent_hooks.go)。新上游插件 API 也在演进，接入必须固定版本验证。
- Kimi：[固定源码的 wire 类型](https://github.com/MoonshotAI/kimi-code/blob/5710c5496a213742ca0687f69dd8eeb5bde52dbf/packages/agent-core-v2/docs/wire-manifest.d.ts)、[计划与权限模式](https://www.kimi.com/code/docs/en/kimi-code-cli/guides/interaction)、[gtmux 已有读取器](../../internal/transcript/kimi.go)。
- Gemini：[原生 hook 契约](https://geminicli.com/docs/hooks/reference/)；同时只读核对本机 0.35.3 的 `BeforeModelInput/AfterModelInput` 类型，未运行或变更该 CLI。
- Cursor：[hook 公共字段](https://cursor.com/docs/hooks#input-all-hooks)；没有将当前在线文档的可选字段冒充本机旧 CLI 已提供的字段。
- Copilot：[hook 输入契约](https://docs.github.com/en/copilot/reference/hooks-reference#hook-event-input-payloads)；Kiro：[hook](https://kiro.dev/docs/hooks/types/)及[profile 配置](https://kiro.dev/docs/custom-agents/configuration-reference/)。
- Aider：[聊天模式与多模型](https://aider.chat/docs/usage/modes.html)；Crush：[配置与模型槽位](https://github.com/charmbracelet/crush/blob/main/docs/config/README.md)；Amp：[按模式路由](https://ampcode.com/docs/the-dial#models-behind-the-dial)。这些上游能力没有被计作 gtmux 已接入能力。

## HQ 身份与紧凑展示

弹窗按实际资料高度展开，内容多时在高度上限内滚动；首次读取仍显示固定占位，不闪现“未记录”。已知 Provider 在展示时使用 OpenAI、Anthropic、DeepSeek、DeepSeek Account、Google 的正规写法，原始值和自定义供应商标识不改。缺少历史启动链时只提示“这条消息未记录启动链”。

界面名称仍统一为 HQ，副标题说明“gtmux 协调 Agent”，负责协调 Agent、跟进进度、审阅回执，并把需要用户决定的事带回来。HQ 自身对话的资料沿用回复所属会话的 Agent 身份及历史/当前边界。

在其他会话点开 HQ 来信头像，只采用来信已记录的 Agent 类型；旧投递通常只记了 HQ 角色，用一句灰色说明交代，不把“未记录”铺成主信息。头像统一用 gtmux 标识。“当前 HQ”默认折叠，点击区至少 44pt；展开后另取同一客户端、在线雷达中唯一的 supervisor 行，注明当前信息不代表发信时的 Agent。离线、多个候选或其他服务器的数据不补入，也不借用收信会话的 Agent。这里不增加接口请求或轮询。

右上角全局折叠恢复品牌色“折叠全部 / 展开全部”文案，保留至少 44pt 点击区；单轮回复和工具的折叠互不影响。
