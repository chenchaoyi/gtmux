# 接入一个 coding agent

怎么让 gtmux 认识一个新的 coding agent（Claude Code、Codex、Gemini、Cursor、opencode……）：
流程、身份的唯一来源、以及那些真花过时间的坑。接新 agent 之前先读这份；收工前过一遍末尾的清单。

规范：`openspec/specs/agent-integration/spec.md`。注册表：`internal/agents`。
Codex 的特殊处理与排查入口：[CODEX.zh.md](CODEX.zh.md)。

---

## 1. 支持层级与能力边界

gtmux 对 agent 的支持分层，但这些层级是接入工作的划分，不是一个总开关：回执、就绪、内容、
无头运行和用量各有自己的来源与边界。

| 层级 | 亮起什么 | 需要什么 |
|---|---|---|
| 0 · 可感知 | tmux 雷达行、focus 和输入；声明了 resume 命令才能恢复会话。 | 带检测命令的 manifest（可选：idle 字形、图标、resume argv）。 |
| 1 · 事件对齐 | hook 驱动的 `waiting`/`done`、回执校验、通知和 HQ 唤醒。 | 把生命周期事件送进 gtmux 的安装器；agent 还必须实际执行这些 hook。 |
| 2 · 摘要对齐 | digest 中来自 transcript 的 `goal`/`last`，以及 HQ 会话年龄所用的首条消息时间。 | 会话映射和 transcript 解析器。`ask` 另从 waiting pane 的编号选项读取。 |
| 2+ · 用量对齐 | 上下文/消耗数字（`gtmux usage`）和 HQ 自轮换的 `ctx` 判据。 | 能定位到的会话日志，包含解析器支持的用量记录：Claude 的逐消息用量，或 Codex 的累计 `token_count`。用量不受 driver 的 `content` 开关控制。 |

HQ 传感器首先需要能解析出的会话 ID；三个判据各自取证：

| 判据 | 来源 | 当前覆盖 |
|---|---|---|
| `turns` | 传感器为当前会话建立观测窗口之后，HQ pane 的 `UserPromptSubmit` 事件。 | 实际发出这些事件的 hook agent；不会从完整 transcript 补算此前轮次。 |
| `age` | 建立观测窗口时，能读到就用 transcript 的首条消息时间，否则用传感器首次看到这个会话的时间。 | Claude、Codex、opencode、Kimi 有 transcript 读取器；其他有映射的会话也会累计年龄，但兜底时间会低估已有会话的年龄。 |
| `ctx` | 用量解析器（`internal/usage/parse.go`）。 | 有可用 token 记录的 Claude、Codex 日志；没有记录就没有上下文数字。 |

唤醒行不显示非正的上下文比例和年龄，也不显示未知（负数）的轮次；开始计数后可以显示 `0 轮`。
没有用量数据不等于会话还有上下文余量。

`rotateInput` 只知道 Codex 的 `/new` 和 Claude 的 `/clear`。其他 agent 不返回命令，
`gtmux hq --rotate` 会拒绝未知的重置操作，不会猜一条命令发过去。感知会话年龄和知道怎么重置是两件事。

**能力降级按已交付的 agent-driver 约定处理**（[spec](../../openspec/specs/agent-driver/spec.md)）：
回执或就绪证据缺失时退回屏幕检查。没有内容读取器时，`goal`/`last` 缺省，不从屏幕重建；
`ask` 仍来自 pane，用量仍可独立读取日志。有会话映射、读取器无错误返回时，`sense` 是 `driver`，
即使还没有日志；读取器缺失、关闭或报错时是 `partial`，没有会话映射时是 `screen`。
无头能力缺失或关闭会拒绝 `spawn --oneshot`，不会改起交互 agent。hook 写的状态标记不受 driver 开关控制。

Codex 的共享 app-server 有时会发出缺少会话 ID 和 cwd 的 `Stop`，还带着别的客户端继承来的
`TMUX_PANE`。只有一个活跃 pane 绑定的会话刚在自己的日志里写下 `task_complete` 时，才能把这条
事件归给它。归属不了时，雷达再用该 pane 绑定的日志完成记录清理过期的等待标记；后续若已有新的
`task_started`，就继续保持新回合的运行状态。
Codex 的 `UserPromptSubmit` 也可能漏掉会话 ID，留下没有内容的运行标记。只有已绑定会话的
`task_complete` 晚于这个标记时，Stop hook 或下一次雷达读取才能把该 pane 收为完成；
标记若写着别的会话 ID，就不能清除。

同一个共享 app-server 也可能发出没有 cwd 和会话 ID 的 `PermissionRequest`。
继承来的 pane 不能证明是谁在提问；hook 只用唯一会话绑定确定归属，否则抑制等待、记录诊断，
不写入 waiting 生命周期记录。
雷达随后从真正显示审批菜单的 pane 识别等待。旧 hook 若已给空闲 Codex pane 留下误写的等待标记，
雷达看到就绪输入框后会清掉它。

### Codex 通知的归属

Codex 有两条独立的通知路径：TUI 可借 tmux 透传向 Ghostty 发终端通知；gtmux hook 则把桌面通知交给
菜单栏 App。HQ 的静默规则原本只管后一条，所以 gtmux 启动 Codex HQ 时把 TUI 通知限定为
`approval-requested` 和 `plan-mode-prompt`，只保留确需输入的提醒。全局 Codex 设置及普通 Codex 会话不变；HQ agent 命令中显式写的
`tui.notifications` 优先。已经运行的 HQ 要重新启动进程才会采用新参数；
轮换只在旧进程内发送 `/new`，不会更改启动参数。

hook 对 Codex 采用以下判据：

| 输入 | 所需证据 | 处理 |
|---|---|---|
| 无法确认 pane 的 `Stop` | 缺少唯一的完成日志和会话绑定 | 生命周期记录照留；不发没有跳转目标的通用完成通知。 |
| 已归属普通 pane 的 `Stop` | 确认的 pane 绑定 | 按普通完成通知规则处理；HQ 的例行完成静默。 |
| `PermissionRequest` | 等待短暂稳定后，编号审批菜单仍在该 pane 上 | 此时才标记等待并通知。事件发生在 Codex 自动审核之前，单靠事件不能断言需要人。 |
| 无法确认 pane 的 `PermissionRequest` | 无确认归属 | 不通知、不猜测写入 pane；雷达可从实际 pane 上的活菜单补识别。 |

被抑制的 hook 通知会写入带原因的结构化诊断。屏幕确认是 Codex 尚未提供“自动审核后确需用户”
事件时的保守办法；若真实菜单在检查结束后才出现，雷达下次轮询仍能识别。

---

## 2. 身份只住在一个地方：注册表

历史上每个子系统各留一份按 agent 键的表，键不一致、成员各漂各的。现在 `internal/agents` 里每个 agent 一份 `Manifest`，
每个子系统从它派生自己的表。给一个 agent 加身份，就是写一份 manifest。

```go
// internal/agents/registry.go
type Manifest struct {
    Key, Label      string   // "claude", "Claude Code"
    Aliases         []string // alternate keys that resolve here (cursor-agent → cursor)
    Detect          []string // radar process-subtree commands; empty ⇒ no radar profile
    IdleGlyph, Icon string   // the idle marker its TUI paints; optional app/image path
    Resume          []string // resume argv; nil ⇒ not resumable by session id
    Resource        string   // resource-attribution name; "" ⇒ not attributed
    HookDisplay     bool     // registered in the hook-time known-agent gate
    Hooked          bool     // events feed the receipt/ready stream (Tier 1)
    Content         string   // transcript-parser key (Tier 2); "" ⇒ none
    Headless        string   // headless one-shot key; "" ⇒ none
    Semantics       bool     // has a DEDICATED classifier event table (else the generic one)
    Instructions    string   // global instruction-file path; empty ⇒ not a knowledge carrier
    InstructionsEnv string   // optional env var that relocates the instruction-file home
}
```

这些子系统已经在读注册表，你不用改它们：

| 关注点 | 访问器 | 消费方 |
|---|---|---|
| 装了 hook 的集合 | `agents.HookEquippedKeys()` | `internal/driver` |
| 雷达检测配置 | `agents.Profiles()` | `internal/radar` |
| resume 命令 | `agents.ResumeArgv()` | `internal/resume` |
| 资源归属 | `agents.ResourceNames()` | `internal/resource` |
| hook 显示名 | `agents.DisplayNames()` | `internal/hook` |
| transcript 键 | `agents.ContentKeys()` | `internal/driver` |
| 全局指令载体 | `agents.All()` → `Instructions` / `InstructionsEnv` | `internal/knowledge/distribute.go` |

Headless 仍需在 `internal/driver/driver.go` 手动接线：现在通过 `withHeadless` 显式注册 Claude 和 Codex。
只填 `Manifest.Headless` 不会接通 driver；`HeadlessKeys()` 目前由注册表测试检查。

注册表的数据由 `internal/agents/registry_test.go` 里的 golden 测试钉住（从旧表逐字抄来），
再加每个子系统的迁移守卫测试。

### 能力接线检查与 transcript 样本

把每个非空能力字段都当作必须接通并验证的承诺。注册表填写了 `Content`，driver 就必须有
对应解析器；填写了 `Hooked`，就必须有安装器和显示名映射；填写了 `Semantics`，就必须有专用
分类表。一致性测试要指出缺失的 agent 和能力，不能让只接了一半的集成看起来已经完整。

每个层级 2 解析器还要在包内的 `testdata/` 保存脱敏样本，记录实际观察到的日志形状；至少覆盖
一个当前形状和仍要兼容的旧形状。`internal/transcript/testdata/codex-current.jsonl` 把用户输入记录为
`role: user`、包含 `input_text` 块的 `response_item` 消息；解析器也支持 `event_msg.user_message`。
Codex 解析器会忽略注入的
`AGENTS.md` 和环境上下文；遇到未知事件也不能阻止后续已识别轮次的读取。agent 新版本改变日志时，
样本和这份约定要一起更新。

### 留在各自领域的东西（以及为什么）

三样东西属于行为，留在自己的包里，按注册表的 agent 键索引；把领域枚举搬进纯数据的注册表是过度抽象：

- 事件语义，原生事件 → gtmux 语义的映射表：`internal/hook/classify.go`
  （`agentEventSemantics`，否则用通用表）。
- 提示符 / 就绪签名（启动横幅、提示符/选择器字形）：`internal/prompt`。
- hook 安装规格，gtmux 写的那个文件/插件：`internal/app/agent_hooks.go`。

Go 测试检查具体的接线：`internal/app/opencode_installer_test.go` 查安装器和 hook 显示名；
`internal/driver/registry_wiring_test.go` 查回执、就绪及非空 content 接线；
`internal/hook/registry_conformance_test.go` 查专用分类表。它们由 `go test`/`make check` 运行，
只跑 `scripts/check-design.sh` 不会执行这些测试。解析器样本还必须实际走到 `resolveLog` 的对应分支，
有一个非空包装函数不等于有解析器。`internal/prompt` 的提示符/就绪签名仍需审查。

---

## 3. 一步一步来

### 第 0 步：manifest（必做）

在 `internal/agents/registry.go` 的 `manifests` 里加一条。填 `Key`、`Label`、`Detect`（层级 0）。
能按 id 重启会话的加 `Resume`。跑 `go test ./internal/agents/`，golden 测试会告诉你有没有碰坏已有的 agent。
这就是层级 0：雷达识别、focus 和输入可用；resume 还需要 `Resume`。

### 第 1 步：hook 安装器（层级 1）

设 `Hooked: true` 和 `HookDisplay: true`。然后接一份安装规格让 agent 发事件。有三种扩展模型，先看 agent 支持哪种：

- 命令 hook 模型（Claude、Codex、Cursor、Gemini、Copilot、Kiro）：agent 读一份 JSON/TOML 配置，在生命周期事件上跑 shell 命令。
  在 `internal/app/agent_hooks.go` 加一条 `agentInstaller`，把每个原生事件映射到 gtmux 的记号：
  `beforeSubmitPrompt → UserPromptSubmit`、`afterAgentResponse → Stop`、审批事件 → `PermissionRequest`、会话开始/结束。
  选一个或新加一个 `format`。
- 插件模型（opencode）：agent 没有命令 hook 文件，只有 JS/TS 插件。安装器写一个小插件，订阅 agent 的事件并
  shell 出 `gtmux hook --agent <key> <event>`。入口同样是 `gtmux install hooks --agent <key>`；插件是 `dedicated` 产物，卸载时干净移除。
- 托管块模型（Kimi Code）：agent 的 hook 住在一份 gtmux 不拥有的配置文件里，即放着用户 provider 和密钥的那份
  `~/.kimi-code/config.toml` 里的 `[[hooks]]` 条目。上面两种模型都不合适：没有一整份可以写的文件，而为了改四行去重新序列化
  别人手写的 TOML 也不划算（gtmux 没有 TOML 库，也不该为此引一个）。所以 `internal/app/kimi_hooks.go` 在哨兵注释之间追加一块，
  安装和卸载都会读完整文件，去掉带标记的块，再写回其余文本；安装时追加新块，末尾换行会被规整。
  卸载时，若文件只剩托管块之外的空白，会删掉整个文件。
  这段代码不解析或校验外围 TOML。前面的 TOML 合法时，新 `[[hooks]]` 表头会开启一项，但不能修复本来就损坏的文件。
  托管块若缺结束标记，会一直算到文件末尾。采用这种安装方式前，要用 agent 自己的校验器检查合成样本。

把 agent 的原生事件映射到 gtmux 的：`UserPromptSubmit`、`Stop`、`PermissionRequest`（真正面向用户的审批 → `waiting`）、
`PostToolUse`/resolve（清掉 `waiting`）、`SessionStart`、`SessionEnd`、`PreCompact`/`PostCompact`。如果 agent 的审批信号是一个
独立于工具前事件的事件，给它一张专用语义表（`agentEventSemantics`，`Semantics: true`），让工具前事件留作遥测；
如果它唯一的信号就是工具前事件，通用表的 `semToolStartMaybeApproval` 会替你把有副作用的工具升级成审批。

验证身份是从进程子树解析出来的（见坑清单，前台命令不能当来源）。装好，跑一个真会话，确认 `waiting`/`done` 和带回执校验的
`gtmux send`（`judged_by: driver`）。

### 第 2 步：transcript 解析器（层级 2）

加 `internal/transcript/<agent>.go`，把 agent 的会话日志读成 `[]Turn`，设 manifest 的 `Content` 键（仅此一步就自动接上
`driver.Content`，见 `agents.ContentKeys()`），再给 `resolveLog` + `normalizeAgent` 加解析器分支，并为观察到的日志形状添加脱敏样本。
现在 digest 可以填入 `goal`/`last`；`ask` 仍另从 pane 读取。
pane→会话的映射是白送的：hook 用会话 id 写一条 `resume` 记录，`sessionRef` 读它，所以一个 hook 能拿到会话 id 的可 resume agent
不需要额外接线。

**如果这项集成没有支持的上游 transcript 读取器**，可以留一份 gtmux 自己的副本。opencode 集成就是这样：
插件把用户 prompt 和最终的 assistant 文本通过 `gtmux hook` 流过来（stdin 上管道送 `{session_id, prompt}` / `{session_id, assistant}`），
hook 经 `transcript.AppendOpencode` 以 `{timestamp, role, text}` JSONL 追加到 `~/.local/share/gtmux/octrans/<session>.jsonl`，
解析器读那份。付过学费的两个细节：(a) assistant 文本是以 `message.part.updated` 事件流的形式到达的（`part.text` 是到目前为止的全文），
按 message-id 累积，在 `session.idle` 时把最新的一条落盘；(b) 文件按 agent 自己的会话 id 命名（和 prompt 一起管道送来），
才能和 `sessionRef` 解析出的 `resume` 记录对上。

---

## 4. 坑清单（每一条都付过学费）

- [ ] 启动器的名字不是进程的名字。Kimi 的二进制叫 `kimi`，跑起来的进程叫 `kimi-code`。子树匹配是精确匹配，
  所以只写了启动器名的 manifest 让一个活着的 Kimi pane 在雷达里隐身，实测：对着活会话 0 行，而 `pane_current_command`
  一直显示 `kimi`。`Detect` 里两个都放，并且在活 pane 上验（对 pane 的子进程 `ps -o comm=`），别拿你敲的启动命令当数。
- [ ] 身份来自进程子树，永远别用 `pane_current_command`。Claude Code 把进程名改成版本号（`2.1.220`）；好几个 agent
  就是裸的 `node`。按前台命令定身份会认错 agent，并悄悄关掉回执通道。用 `radar.AgentDriverKey`（遍历子树）。
  代价是一场好几天的「send 永远卡住」追查。
- [ ] hook 必须真的装上，否则整个事件层是黑的。在 driver 的 hook-equipped 集合里是必要条件不是充分条件，
  没有一个真去写 agent 配置/插件的安装器，它就不发事件，层级 1 等于没有（opencode 在白名单里挂了几个月，没有安装器）。
- [ ] 装了 ≠ 信了。有些 agent 对新注册的 hook 要过一次用户确认才会触发；接入 Codex ~0.146 时观察到
  "New hook - review required — press t to trust"。未信任的 hook 即使配置正确，也不会提供事件；
  屏幕检测仍可工作，transcript 摘要还需要能定位到会话。安装器必须说出来；排查「静默 hook」之前先看看 agent 是不是只是在等信任。
- [ ] 插件模型 vs 命令 hook 模型。别假设一定存在一份「事件上跑命令」的 JSON 文件。opencode 只有插件；硬套命令 hook 格式会失败。
- [ ] **shell 出 `gtmux hook` 的插件必须重定向 stdin（`< /dev/null`）。**JS 插件的子进程继承 agent 的控制 TTY 作为 stdin，
  而 `gtmux hook` 会读干 stdin，`io.ReadAll` 对 TTY 永远不 EOF，于是 hook 挂在 agent 的前台进程组里抢走它的键盘输入
  （opencode 的输入框在第一次 send 之后就死了；之后每次 `gtmux send` 都静默失败 `not confirmed`）。`gtmux hook` 现在有守卫
  （`stdinIsTerminal` 跳过字符设备 stdin），但插件仍要重定向，旧二进制才安全。管道喂的调用（`echo … | gtmux hook … UserPromptSubmit`）
  本来就安全，因为它们的 stdin 是管道，碰不到 TTY。代价是一整场调试；征兆是一个卡在 `S+` 状态的 `gtmux hook` 进程。
- [ ] daemon 起的 PTY 会丢 locale/字形。`launchd` 起的 `gtmux serve` 没有 `TERM`/locale，它开的 PTY 会把 CJK 和 TUI 字形
  搅成短横/`_`。强制 `-u` + `LC_CTYPE` 并传 `TERM`。这也会弄坏雷达的字形分类。
- [ ] 事件稀疏就退回第 1 层，没问题。事件密度低的 agent（Codex）只是降低回执命中率；`NoEvidence` 退回两帧屏读。
  别把一个缺失的事件当失败。
- [ ] idle 字形分类要活体确认。死掉的 shell 上残留的标题字形绝不能分类成运行中的 agent，分类器要求进程活着
  （或子树匹配），只有标题不算数。
- [ ] agent 图标：仓库内置的，或厂商已装的 app，否则字母标。§6 现在允许为标识目的（nominative use）提交官方图标：
  把 `<key>.png` 放进 `assets/agent-icons/`，出处记在 `SOURCES.md`；serve 经 `/api/icon` 发给每块屏。有桌面 app 的 agent
  也可以把 `Icon` 指向 `/Applications/<App>.app`。坑：手机端只在 `agents --json` 报告非空 `icon` 时才拉 `/api/icon`，
  所以 `radar.IconFor` 会把内置 PNG 落盘到 `~/.local/share/gtmux/cache/agent-icons/<key>.png`，并在 profile 的 `Icon` 为空时返回那个路径；
  没有这个提示，图标明明发了，手机上还是显示单字标。（opencode 显示 "OC"、Codex 的非 tmux 行显示 "Cx"，直到修掉，就是这么来的。）
  因为提示是真路径而不是不透明记号，菜单栏 app（它把提示当文件打开来解析）也能拿到内置图标，app 侧零改动。
  `~/.config/gtmux/icons/<slug>.png` 仅在 hint 为空或路径不存在时兜底，不覆盖已有的 hint 路径。
- [ ] 审批事件 vs 工具前事件。agent 有独立审批事件的，把工具前事件留作遥测（专用表）。否则要么每个工具都标「等你」，
  要么真审批被丢掉（Kiro 的小写事件必须显式注册）。
- [ ] 通用的工具前事件不应把只读工具标成「等你」。`classify.go` 里的 `sideEffectingTools` 是升级白名单，
  Read/Grep/Glob 等不要放进去；显式的 `PermissionRequest` 是另一种信号，仍表示等待审批。
- [ ] 生成的 manifest 只是对字节的描述，可能是错的。Kimi 附带一份机器生成的 wire manifest，其中三条说法没扛过真会话：
  `origin` 文档写的是字符串，实际写成 `{"kind":"user"}`；assistant 的回复根本不是一条消息记录，而是带 `content.part` 的循环事件；
  UserPromptSubmit 的 `prompt` 在 Claude 是字符串，在 Kimi 是内容片段的数组。每一条都静默失败：JSON 类型不匹配让整条记录失败，
  空 prompt 在它被消费的四处都是空。按 manifest 搭的十三条 fixture 测试全绿，而真日志解析出零个 turn。
  **先拿到真字节再信 schema**，提交一份当 fixture，任何两家 agent 可能写法不同的字段都优先用 `json.RawMessage` + 宽松读取。
- [ ] 拿真字节不需要账号。Kimi 对任意 `base_url` 说 OpenAI chat-completions 协议，所以一个约 40 行的本地假 provider
  （`type = "openai"`，`base_url = "http://127.0.0.1:…"`）就能端到端跑一个真会话：真 hook、真 `wire.jsonl`、真 pane 身份。
  上面每个缺陷都是这么找到的，没有 Moonshot 账号。接下一个 agent 时，先找同样的逃生口，再退而求 fixture。
- [ ] 一个未知字段能弄挂整份配置。Kimi 的 `[[hooks]]` 只接受四个键，多一个就拒掉整个文件，所以往条目里写一个所有权标记
  会让用户丢掉所有 provider，损失远不止一个 hook。用 agent 自己的校验器验过（`kimi doctor` 报 `hooks[11]: Unrecognized key: "owner"`）。
  安装器往用户拥有的文件里写东西时，用 agent 的校验器过一遍结果，并且喂一份坏的确认校验器真能分辨。
- [ ] 键必须一致。一个规范的 `Key`；别的命令名用 `Aliases`（cursor-agent → cursor）。别为某个子系统另造一个键。

### App 侧的备用字母标

App 已读取 Go 提供的 `icon` hint，但各自保留备用字母标表；加 agent 时两处都要核对：

- 菜单栏：`macapp/Sources/GtmuxBar/Components.swift`（`agentMonogram`、`AgentIcons`）。
- 手机：`mobileapp/src/ui/agentMark.ts`。

（备用字母标仍手工维护；图标 hint 已来自 `agents --json`。）

---

## 5. 收工清单

- [ ] manifest 已加；`go test ./internal/agents/` 绿（golden 测试未受影响）。
- [ ] 目标层级真的接上了（层级 1 有安装器，层级 2 有解析器）。
- [ ] 在真 pane 上经进程子树验过身份。
- [ ] `gtmux install hooks --agent <key>` 写入集成；卸载只删 gtmux 写的，用户配置原样保留。
- [ ] 一个活会话驱动了 `waiting`/`done` 和带回执校验的 `gtmux send`（`judged_by: driver`），层级 1 要求。
- [ ] 菜单栏 + 手机的标记/图标已更新。
- [ ] `make check` + `scripts/check-design.sh` 绿。
- [ ] 面向用户的 agent 在 CLAUDE.md / `docs/cli.md` 里有提及；行为变了就更新 spec。
