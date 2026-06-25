# 研究笔记:从 cmux 借鉴什么 + 多路复用器(mux)适配

> 2026-06 调研。基于 cmux 源码(`/Users/ccy/meituan/manaflow-ai/cmux`,只读)。
> 两个目标:① 找出值得反哺 gtmux 的高价值点;② 评估以**扩展/适配器**方式
> 顺带支持其他主流 "mux"(让用户群更大),gtmux 仍以原生 tmux 为主。

## TL;DR

借 cmux 的**智能**,别借它的**管道**。枚举 / 读屏 / 发送 / 聚焦这些原语,
gtmux 的 tmux 底座本来就免费给了,而这恰恰是 cmux 通过 socket **反而更弱**的地方
(见下文 §2 的两个缺口)。真正值钱的是 cmux 在"判断 agent 状态、起名、按 agent
恢复"上的做法 —— 那是 gtmux 现在最薄的环节。

---

## 1. 高价值借鉴点(按 ROI 排序)

### ⭐ A. 精准的"在等你"判定:`FeedEventClassifier`(最高价值)
cmux 不靠扒标题/计时,而是用一张**类型化的 `(来源, 事件) → 语义` 注册表**直接从
hook 负载判定 agent 是否**正卡在你身上**,并区分 **权限请求 / 计划批准(ExitPlanMode)
/ 提问(AskUserQuestion)**。关键正确性细节:
- 对**有专门审批事件**的 agent(claude/codex/hermes),把"工具开始"一律当遥测,
  绝不误判成审批(他们的 issue #4985)。
- 对**没有专门审批事件**的 agent(gemini/kiro),只有当工具是**有副作用的**
  (Bash/Write/Edit/apply_patch/shell/… 见 `sideEffectingTools`)才升级成 actionable。
- 证据:`CLI/FeedEventClassifier.swift`(~340 行纯逻辑,无 app 依赖,**几乎可逐行移植到 Go**)。

gtmux 现在用"pane 标题 braille/✳ + 事件时序"近似这件事;换成这套确定性分类器,
等待检测会从"猜"变成"事实"。**工作量:中(1–2 天移植 + 副作用工具集)。**

### ⭐ B. LLM 自动起名:pane 名 = 2–5 词的任务摘要(最高 ROI)
cmux 的 tab 名不是 pane 标题,而是**用小模型概括 agent 对话**得到的短标题。
- 触发:Stop/turn-end hook,**节流**(≥12 行转写、较上次≥6 行增长、≥180s 间隔)。
- 解析多种转写格式:Claude Code JSONL、Codex rollout、Grok `chat_history.jsonl`、
  opencode/pi/omp 的 hook 负载缓存。
- 概括:`claude -p --model haiku --tools "" --no-session-persistence`(或 `codex exec`
  只读沙箱),提示词要求"2–5 词、与对话同语言、只输出标题",**调用前清掉 `CMUX_*`/
  `CLAUDE_CODE_*` 防止递归触发 hook**。
- 证据:`CLI/CMUXCLI+AutoNaming.swift`、`+AutoNamingDispatch.swift`。

gtmux 现在的 task 列就是 pane 标题;接上这套后任务描述变得有意义、跟随语言。
纯逻辑,不需要 socket。**工作量:中。**

### ⭐ C. 声明式多 agent hook 注册表(支持面从 1 → 17)
gtmux 现在只接 Claude Code。cmux 用一张 `AgentHookDef` 数组覆盖 **17 个 agent**
(codex/gemini/cursor/opencode/grok/amp/kiro/hermes/copilot/…),加一个新 agent ≈
追加 ~15 行数据。生成的 hook 命令很硬:自动找 CLI、用 `$CMUX_SURFACE_ID` 把关、
每 agent 一个禁用开关、`|| echo '{}'` 兜底(hook 永远弄不坏 agent)。
- 证据:`CLI/CMUXCLI+AgentHookDefinitions.swift`(`agentDefs` 156–371)。

借这套结构,gtmux 的"多 agent 支持"变成填表。**工作量:中**(各家配置格式
flat/nested/yaml/plugin 是大头,先做 `.flat`+`.nested`)。

### D. 按 agent 恢复**会话**(不是恢复一个死 pane)
tmux-resurrect 只恢复 shell,不恢复 agent 对话。cmux 记下每个 agent 的
启动方式 + sessionId,重建 `<agent> --resume <id>`:claude `--resume`、codex
`resume <id>`、amp `threads continue`、opencode `--session`…(共 18 家)。
- 证据:`Packages/macOS/CMUXAgentLaunch/.../AgentResumeArgv.swift`(`builtInKind`)。
- 配套:快照里存 `wasAgentRunning` + (ANSI 安全截断的)scrollback,用来决定**哪些**
  pane 自动恢复;`autoResume` 逐 pane 选择开 + 签名防篡改的审批记录。

这是相对 tmux-resurrect 最大的韧性升级。**工作量:中**(纯查表,难点是在 spawn
包装层捕获每家的 sessionId)。

### E. hook 会话存储的两个防坑设计
gtmux 一旦把 hook 做深,一定会踩这两个坑,cmux 已经解了:
1. **同一 pane 里旧会话的迟到 hook 写错状态** → 用 `activeSessionsBySurface` 的
   "当前会话边界"挡掉(issue #5908)。
2. **子 agent 的 Stop 把还在干活的主 agent 误标 idle** → 用 `activePromptTurnIds`
   嵌套 turn 栈(`recordPromptSubmit`/`recordPromptStop`)。
- 证据:`cmux.swift` 的 `ClaudeHookSessionStore`(flock 加锁、7 天清理)。**工作量:中。**

### F. 移动 / 远程层可借的零碎
- **静默"消除/角标同步"推送**:priority-5 `content-available` + `apns-collapse-id`
  合并 + 角标用绝对值 + **扇出到所有设备**(第二台离线手机也能清)。gtmux 已有 APNs
  relay,补这套消息纪律是干净的 UX 升级。`Sources/Cloud/PhonePushClient.swift`、
  `web/services/apns/sender.ts`。**工作量:低。**
- **`hideContent` + "仅离开时推送"**:终端内容永不出 Mac;在用 Mac 时不推。**低。**
- **凭证只走加密信道的路由策略**:token 只允许走 Tailscale/loopback,明文 LAN 拒发并
  告警(`MobileShellRouteAuthPolicy.swift`)。对 gtmux 的 tunnel/LAN 故事直接适用。**低。**
- **配对码本身不是凭证**(进阶):cmux 的配对二维码只含 `host:port` + 不透明 user id,
  真正的鉴权是手机本就持有的账号 token —— 比 gtmux 的 `{url,token}`(截图即泄露)更安全,
  且配对码可不过期。但需要一个身份层;轻量版:让配对 token 只是"换取短时凭证的凭据"。
  **工作量:中–高。**
- **结构化 render-grid 流式**(整屏快照 + 行级 diff + 样式表 + scrollback + `state_seq`
  重同步)比 gtmux 现在的 capture-pane-over-SSE 更省流、断线重连更稳。但这是个协议 +
  客户端 emulator,**工作量:高**;现状够用,列为优化项。

---

## 2. 多 mux 适配:架构与现实

### gtmux 检测其实分两半
- **(a) 枚举 + 读标题/命令**:`tmux list-panes -F …` —— 这部分**和 mux 强耦合**,
  每种 mux 要一个适配器。
- **(b) `⏸ 等待` / `✓ latest` 的 hook**:按**每个 pane 的 id** 记状态。tmux 给 `$TMUX_PANE`,
  cmux 给 `$CMUX_SURFACE_ID`,本质一样。**只要把"pane-id 来源"抽象一下,这半套几乎天生 mux 无关。**

所以"支持其他 mux"= 定义一个 `Multiplexer` 接口(列表 / 读屏 / 发送 / 聚焦 / 新建)
+ 把 hook 的 pane-id 来源参数化。tmux 是参考实现 #1。

### cmux 作为适配器:能做,但雷达会更弱
cmux 的 socket 暴露够建一个**只读适配器**:`surface.list`(枚举 + 标题 + tty)、
`surface.read_text`(读屏,**纯文本无 ANSI**)、`surface.send_text/send_key`(发送)、
`surface.focus`(聚焦),每 pane 有 `CMUX_SURFACE_ID`,鉴权走 `cmuxOnly`(进程血缘)或 password。

**但有两个真缺口(对"雷达"最关键)**:
1. **每 pane 的实时状态(busy/idle/exited)socket 查不到** —— 它在内部用
   `report_shell_state` 推进 cmux,但任何读 RPC 都不回传。tmux 直接给
   `#{pane_current_command}` / `pane_dead`。**这是雷达最大的缺口。**
2. **实时前台命令 + PID 不在 surface 行上** —— 要另外调 `system.top` 再按 tty join,
   比 tmux 一条 `list-panes -F` 重且有竞态。

> 含义:即便做了 cmux 适配器,雷达的"状态"也得靠 **gtmux 自己往 cmux pane 里装 hook**
> (用 `CMUX_SURFACE_ID` keying)来补,而不是靠 cmux 的 socket。这反而印证了
> "借智能(hook 判定),别依赖它的管道"。

### 其他值得纳入接口设计的 mux
- **Zellij**(Rust,最流行的非 tmux mux):有 CLI/action + plugin,可作适配器 #3。
- **WezTerm 多路复用**:`wezterm cli list`(JSON)能枚举 + `send-text`,适配器友好。
- **tmate / byobu**:本质是 tmux,现状已可用。
- **screen**:能枚举但状态信息贫乏,优先级低。

设计 `Multiplexer` 接口时按 tmux + cmux + Zellij + WezTerm 这四个对齐能力面,
缺口(如 cmux 的实时状态)用"gtmux 自带 hook"统一补齐。

---

## 3. 建议的推进顺序

1. **先做纯逻辑借鉴(无关 mux,马上提升体验)**:A 等待分类器 → B 自动起名 →
   C 多 agent hook 注册表。这三样直接把 gtmux 在 tmux 上的体验拉高一档,且为多 mux 打基础
   (hook 判定是 mux 无关的)。
2. **再抽 `Multiplexer` 适配接口** + 把 hook 的 pane-id 来源参数化;tmux 落为参考实现。
3. **最后按需加适配器**(WezTerm/Zellij 比 cmux 更适合做"雷达适配器",因为它们能查实时状态;
   cmux 适配器要配合 gtmux 自带 hook)。
4. 移动/推送层的 F 系列零碎随手做(低成本增量)。

> 战略提醒:cmux 自己已经有 sidebar 雷达 + 通知 + 等待检测 + 会话恢复 + **配对手机的 iOS app**。
> 对**已经在用 cmux** 的人,gtmux 增量有限。多 mux 适配的意义在于**覆盖 WezTerm/Zellij/远程
> tmux** 这些 cmux 够不到的场景,而不是去 cmux 的地盘抢用户。
