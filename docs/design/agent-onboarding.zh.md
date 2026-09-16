# 接入一个 coding agent

怎么让 gtmux 认识一个新的 coding agent（Claude Code、Codex、Gemini、Cursor、opencode……）：
流程、身份的唯一来源、以及那些真花过时间的坑。接新 agent 之前先读这份；收工前过一遍末尾的清单。

规范：`openspec/specs/agent-integration/spec.md`。注册表：`internal/agents`。

---

## 1. 心智模型：三个支持层级

gtmux 对 agent 的支持分层。选定你要达到的层级即可，不必一次做完；每一层都能平滑退回下一层。

| 层级 | 亮起什么 | 需要什么 |
|---|---|---|
| 0 · 可感知 | agent 出现在雷达里（行、经标题字形 / 进程子树得到的状态），可 focus、可输入、可 resume。 | 一份带检测命令的 manifest（可选：idle 字形、图标、resume argv）。 |
| 1 · 事件对齐 | `waiting`/`done` 判定、带回执校验的 `send`/派活、通知、HQ 唤醒。 | 层级 0 加一个 hook 安装器，让 agent 把 `UserPromptSubmit`/`Stop`/`PermissionRequest`/… 发进 gtmux 的事件流。 |
| 2 · 摘要对齐 | 该 agent 的确定性 digest（`goal`/`last`/`ask`）、HQ 参谋长视图，以及 HQ 自轮换的 `age` 判据。 | 层级 1 加一个解析该 agent 会话日志的 transcript 解析器。 |
| 2+ · 用量对齐 | 上下文/消耗数字（`gtmux usage`），以及 HQ 自轮换的 `ctx` 判据。 | 层级 2 加一份逐条消息 token 记账能被 usage 解析器读懂的会话日志，目前只有 Claude 的形状（`message.role=="assistant"` + `usage.input_tokens` / `cache_read_input_tokens`）。 |

这对承载 HQ 的 agent 意味着什么：HQ 自轮换是抓「会话老到判不了事」的参谋长的那个传感器，
它感知三件事实，而它们落在不同层级：

| 判据 | 需要 | claude | codex · opencode · kimi | gemini · cursor · copilot · kiro |
|---|---|:---:|:---:|:---:|
| `turns` | 层级 1（事件流） | ✅ | ✅ | ✅ |
| `age` | 层级 2（transcript） | ✅ | ✅ | ❌ |
| `ctx` | 层级 2+（可解析用量的日志） | ✅ | ❌ | ❌ |

没有数据的判据在唤醒行里直接省略，不印成零；选 HQ agent 的操作者也该事先知道自己的 agent 喂得出哪几条判据。
HQ 跑在 gemini/cursor 上时，对抗会话老化的三道防线只剩一道，这正是平滑降级该有的样子。
印成零比省略更糟：快满的会话印成 `ctx 0%`，读者只会觉得还有余量，反而在唤醒要求轮换的那一刻更不去轮换。

同一形状的相关点：`rotateInput` 发的是 agent 自己的「开新对话」命令（codex 用 `/new`，其余用 `/clear`）。
不认这条命令的 agent 就是不轮换，传感器会再敲，无害，但 opencode 上未经验证。

**平滑降级是硬规则（不变量 I2）：**缺一项能力绝不能把 agent 拖到再下一层以下。没有 transcript 解析器 → digest
退回屏幕推导的形式。hook 事件一直不来 → 校验退回两帧屏读，和无 hook 的 agent 完全一样。没有证据不等于失败：
永远别写在 agent 不提供的能力上硬失败的代码。

---

## 2. 身份只住在一个地方：注册表

历史上每个子系统各留一份按 agent 键的表，键不一致、成员各漂各的。现在 `internal/agents` 里每个 agent 一份 `Manifest`，
每个子系统从它派生自己的表。给一个 agent 加身份，就是写一份 manifest。

```go
// internal/agents/registry.go
type Manifest struct {
    Key, Label      string   // "claude", "Claude Code"
    Aliases         []string // alternate keys that resolve here (cursor-agent → cursor)
    Detect          []string // radar process-subtree commands; "" ⇒ no radar profile
    IdleGlyph, Icon string   // the idle marker its TUI paints; a vendor app path (no bundled trademark)
    Resume          []string // resume argv; nil ⇒ not resumable by session id
    Resource        string   // resource-attribution name; "" ⇒ not attributed
    HookDisplay     bool     // registered in the hook-time known-agent gate
    Hooked          bool     // events feed the receipt/ready stream (Tier 1)
    Content         string   // transcript-parser key (Tier 2); "" ⇒ none
    Headless        string   // headless one-shot key; "" ⇒ none
    Semantics       bool     // has a DEDICATED classifier event table (else the generic one)
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
| transcript / headless 键 | `agents.Content/HeadlessKeys()` | `internal/driver` |

注册表的数据由 `internal/agents/registry_test.go` 里的 golden 测试钉住（从旧表逐字抄来），
再加每个子系统的迁移守卫测试。

### 留在各自领域的东西（以及为什么）

三样东西属于行为，留在自己的包里，按注册表的 agent 键索引；把领域枚举搬进纯数据的注册表是过度抽象：

- 事件语义，原生事件 → gtmux 语义的映射表：`internal/hook/classify.go`
  （`agentEventSemantics`，否则用通用表）。
- 提示符 / 就绪签名（启动横幅、提示符/选择器字形）：`internal/prompt`。
- hook 安装规格，gtmux 写的那个文件/插件：`internal/app/agent_hooks.go`。

一致性检查（`scripts/check-design.sh`）把这三样绑回注册表，谁也漏不掉：每个 `Hooked` 的 agent 必须有安装规格，等等。

---

## 3. 一步一步来

### 第 0 步：manifest（必做）

在 `internal/agents/registry.go` 的 `manifests` 里加一条。填 `Key`、`Label`、`Detect`（层级 0）。
能按 id 重启会话的加 `Resume`。跑 `go test ./internal/agents/`，golden 测试会告诉你有没有碰坏已有的 agent。
这就是层级 0：雷达认得它，focus/send/resume 可用。

### 第 1 步：hook 安装器（层级 1）

设 `Hooked: true` 和 `HookDisplay: true`。然后接一份安装规格让 agent 发事件。有三种扩展模型，先看 agent 支持哪种：

- 命令 hook 模型（Claude、Codex、Cursor、Gemini、Copilot、Kiro）：agent 读一份 JSON/TOML 配置，在生命周期事件上跑 shell 命令。
  在 `internal/app/agent_hooks.go` 加一条 `agentInstaller`，把每个原生事件映射到 gtmux 的记号：
  `beforeSubmitPrompt → UserPromptSubmit`、`afterAgentResponse → Stop`、审批事件 → `PermissionRequest`、会话开始/结束。
  选一个或新加一个 `format`。
- 插件模型（opencode）：agent 没有命令 hook 文件，只有 JS/TS 插件。安装器写一个小插件，订阅 agent 的事件并
  shell 出 `gtmux hook --agent <key> <event>`。入口同样是 `install-hooks --agent <key>`；插件是 `dedicated` 产物，卸载时干净移除。
- 托管块模型（Kimi Code）：agent 的 hook 住在一份 gtmux 不拥有的配置文件里，即放着用户 provider 和密钥的那份
  `~/.kimi-code/config.toml` 里的 `[[hooks]]` 条目。上面两种模型都不合适：没有一整份可以写的文件，而为了改四行去重新序列化
  别人手写的 TOML 也不划算（gtmux 没有 TOML 库，也不该为此引一个）。所以 `internal/app/kimi_hooks.go` 在哨兵注释之间追加一块，
  和编辑 shell rc 文件一个做法；卸载只删哨兵之间的内容，别的一概不读。追加永远是合法 TOML，因为一个表头结束前一个表的作用域，
  所以这块不可能落进别人的表里。凡是 agent 的 hook 和它的用户配置同住一个文件，就用这种。

把 agent 的原生事件映射到 gtmux 的：`UserPromptSubmit`、`Stop`、`PermissionRequest`（真正面向用户的审批 → `waiting`）、
`PostToolUse`/resolve（清掉 `waiting`）、`SessionStart`、`SessionEnd`、`PreCompact`/`PostCompact`。如果 agent 的审批信号是一个
独立于工具前事件的事件，给它一张专用语义表（`agentEventSemantics`，`Semantics: true`），让工具前事件留作遥测；
如果它唯一的信号就是工具前事件，通用表的 `semToolStartMaybeApproval` 会替你把有副作用的工具升级成审批。

验证身份是从进程子树解析出来的（见坑清单，前台命令不能当来源）。装好，跑一个真会话，确认 `waiting`/`done` 和带回执校验的
`gtmux send`（`judged_by: driver`）。

### 第 2 步：transcript 解析器（层级 2）

加 `internal/transcript/<agent>.go`，把 agent 的会话日志读成 `[]Turn`，设 manifest 的 `Content` 键（仅此一步就自动接上
`driver.Content`，见 `agents.ContentKeys()`），再加 `resolveLog` + `normalizeAgent` 两处 case。现在 digest 能渲染 `goal`/`last`/`ask` 了。
pane→会话的映射是白送的：hook 用会话 id 写一条 `resume` 记录，`sessionRef` 读它，所以一个 hook 能拿到会话 id 的可 resume agent
不需要额外接线。

**如果 agent 在磁盘上不留可读的 transcript**（opencode 1.18.x 只持久化 `session_diff`，不存消息），gtmux 自己留一份：
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
- [ ] 装了 ≠ 信了。有些 agent 对新注册的 hook 要过一次用户确认才会触发，Codex ≥ ~0.146 会显示
  "New hook - review required — press t to trust"。用户没信任之前 gtmux 什么都看不到（没 waiting/done、没 digest），
  哪怕配置完全正确。安装器必须说出来；排查「静默 hook」之前先看看 agent 是不是只是在等信任。
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
  所以 `radar.IconFor` 会把内置 PNG 落盘到 `~/.local/share/gtmux/agent-icons/<key>.png`，并在 profile 的 `Icon` 为空时返回那个路径；
  没有这个提示，图标明明发了，手机上还是显示单字标。（opencode 显示 "OC"、Codex 的非 tmux 行显示 "Cx"，直到修掉，就是这么来的。）
  因为提示是真路径而不是不透明记号，菜单栏 app（它把提示当文件打开来解析）也能拿到内置图标，app 侧零改动。
  它的 `~/.config/gtmux/icons/<slug>.png` 投放仍是手动覆盖。
- [ ] 审批事件 vs 工具前事件。agent 有独立审批事件的，把工具前事件留作遥测（专用表）。否则要么每个工具都标「等你」，
  要么真审批被丢掉（Kiro 的小写事件必须显式注册）。
- [ ] 只读工具永远不标「等你」。`classify.go` 里的 `sideEffectingTools` 是白名单；只读工具（Read/Grep/Glob/…）别放进去。
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

### Go 注册表（还）没喂到的两块屏

菜单栏和手机 app 各自保留自己的 agent 标记/图标表，加 agent 时两个都要更新：

- 菜单栏：`macapp/Sources/GtmuxBar/Components.swift`（`agentMark`、`AgentIcons`）。
- 手机：`mobileapp/src/ui/agentMark.ts`。

（以后可能会改成从 `agents --json` 喂；在那之前是手工维护。）

---

## 5. 收工清单

- [ ] manifest 已加；`go test ./internal/agents/` 绿（golden 测试未受影响）。
- [ ] 目标层级真的接上了（层级 1 有安装器，层级 2 有解析器）。
- [ ] 在真 pane 上经进程子树验过身份。
- [ ] `install-hooks --agent <key>` 写入集成；卸载只删 gtmux 写的，用户配置原样保留。
- [ ] 一个活会话驱动了 `waiting`/`done` 和带回执校验的 `gtmux send`（`judged_by: driver`），层级 1 要求。
- [ ] 菜单栏 + 手机的标记/图标已更新。
- [ ] `make check` + `scripts/check-design.sh` 绿。
- [ ] 面向用户的 agent 在 CLAUDE.md / `docs/cli.md` 里有提及；行为变了就更新 spec。
