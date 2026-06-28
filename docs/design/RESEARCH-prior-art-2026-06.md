# gtmux 同类开源项目调研 (2026-06-28)

> 目的：在继续做 chat-history / 远程连接指示 / 双隧道防护等功能之前，先看看「别人怎么做的」，避免重复造轮子，并提炼值得借鉴的设计。
> 方法：5 个并行研究子代理，分别覆盖五个最接近 gtmux 的项目集群，全部基于实时抓取的 GitHub README / 文档 / 源码（2026-06）。star 数为抓取时近似值。

## TL;DR — 一页结论

1. **gtmux 的护城河成立且独一份**：没有任何一个竞品同时具备「tmux 原生 + 多 agent 雷达 + 语义化 waiting/working/idle + 原生菜单栏 + 手机推送/远程 + 浏览器镜像」。所有竞品要么是单 session 的 CLI wrapper（Happy/Omnara/VibeTunnel），要么是纯桌面编排器（Conductor/Crystal），要么是无 UI 的中间件（AgentAPI）。**保持「雷达」定位为核心。**
2. **「agent 是不是在等我」是全行业未解的中心难题，而 gtmux 已经领先**。绝大多数工具只能检测「忙不忙」（buffer diff / CPU 采样），分不清「做完了」和「卡在等你」。只有用结构化信号（hooks / OSC 转义序列）的少数项目能做到——正是 gtmux 的路线。
3. **chat-history 该做成结构化的 turns，不是终端流**；整个行业都在往「结构化对话」收敛。解析算法是已解决问题，我已拿到可直接移植的 Claude / Codex schema 清单（见 §6），**自己写 parser、移植算法，不引依赖**（core 必须 cgo-free + Go）。
4. **不要做成 hook-only**：cmux 自己的 bug（#3749「codex agents go silent when hooks miss」）证明纯 hook 状态会「卡死」行。gtmux 的「确定性分类器兜底 + hook 提精度」是对的，继续保持。
5. **需要你拍板的战略问题已单独记到 `DECISIONS-FOR-CCY.md`**（端到端加密 / QR 配对 / 是否从雷达走向编排器 / 输入预测回显）。

---

## 1. 移动端 / 远程 Claude Code 客户端

| 项目 | star | 形态 | 传输 | 渲染 |
|---|---|---|---|---|
| **Happy** (`slopus/happy`) | ~22k | iOS/Android/web，最接近的竞品 | **零知识云中继**（WebSocket sync，服务端只存加密 blob） | 结构化原生 chat（非终端） |
| Omnara (`omnara-ai/omnara`) | ~2.6k | 手机/web，**已归档**，转向 Agent SDK | 云中继 + SDK(REST/MCP) | 结构化步骤流 |
| AgentAPI (`coder/agentapi`) | ~1.4k | **Go**，HTTP API 中间件（无 UI） | localhost HTTP + SSE | 内存终端模拟器 → diff 成消息 |
| claude-code-webui | ~1.1k | 浏览器 chat，**已归档** | LAN，**无鉴权** | 消费 `stream-json` → 气泡 |
| VibeTunnel (`amantus-ai/vibetunnel`) | ~4.6k | **Swift 菜单栏 + TS server**，架构最像 | BYO 隧道(Tailscale/ngrok/CF/局域网) | PTY + xterm.js(DOM) |
| Claudia/opcode | ~22k | Tauri 桌面，无远程 | — | 读 `~/.claude/projects/` |

**关键借鉴：**
- **Happy = 安全标杆**：QR 配对(临时 TweetNaCl 密钥对) + 每 session/每机器随机 AES-256 DEK + 零知识中继 +「连推送内容都加密，我们看不到」。这正面回应了 gtmux 自己写在 CLAUDE.md 的风险（token 泄露 = 在 Mac 上执行命令，当密码看）。
- **AgentAPI = chat-history 的蓝图**：`snapshot → 输入 → diff → 剥离 TUI 残渣 → 干净消息`，`status: stable|running` ≈ gtmux 的 idle/working。而且是 Go，和 gtmux core 同语言，可直接借思路。
- **Omnara 的教训**：它归档时明确说「wrapper 套 CLI 太脆，转用 Agent SDK」。gtmux 的 **hook + tmux 扫描** 比 wrapper 健壮，是真实优势。

---

## 2. 多 agent 编排 / 并行管理器

| 项目 | star | 隔离 | 状态检测 |
|---|---|---|---|
| **claude-squad** (`smtg-ai/claude-squad`) | ~7.9k | **tmux + git worktree**（和 gtmux 同底座！） | **SHA256 哈希 tmux buffer** 判断变没变；**无 needs-input 检测** |
| cmux (`manaflow-ai/cmux`) | ~23k | 原生 pane（libghostty/Swift，同 DNA） | **OSC 9/99/777 转义序列** + `cmux notify` 接 hook；蓝环 + 未读角标 + 跳转最新 |
| vibe-kanban (`BloopAI/...`) | ~27k | worktree | 看板列状态（**正在日落**） |
| Conductor / Crystal / uzi / ccmanager / container-use / sculptor | 各异 | worktree / 容器 | Crystal 少数把「waiting for input」做成一等状态 |

**关键发现：**
- **worktree + 每 agent 一个终端是事实标准底座**，claude-squad / uzi 用的就是 gtmux 信任的 tmux+worktree。gtmux 离「能参与这套工作流」只差一步。
- **真正的瓶颈是 review/分发，不是生成**（vibe-kanban 全部论点）。可在 gtmux 的行里补充：**diff 统计、dev-server 端口/URL（uzi `ADDR`）、PR 状态/分支（cmux 侧栏）、按项目分组（ccmanager）、idle agent 的「待 review」态**——这是「radar++」，价值高、成本低。
- **编排器赛道在洗牌**：vibe-kanban 日落、Crystal 被弃用改名 Nimbalyst，就在调研窗口期内发生。正面做「又一个 worktree launcher」是最弱的位置。
- **状态检测的两个免 hook 兜底技巧**：claude-squad 的 **buffer 内容哈希**、cmux 的 **OSC 9/99/777 捕获**，可作为 hook 没装时的补充信号。

---

## 3. 终端流（浏览器/手机）+ tmux 仪表盘

| 项目 | star | 捕获 | 传输 | 渲染 |
|---|---|---|---|---|
| VibeTunnel | ~4.6k | PTY + 命名管道 + asciinema | **SSE**（刻意不用 WS：自动重连、穿代理好）+ 二进制模式 | xterm.js(DOM) |
| ttyd | ~12k | PTY/libuv | WebSocket（**5s ping**保活） | xterm.js(WebGL2) |
| sshx (`ekzhang/sshx`) | ~7.5k | PTY | **端到端加密** WS（中继看不到明文）+ 边缘节点 | xterm.js(WebGL) + **Mosh 式预测本地回显** |
| sesh / tmux-resurrect+continuum / gitmux / tmux-notify | — | — | — | tmux 原生集成 |

**关键借鉴（对 gtmux 的终端镜像 + 输入路径）：**
- **浏览器单域名 6 连接上限**（VibeTunnel 实测「第 7 个终端静默失败」）：gtmux 若对一个隧道开多个轮询/流，必须**多路复用成一条连接、按 pane id 打标**。
- **SSE 自带重连**（`EventSource` + `Last-Event-ID` 服务端回放）= 手机睡眠后重连的廉价解法。配合 gtmux 既定铁律：**离线不清屏、缓存帧置灰**。
- **Mosh 式预测本地回显**（sshx）：手机隔着隧道打字感觉「坏了」，本地先回显能极大改善 `send-keys` 体验。
- **xterm.js 移动端触摸/选择/预测键盘在所有渲染器下都坏**——gtmux 的「只读为主」反而是优势：把 helper textarea 设 `disabled`/`type=password` 禁掉软键盘和预测文本，用自己的 Moshi composer 显式发送。**用 DOM 渲染**（和 gtmux 既有结论一致，xterm v6 也砍了 canvas）。
- gtmux 用 `capture-pane` 快照（不拥有 PTY）对「多 agent 雷达」是对的选择；代价是非增量流——前台活跃 pane 可考虑 `capture-pane` **diff 流**而非整屏重拍。
- **tmux 状态栏**：gtmux 目前不往 tmux 自己的 status line 渲染。gitmux 证明一个 `#(gtmux status …)` 段成本很低，能让雷达在 tmux 里也可见。

---

## 4. 通知 / 状态监控（hook 为主）+ 菜单栏监视器

**Claude hook 生态已形成约定**：`Notification` = 需要你 / `Stop` = 完成。值得学的进阶：
- **claude-notifications-go** (777genius)：用 **Stop 前跑过哪些工具** 把完成分成「真干活了(Write/Edit/Bash)」vs「只读回答了(Read/Grep)」；`suppressQuestionAfterTaskCompleteSeconds:12` 去重。
- **claude-status-bar** (m1ckc3s, Swift 菜单栏)：hook 驱动，**工作中动画 + 计时器 "1m1s"**、等权限黄点、工具标签(Editing/Reading)、**等权限的 session 排在 thinking 前面**——和 gtmux 排序一致。**完全依赖 hook、无兜底**。
- **tap-to-tmux** (flavio87)：唯一明确解决「没有 Claude 式 hook 的 agent」——对 pane PID 做 **CPU tick 采样**（忙=working，静=idle/waiting）；**冷却期在你与 session 交互时重置**。
- **code-notify** (mylee04)：**每 agent 安装注册表**——Claude→`settings.json`、**Codex→`config.toml` 的 `notify`**、Gemini→`settings.json`；并诚实标注「Codex 只有完成事件，没有 idle/permission」。
- **Codex 原生 `notify`**：两个事件 `agent-turn-complete`（轮次完成，待输入）/ `approval-requested`（要权限）；**没有「工作中」事件**——working 态必须靠进程/CPU 采样。建议只给父 agent 配 notify。
- **Gemini hooks**：Before/After Tool/Model 生命周期较全（可推 working），但 needs-input 要从 `AfterAgent` + `Notification` 推。
- **cmux 的反面教材** (#3749)：「sidebar agent status is hook-only with no fallback — codex agents go silent」——铁证：**永远不要 hook-only**。
- **CCSeva / ccstatusline 等**是 quota/用量菜单栏（和 gtmux 的工作流状态是不同赛道），但「只在变差时通知 + 冷却」「增量扫 jsonl + 去重」可借。
- **noti/ntfy/zsh-notify**：「终端已聚焦就不通知」「>10s 才通知」——gtmux 的 `IsViewing` 已是同思路。

**进阶能力**：claude-ntfy-hook 证明**手机上 Allow/Deny 回到 agent** 可行（通知动作 → 回调 → `PreToolUse` 返回 exit code 2 拦截 / 0 放行）。gtmux 的手机快捷回复可以不止 `send-keys`，还能答**权限提示**。

---

## 5. 跨集群「gtmux 怎么不同 / 怎么领先」

- **确定性分类器 + hook（非 hook-only）**：精度靠 hook，存活靠分类器兜底。
- **三屏同语言**：菜单栏 + RN 手机 + 浏览器，共享 色+形+字形。
- **原生 AppKit 菜单栏**（竞品多为 Electron；唯一另一个原生的是 claude-status-bar）。
- **自带 APNs 中继 + 双向 `POST /api/send`**。
- **真三态 waiting/working/idle**（多数竞品只有两态，因为源 agent——尤其 Codex——不发「工作中」事件）。

---

## 6. 给 chat-history parser 的 schema 清单（直接喂给功能 A）

> 共同结构：两边都是**扁平事件流，按 id 配对工具调用重建 turns**，都不嵌套。统一模型 `Turn{prompt, steps[], response}` + 每 agent 一个 adapter。可移植参考：`daaain/claude-code-log`(models.py/converter.py)、`PixelPaw-Labs/codex-trace`(parser/turn.rs, MIT, Rust→Go 直译)、MilkoorY 的 Codex 逆向文（dev.to）。

### Claude Code（`~/.claude/projects/<cwd-slug>/<sessionId>.jsonl`，逐行）
- 顶层 `type`：`user` / `assistant` / `system` / `summary` / **还有较新的** `ai-title` / `queue-operation` / `attachment`（别只认前三个；未知 type **跳过不崩**）。
- 字段：`uuid` `parentUuid` `isSidechain` `sessionId` `timestamp` `cwd` `gitBranch` `version` `agentId` `spawnedAgentId` `requestId` `isMeta`。
- `message.content` 是 **`string | block[]`**（用户 prompt 常是裸字符串）。block：`text` / `thinking{thinking,signature}` / `tool_use{id,name,input}` / `tool_result{tool_use_id,content,is_error}` / `image{source}`。
- ✅ **最终回复 = `stop_reason=="end_turn"` 的 assistant**（`tool_use` 的 stop_reason 是中途）。
- ✅ 工具配对：`tool_use.id == tool_result.tool_use_id`；也读 user 条目上**并列的 `toolUseResult` 字段**。
- ✅ 用 `parentUuid` 串线；无 `uuid` 的（summary/ai-title/queue-op）绕过串线、单独挂。
- ✅ `isSidechain:true` = 子 agent（Task），按 `agentId` 折叠或单列（仿 `{trunk}#agent-{agentId}` 重键）。
- ✅ `isMeta:true`(斜杠命令) 过滤/折叠；compaction 续接（system 带 `compact_*`）→ 别假设首条就是首个用户轮。
- ✅ 多快照去重：按 `message.id`+`requestId`，**取最后/最大**（ccusage 的 first-wins 会少算）。
- ✅ base64 `image` 块惰性截断。
- ✅ cwd-slug：`/Users/x/code/app` → `-Users-x-code-app`；显示名从条目里的 `cwd` 取，不要从 slug 反推。**实操更稳：直接按 sessionId glob 文件，绕开 slug 编码。**

### Codex（`~/.codex/sessions/YYYY/MM/DD/rollout-<ts>-<sessionId>.jsonl`，每行 `{timestamp,type,payload}`）
- 顶层 `type`：`session_meta` `turn_context` `response_item` `event_msg` `session_archived/unarchived`。
- ✅ **内容真源 = `response_item`**；`event_msg` 是「展示用/生命周期」（但 `user_message`/`agent_message` 给的是干净文本，适合紧凑列表）。
- `response_item.payload.type`：`message`(role assistant|user|**developer**；assistant 文本是 `content[].type=="output_text"`) / `function_call{name,arguments(JSON 字符串,要再 parse),call_id}` / `function_call_output{call_id,output}` / `reasoning` / `structured_output` / `mcp_tool_call`。
- ✅ **按 `call_id` 配对**（缓冲 pending，到达即配，中间可有任意行；不嵌套；`status:in_progress` 直到 output 到）。
- ✅ **过滤 `role=="developer"`**（注入的系统上下文）；首条空 diff-comment 用户消息忽略（#24077）。
- ✅ **最终答案**：`event_msg/task_complete.last_agent_message` → `agent_message` 的 `phase:"final_answer"` → 最后一个 `output_text`/`structured_output`。
- ✅ 轮边界：新格式 `task_started(turn_id)…task_complete`；旧格式按 `user_message` 切（合成 id）。**容忍 2025/08 旧元数据格式**。
- ✅ **`token_count` 到处都是、无规律 → turn 结构里直接忽略**（只对统计有用，且是累计值要做差）。
- ✅ 超大行 / 内联 base64 图片要能扛（#22603、ccusage #952 静默丢整天）。
- ⚠️ MilkoorY 铁律：**信磁盘上的真实数据，别信 Codex 源码里的 struct 注释**（`protocol.rs` 和落盘格式不一致，照源码建模会「一个 session 只解析出 1 个事件」）。

**复用 vs 自造**：没有现成库做「Claude+Codex 统一 turn 解析」。ccusage 是唯一同时吃两边的成熟代码，但是 TS、为 token 计费写的（去重逻辑甚至不适合 turn）。**结论：自己写 Go parser，移植上面三个参考的算法与数据模型。**

---

## 7. 来源
见各集群正文内联 URL。最关键的几个：
- Happy 安全模型 https://happy.engineering/docs/security/
- AgentAPI https://github.com/coder/agentapi · https://hugodutka.com/posts/agentapi/
- VibeTunnel 架构 https://steipete.me/posts/2025/vibetunnel-turn-any-browser-into-your-mac-terminal
- Claude 逆向 schema https://github.com/daaain/claude-code-log（models.py/converter.py）
- Codex 逆向 https://dev.to/milkoor/reverse-engineering-codex-cli-rollout-traces-3b9b · https://github.com/PixelPaw-Labs/codex-trace
- cmux hook-only 翻车 https://github.com/manaflow-ai/cmux/issues/3749
- tap-to-tmux CPU 采样 https://github.com/flavio87/tap-to-tmux
