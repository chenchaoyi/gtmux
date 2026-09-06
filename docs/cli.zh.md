# CLI 与命令

[English](cli.md) · **中文**

| 命令 | 做什么 |
| --- | --- |
| `agents [--watch\|--json]` | 你所有 pane 里的 coding agent：谁在等你 / 在跑 / 空闲，在哪儿，跳过去用哪个 pane id |
| `panes [--json]` | **每一个** tmux pane（不只是 agent），按 agent/普通 分档，是 pane 浏览器背后的全集 |
| `overview [--popup]` | session / window / pane 汇总；`--popup` 适配 tmux 弹窗尺寸 |
| `restore [--pick\|--one\|<name>\|--dry-run\|--plan[ --json]] [--resume-agents=auto\|type\|off]` | 每个 session 一个终端标签页并全部接回；可选地把记下来的 agent 对话也重新拉起；`--plan` 先看会恢复出什么 |
| `focus <name\|pane-id\|--last>` | 跳到某个 session 的标签页；给 pane id（`%N`）就落到那个 pane；`--last` 是最近刚跑完的那个 agent |
| `new [name]` | 新建一个 tmux session，并开一个终端标签页 |
| `adopt <session_id>…` | 把感知到的非 tmux（native）agent 会话转进 tmux |
| `doctor [--fix [--yes]]` | 按主题分组的体检；在 TTY 上会当场问你要不要修可改进的项；`--fix` 是一站式配置（hook、set-titles、重启恢复、菜单栏 app） |
| `install [hooks\|app\|all]` | 装 gtmux 需要的东西；不给目标就问你。`install hooks --agent codex\|cursor\|gemini\|copilot\|kiro\|opencode` 接入另一个 agent |
| `uninstall [hooks\|app\|all]` | 反过来卸掉；不给目标就问你（两者后果差很远） |
| `serve [--port N]` | 给手机 app / 网页镜像用的只读 HTTP+SSE 雷达（放在 VPN 或隧道后面） |
| `tunnel [--backend cloudflare\|self] [--quick] [--service] [--redeem <码>]` | 把雷达开到任意网络 —— Standard（Cloudflare）或 Direct（自托管 / 付费），见 [phone.zh.md](phone.zh.md) |
| `pair [list\|revoke <id>]` | 接入**你自己的**设备（全权）：一个一次性配对码，手机扫、浏览器开、或者一行 `gtmux attach` |
| `share [new\|set\|link\|on\|off\|revoke <id>\|status]` | 给协作者的受限、可吊销链接 —— 每条链接单独的可见/可输入白名单（见下） |
| `attach <地址\|配对链接\|分享链接> [%pane]` | 把远端 tmux pane 的 PTY 经 serve 的 WebSocket 接到你本地终端（owner 或访客） |
| `devices [revoke <id>\|--push\|--forget-push <id\|orphans\|all>]` | 已配对设备清单（`pair list`/`pair revoke` 的别名）；`--push` 查看、`--forget-push` 清理推送 token |
| `app`（别名 `menubar`） | 启动菜单栏 app（`Gtmux.app`） |
| `update [--check\|--cli-only]` | 自更新 CLI + 菜单栏 app |

直接敲 `gtmux` 打印帮助，`gtmux --version` 打印版本。输出语言依次看
`--lang=en|zh`、`$GTMUX_LANG`、`gtmux config lang`，都没设就看系统 locale
（`LC_ALL`/`LANG`：`zh*` 出中文，默认英文）。所有东西都是显式调用，不装 shell hook，
任何 shell 都能用。

## `gtmux agents`

```
gtmux agents — 6 agents · 1 waiting · 1 working · 4 idle

⏸ waiting  Claude Code  api:0.0     permission to run tests     %7
⠿ working  Claude Code  web:0.0     refactor auth middleware    %11
✳ idle     Claude Code  worker:0.0  add retry backoff     %8  ✓ latest
✳ idle     Codex        docs:0.0    —                     %1

jump: gtmux focus %7
```

每行是**状态 · agent · 位置 · 任务 · pane id**，按紧急程度排序。

- **⠿ working**：在忙，别打扰。
- **⏸ waiting**：干到一半，卡在**你**这儿等一个授权或批准；永远排最上面。
- **✳ idle**：这一回合结束了，你想动的时候再动（不急）。
- **⚠ errored**（琥珀）：一个空闲会话，但结束在 API / 工具报错上（比如
  `Unable to connect to API`），不是干净地跑完。它仍然算空闲（该你动），只是标出它是怎么结束的，
  行上会带错误摘要。`--json` 里是 `error: true` + `error_text`。

`gtmux agents --watch` 是会自动刷新的实时看板（用
[bubbletea](https://github.com/charmbracelet/bubbletea) 做的）：约 1.5 秒一轮，
**↑/↓** 选择、**Enter** 跳到那个 pane、**r** 刷新、**q** 退出。`--json` 输出同样的数据，
给脚本和菜单栏 app 用。

### 它是怎么认出来的（不只支持 Claude）

- **状态**取自 agent 自己写的 pane 标题。开头是盲文旋转符（`⠋⠙⠹…`，大多数 agent TUI
  都在转这个）就是 **working**；Claude Code 的 `✳` 是 **idle**。这套判断对任何会转
  spinner 的 agent 都成立。
- **是哪个 agent** 靠前台命令匹配（`claude`、`codex`、`gemini`、`cursor`、`opencode` …），
  或者靠标题里的名字。
- 用 `~/.config/gtmux/agents.json` 扩展或覆盖：一个 `{"name","commands","idleGlyph"}`
  的 JSON 数组，你写的优先于内置。
- 只有 agent **进程真的在跑**，那个 pane 才会被列出来。一个普通 shell 顶着残留的 agent
  标题（比如 resurrect 恢复出来但从没重新拉起的会话）**不算**。
- 跑在 **tmux 之外**的 agent（终端里裸跑的 `codex`/`claude`）通过同一个 hook
  **被感知**到，只读地列在 **不在 tmux** 分区里，`source:"native"`。它们没有 pane
  （不能跳、不能回）；能 resume 的可以用 `gtmux adopt <session_id>` 拉进 tmux。

`⏸ waiting` 和 `✓ latest` 来自[通知 hook](#通知-hook) 写的状态文件。没装 hook 的话，
agent 永远不会显示 `⏸`，其余功能照常。

## `gtmux panes`

`gtmux panes` 列出**每一个** tmux pane，不只是 coding agent，是 `gtmux agents` 的全集。
它是 pane 浏览器背后那个只读的产出方：一棵 session → window → pane 的树，
或者 `--json` 给一个结构化数组。每个 pane 带一个 `tier`：`"agent"`（coding agent 的
pane，判定方式和雷达完全一致）或 `"plain"`（shell、编辑器、dev server、日志，其余都算），
另外还有位置、cwd、当前命令、标题、是否活动、是否在 copy-mode。

```sh
gtmux panes            # session → window → pane 树；▸ 标出 agent pane
gtmux panes --json     # [{pane_id, loc, session, window, pane, cwd, command, title, active, in_mode, tier, agent}]
gtmux panes watch %N   # 把一个普通 pane 挂到雷达上，作为「关注」行
gtmux panes unwatch %N # 摘掉
gtmux panes --watched  # 列出被关注的 pane id
```

为什么不和 `agents` 合并：`gtmux agents --json` 是一份锁死的契约，含义就是「coding agent」，
那是雷达。`panes` 是给能够到**任意** pane 的浏览器用的全集。`gtmux focus`/`send`/`attach`
本来就接受任何 pane id，所以 `tier:"plain"` 的 pane 是一等的 focus/输入/attach 目标，
它只是拿不到那些只对 agent 成立的智能（digest、1/2/3 批准、派活、中控）。

**分档能力对照** —— 同一套原语，不同权限（各端照此实现）：

| 档 | 在雷达上 | 查看/抓屏 | focus/跳转 | 输入/send | attach | 智能（digest / 1·2·3 / 派活 / 中控） |
|---|---|---|---|---|---|---|
| agent（tmux） | 自动 | ✓ | ✓ | ✓ | ✓ | ✓ |
| 普通 tmux pane | 手动挂（`panes watch`） | ✓ | ✓ | ✓ | ✓ | — |
| 感知到的非 tmux agent | 「不在 tmux」 | — | — | — | — | —（只读） |

agent 雷达**不会被稀释**：普通 pane 只有你用 `gtmux panes watch %N` 主动挂上去才会出现，
而且是一条独立的**关注**行（没有 agent 状态），pane 关掉就自动摘掉。访客的分享范围
在任何 pane 上同样把关可见与可输入。

## `gtmux digest` + `gtmux hq` —— 中控（supervisor）

`gtmux digest` 是一眼看清整支舰队，而且给的是**含义**不只是状态 ——
一张排好列的表，不是一堵散文墙：

```
1 needs input · 2 working · 1 completed

needs input (1)
  ⏸ api:0.0        1.Yes · 2.Yes, don't ask again · 3.No             3 opts   4m

working (2)
  ⠿ web:2.0        fix the login-token refresh bug                 working   1m
  ● hq:0.0 ⌂       ctx 92% — approaching limit                          ⚠   3h

completed (1)
  ✳ mobile:1.0     done, tests pass                                          2d
```

先是一行按状态的计数摘要，然后每个状态一节（先「需要你」，再 working、completed，
有报错的话最后是 errored）。每行是状态字形 · 名字 · 目标/最近/在问什么（按终端宽度截断） ·
右边一个徽标（派活状态 / 选项个数 / 用量告警） · 右对齐的相对时间。每个字段都是从
gtmux 已经知道的东西里**确定性**拼出来的（零 LLM token）：**goal** 是这个会话最后一条
用户提示，**last** 是它最后一条回复的尾巴（两者都来自 agent 自己的 transcript），
**asks** 是等待提示里解析出来的选项，再加上 errored / 后台运行这些修饰。`--json`
输出机器形态（也由 `GET /api/digest` 提供）。gtmux 没有 transcript 的会话，
仅凭雷达信号也照样渲染 —— agent 不需要配合。JSON 每行都自报感知档位 `sense`：
`driver`（agent 的 hook 供状态，transcript 供 goal/last）、`partial`（hook 通了但没解出
结构化内容）、`screen`（纯抓屏 / 进程推断）。这是新增字段，从 digest 手上已有的事实推出来，
不新采集任何东西；消费方可以据此调整信任权重。

`gtmux hq` 打开（已经在跑就聚焦，绝不重复起）**中控**：你的 coding agent 跑在
`~/.config/gtmux/hq/` 下一个专属 tmux 会话里，第一次会种下一份教它这套循环的说明书 ——
读 `gtmux digest --json`、做判断、值得的时候才钻进某个 pane（`tmux capture-pane`）、
用 `gtmux send` 驱动、向你汇报。说明书种成 `AGENTS.md`（跨 agent 的通用约定），
配一个 `@AGENTS.md` 导入的 `CLAUDE.md`，所以中控可以是**任何** CLI agent；
它按你的语言整份下发（`GTMUX_LANG` en/zh）：这个受管文件记着自己是什么语言，
版本变了会**用它原来的语言**重新生成（旧文件留备份，`LOCAL.md` 不动）。
在另一种语言的终端里跑 `gtmux hq` **不会**把它翻译过去 —— 它会说明情况，然后什么都不改。
只有 `gtmux hq --lang en|zh` 能改这一点。**全新启动**且没给 `--agent` 时，
`gtmux hq` 会**问你用哪个已装的 agent**（PATH 上有二进制、且装得上 hook 的那些），
并且**记住你的选择** —— 一台只登了 Codex 没登 Claude 的机器，中控不会再卡在
「Please run /login」上。你当然也可以直接指定（`gtmux hq --agent codex`，或者
`GTMUX_HQ_AGENT`）；非交互调用（脚本）不弹问，走默认。个性化写在同目录的 `LOCAL.md`：
你的优先级、汇报口味、免打扰时段，它能扛过每一次说明书升级；`AGENTS.md` 本身是受管的、
会被重新生成，改在那儿的内容会被挪进备份。它在那个目录里记的笔记跨会话保留。
在雷达里，它那一行带 `role:"supervisor"`。

`gtmux hq --board [--json]` **打印**态势板，而不是打开中控 —— 这是一次读取，
给了这个参数就只做这件事。它存在的理由是：让某个端能显示中控的综合判断，而不必知道
中控家目录在哪儿。那个路径是可搬的，而且至少在一台真机上是通过符号链接到达的，
所以解析它应该归 CLI，不该由每个消费方各写一遍。菜单栏 app 的态势板阅读器是第一个调用方。
从没写过态势板会返回 `exists:false`，这是正常状态，不是错误。

`gtmux hq --home` 打印同一个路径，给那些需要**动手**而不只是读的调用方：
`gtmux knowledge` 的写操作只接受来自中控家目录的调用，所以提供这些操作的端要**在那儿**
执行动词，与其自己再拼一遍路径，不如问 CLI「那儿」在哪。它一寸都没有放宽 cwd 那道闸 ——
动词仍然完全按 cwd 判定，而且路径从来不是秘密（每一次拒绝都会把它打出来）。
还没有中控家目录的机器照样打印路径，并以非零退出说明情况，因为那次即将发生的 chdir
本来也会失败，而且失败时你知道的更少。

`gtmux hq --rotate` 就地把在跑的中控会话退役、换一个新的：它解析出 hq 的 pane，
把那个 agent 自己的重置命令敲进去。这是中控**自己的**动词 —— 一旦 `self-rotate`
的敲门说这个会话已经磨损了，它的说明书就会不用你吩咐地执行
（见[自轮换](#自轮换--当中控自己的会话成了问题)），而且永远发生在
把 `notes/board.md` 和知识库更新到位之后，因为那两样就是下一个会话的全部交接。
它绝不会去启动一个中控：没有在跑的中控就没有东西可轮换，它会这么说。

### 唤醒通道 —— 中控是怎么知道事情的

决策密度高的事件会往活着的 hq pane 里敲进**一行**信号，这是唯一的敲门。
格式是固定的，也刻意不像对话，所以一屏信号扫一眼就能读：

<!-- gtmux:rendered wake-lines -->
```
» ◆ gtmux·waiting·permission  api:0.0 (%7) │ title:"run the tests?"
» ▸ gtmux·done  web:2.0 (%11) │ 3m │ goal:"fix the login bug" │ tail:"tests pass" · #a3f1c2
```

`» <等级> gtmux·<类别>  <位置> (<pane>) │ <字段> │ …`，其中每一段由 agent 或用户写出的
内容都加引号并带标签（`title:` / `goal:` / `tail:` / `ask:` / `err:`）——
它是中控**转述的数据**，绝不是它要照办的指令。

**等级**打头，位置固定，所以一屏信号线先按分量读、再按字读：
**`◆` 决策级**（需要你：不可逆、代价大，或者对方明确在问） ·
**`▸` 注意级**（某条线卡住了，或者变化到值得你知道） ·
**`·` 台账级**（记账：记下来、可拉取，打扰价值为零）。它是类别的一个**投影**，
不是对类别的第二种意见：什么该发、按什么严重度发，是在别处定的，这里只说它该读得多响。
这些字形和 `»`、`│` 过的是同一道编码关：不用 emoji，不越出语法已经用到的那些块，
因为颜色是那些自己掌控渲染的端可以再加的一层，绝不能是唯一的载体。类别如下：

| 类别 | 等级 | 什么时候发 |
| --- | :---: | --- |
| `waiting·<kind>` | ◆ | 一个 agent 卡在你这儿（授权 / 计划 / 提问） |
| `resolved` | ▸ | 那个等待**解除了**：你在 pane 里回了，或者 agent 自己继续了；中控会撤掉过期的追问 |
| `asks` | ◆ | 回合末尾的**回复**里问了个问题，但没有菜单（只看菜单的传感器会漏掉） |
| `done` | ▸ | **任何**会话干完活进入空闲，不限于派出去的任务。如果完成发生在你正看着的那个 pane 里就抑制（`hqWake.done`：默认 `unattended` \| `always` \| `tick`），并按 pane 合并限流 |
| `crash` | ◆ | 这一回合**死在** agent / API 报错上，绝不会被读成「完成」 |
| `goal-changed` | ◆ | 你直接往某个 agent 自己的窗口里提交了提示（包括斜杠命令），于是中控感知到一件不是它派的活 |
| `new-session` | ▸ | 新感知到一个 agent pane，去建联 |
| `reap-suggest` | ▸ | 某次派活看起来可以回收了，行里带着可直接用的 `gtmux reap <id>` |
| `stuck·waiting` | ▸ | 一个 pane 等你超时了。每次等待只发一次，而且只在**是 agent 在问**时才发（gtmux 仅凭屏幕推断出来的等待永不升级） |
| `resource·warn` / `limits·warn` | ▸ | 机器 / 订阅越过了某条线（有阻尼，见 `gtmux resource`） |
| `usage·warn` | ▸ | 某个会话越过了上下文 / 消耗的某一层（见 `gtmux usage`） |
| `wake-degraded` | ◆ | 感知本身坏了：唤醒不再落到 HQ pane 上 |
| `tick` | · | 周期简报，只在真的有变化时才发（安静的那一轮零成本） |
| `distill` | · | 该做一轮知识沉淀了（约每周一次；`gtmux capture` 攒够 5 条候选会提前）——中控把这一段时间的教训折进知识库，并剪掉过期的 |
| `self-check` | · | 中控该做自己的家务了（约每天一次）：清理陈旧的待办、检查记忆和日志健康 |
| `unread` | · | 有事件压在中控的消费水位之外：给一个计数和拉取游标，不对重要性下任何判断 |
| `self-rotate` | ◆ | **中控自己的会话**磨损了（上下文 / 时长 / 回合数）：它交接完自己轮换，见下 |

**常驻类别会自我复核。** 那些要等一个动作才能清掉的类别（`self-rotate`、`unread`、各种 warn）
在每次**重复**之前会问两个第一次敲门不必问的问题：前提还成立吗？上次说完到现在有变化吗？
一次前提集合和世界都没变的重复会被抑制，一旦发生漂移就重新武装，另有一条安全下限保证
常驻的债不会被忘掉；而排队中的唤醒在敲进去之前会**重新采样**，前提在排队期间已经死掉的那条
会被丢弃，而不是作为一句关于已经变了的世界的断言送达。

`distill` 和 `self-check` 是**维护类**：它们以最低优先级敲门，所以永远不会插到一个卡住的
agent 前面；两轮都只在中控真的做了事情时才出声。它们也是仅有的纯靠时钟触发的类别 ——
中控自己没有定时器，所以**是 `gtmux serve` 让这些周期仪式得以发生**。

其余一切都在**拉取侧**：中控醒来，然后自己读 `gtmux events --since-seq <n>` 或
`gtmux digest`。普通的进展回合永远不会碰它的屏幕。

**中控对信号线的回复也是信号线**：一行，以 `⟣` 加一个字形开头，所以它的 pane
和它的收件箱读起来是同一种扫法。图例：

| 回复 | 含义 |
| --- | --- |
| `⟣ ✅ <pane> <判断> → <下一步>` | 一次值得知道的完成 |
| `⟣ ▪ noted: <一句话>` | 例行结果，记进看板，什么都不欠 |
| `⟣ 📓 captured: <主题>` | 一条持久经验写进了知识库 |
| `⟣ ⚠ <升级>` | 有事需要**你**，按升级策略 |
| `⟣ ◈ 简报 <时间> │ <计数> │ 要事` | 周期简报（后面最多 5 行缩进的 `· `） |

### 消费水位 —— 为什么不会有东西丢掉

上面每一个类别，都是 gtmux 在判断某个事件值不值得敲门，而这个判断需要 gtmux 没有的上下文：
只有中控知道，它正在等的就是那个刚跑完的 pane。所以这些类别是**优先级标签，不是覆盖面** ——
它们说的是「先读哪个」。真正保证中控**总归会知道**的，是一条**消费水位**：gtmux 记录中控
读到了流的哪儿，一旦有事件压在水位之外超过 `hqWake.unreadDebounceSec`（默认 **120 秒**），
它就敲一次门，只给计数，别的什么都不说：

<!-- gtmux:rendered unread-line -->
```
» · gtmux·unread  7 unconsumed (%21 ×4 · %13 ×2 · control) │ pull: gtmux events --since-seq 6653 --json
```

这一行会说清它数的是**什么**，按来源分（`control` 是 gtmux 自己的维护触发，
`native` 是非 tmux 的 agent 会话），多的排前面，超过三项折成 `+N more` ——
所以「积压里绝大多数来自同一个 pane」这件事，不用拉取就看得见。

这笔债只有中控**消费**才会清，而且只有两件事算数：从中控家目录发起的、不带过滤的
`gtmux events --since-seq <n>`（醒来之后的日常拉取，也就是
「醒来就拉」这个日常动作，所以它不需要你养成任何新习惯），或者一次显式的
`gtmux events --ack <seq>`。带 `--severity` 过滤的读**不算**（它只看到子集），
从水位前面开始的读也不算（它跳过了中间那段）；检测到**序号断裂**的读同样不算
（有事件在没被读之前就轮转掉了）：否则这条警告只有一次被看见的机会，之后这次丢失就被
默默原谅了。所以拉取会重新告警，敲门行也会带上 `· sequence gap` 标记和一条重建提示
（`gtmux digest --json`，然后显式 `--ack`），直到中控刻意地 ack 过去为止。
在那之前，这条敲门每 `hqWake.unreadRepeatSec`（默认 **300 秒**）以常驻优先级重复一次。

有三类记录不计入**计数**：中控自己写的行，否则这条通道会永远自己喂自己；
一次没有 pane 的生命周期**闪烁**，也就是一条没有 pane 的 `SessionStart`，
它的 `SessionEnd` 几秒内就跟上了 —— 一个中控既没法处置也没法归属的短命子进程；
还有 gtmux 自己的**审计轨迹**（`gtmux:audit:*`），也就是这套监督**做过什么**的流水
（每一批唤醒送达或丢弃及其原因、每一次 `gtmux send` 的结算、回收、中控会话的轮换链），
它记的是中控本来就知道的动作，把它算进去等于每送达一次敲门就凭空造出一笔新债。
闪烁规则认的是那个 Start/End **配对**，绝不只认空 pane；审计规则认的只是那个子命名空间：
native（非 tmux）agent 的回合、以及 gtmux 非审计的 `gtmux:*` 触发（维护、降级、对账）
同样没有 pane，它们照样计数，而且对它们来说这条敲门是**唯一**的通道 ——
所有按类别的唤醒都要求有 pane。

**中控的拉取给出的是同一个集合**：那笔债，不含它自己的轨迹。在一支舰队上实测一周，
敲门送它去读的内容里有 68.7% 是它自己的回声，于是一条只带**一件**新事实的敲门，
要花掉一整个回合去读。现在计数和读取指的是同一批记录；`--all` 能把原始视图拿回来，
两者都算消费（一个恰好是它欠的，另一个是超集）。日志里什么都不会被删掉，
非中控发起的读原样返回。

这就是感知从「猜得准」变成「完整」的地方。在此之前，没有任何类别认领的事件就是不会到达：
一个跑完了、但那活不是通过 gtmux 派出去的会话，既不算 `done`（台账里没有条目）
也不算 `asks`（没有问题），于是这个回合末尾就正确地、但没人读地躺在流里，
直到用户当面问起。`gtmux doctor` 的 **HQ 维护**一节会报告这个滞后（`event consumption`），
落后 20 条以上或 30 分钟以上就标出来 —— 否则这个故障在两个方向上都是无声的。

投递会守住你的草稿，并且自己确认自己：一行永远不会被敲进非空的中控输入框
（它会排到磁盘上，等框清了再落），而一批内容只有在屏幕上被看到之后才会从队列里删掉 ——
所以一次失败的发送会重试，而不是消失。这让它成为**至少一次**投递，因此行尾有 `#<id>`：
同一个 id 出现两次就是重发，中控的说明书要求忽略它。每一种终态也都进了审计
（`gtmux:audit:wake-delivered` 带完整批次内容，`gtmux:audit:wake-dropped` 带原因：
被挤掉 / 未确认 / 被取代），所以「14:02 中控被告知了什么、又有什么从没被告知」
是一次 `gtmux events --all` 查询，而不是一次重建推理。

### 自轮换 —— 当中控自己的会话成了问题

上面每一个类别说的都是舰队。`self-rotate` 是例外，它的存在源于一次具体的故障：
**在一个又长又接近满的会话里，中控开始把自己的产出读成来自外部的输入。**
2026-08-03 就发生过一次：它读到一行「那条消息是我发的，别担心」，当成用户的安抚，
于是撤回了一个它本来提得没错的怀疑。那行是它自己上一个回合写的。事件流能定这个案，
因为这两种行为带着不同的类型：在中控的 pane 上，`UserPromptSubmit` 是你，`Stop` 是它。

中控自己抓不住这个 —— 本该发现问题的那个能力，正是退化了的那个 —— 它也没法给这次检查
排期，因为两次唤醒之间它根本没在跑。所以由 `gtmux serve` 从外面盯着并敲门：

```
» gtmux·self-rotate  ctx 82% · 14h · 380 turns │ over: ctx 82% ≥ 75% │ board+KB current → hand off → gtmux hq --rotate
```

三个事实被感知，**任意一个**越线就算越界：

| `hqWake` 键 | 默认 | 量的是什么 |
| --- | --- | --- |
| `selfRotateCtx` | **0.75** | 实时上下文占比，和 `gtmux digest` 报的 `ctx` 是同一个 |
| `selfRotateHours` | **12** | 会话年龄，从 transcript 的**第一条**消息算起（`serve` 重启也归不了零） |
| `selfRotateTurns` | **300** | 会话开始以来中控 pane 上的提示提交次数 |

把其中任何一个设成 **0**，就单独关掉那一条判据，另外两条照常。
`selfRotateRepeatSec`（默认 **1800 秒**）控制重敲的节奏，
`selfRotateCheckSec`（默认 **300 秒**）控制评估的节奏。默认值刻意保守 ——
一次你不信的敲门，比不敲更糟。

这笔债**只有会话真的轮换了**才清，判据是出现了新的 agent 会话 id；把唤醒送到不算清，
和消费水位完全一样。它也不会白白**重申**：过了 `hqWake.selfRotateRepeatSec` 之后，
只有越界集合或者舰队发生了变化才会再敲，而 `hqWake.selfRotateFloorSec`（12 小时）
是它在什么都没变时能保持沉默的上限。没有这条的时候，一次年龄越界（那是永远不会自己恢复的）
会每半小时敲一次、永远敲下去：实测有一夜在一支完全静止的舰队上敲了 17 次，
而上下文的上涨很大程度上正来自那些「回答敲门」的回合本身。中控的说明书要求它按顺序做三件事，
而且**不要**先问你：把 `notes/board.md` 和知识库更新到位（那是继任者的全部交接）、
记下这次交接，然后跑 `gtmux hq --rotate` —— 它会解析出中控的 pane，
把那个 agent 自己的重置命令（`/clear`，codex 是 `/new`）敲进去。
轮换之后又收到 `self-rotate`，意思是那次轮换没成，不是说还欠第二次。
`gtmux doctor` 的 **HQ 会话健康**一行给的是同一组数字，你可以据此核实它的说法、
或者质疑这些阈值。

在 `~/.config/gtmux/config.json` 里写 `"hqNudge": false` 可以整条通道关掉
（没有中控 pane 就没有唤醒，也没有开销）。唤醒只**告知**：gtmux 从不替另一个 agent
回答提问，从不往 TUI 里发导航键，默认策略也是让中控把决策交到你面前，而不是替你做。

## `gtmux capture` —— 往中控知识库里丢一条便宜的记录

```
gtmux capture "<一句话经验> @<主题>"   # 主题取自知识词表：六个内置的 + 中控自己声明的
gtmux capture --list                    # 看待沉淀队列
gtmux capture --list --json             # 同一个队列，每行带去重键
```

```
last distill: 3d ago
2 pending-distill candidate(s):
  [pitfalls] wrangler TLS-resets from the office network — retry
```

干活干到一半去写一条打磨好的知识库条目，成本高，于是就被跳过。
所以 `capture` 把**察觉**（当场一行）和**好好写下来**（攒到中控的沉淀轮再写）拆开。
它刻意是一个**公开**命令：任何 worker（不只是中控）学到一条持久的、跨场景的事实，
都可以把一条**候选**丢进待沉淀池（`~/.config/gtmux/hq/knowledge/.pending-distill.jsonl`）。
每行带着这条经验、它的主题标签、一个**去重键**（`<主题>/<经验 slug>`，
所以沉淀轮会把同键的候选**合并**，而不是散落一堆近似重复），
以及自动采集的事件上下文（当前 pane、事件水位、时间戳，调用方是被跟踪的派活时还有 `$GTMUX_TASK_ID`）。

一条候选**不是**知识库条目：中控的沉淀轮才是那道质量闸，由它判断什么算持久、归到哪个主题、
以及剪掉什么 —— 所以把入口开放是安全的（最坏情况是这条候选在沉淀时被丢掉）。
这是捕获闭环的第②层，见 `openspec/changes/archive/2026-07-29-hq-capture-loop`。

`--list` 会把**队列上次被清空是什么时候**放在开头，因为光看深度判断不出这个闭环还活着没有：
一个空队列，看上去和「昨天刚被沉淀清空」还是「从来没跑过」是一模一样的。
队列里攒够五条候选还会**把下一次沉淀提前** —— 一条捕获到的经验一天之内就归档，
不用等满一周。

`--list --json` 是同一个队列，但给的是**端**而不是读者：一个数组（绝不会是 `null`），
每行带主题、时间戳、来源，以及这个参数存在的理由 —— 它的**去重键**，
也就是 `gtmux knowledge dismiss --capture <键>` 要点的那个名字。文本形态不打印这个键，
所以一个 GUI 可以显示队列却永远动不了它。注意：动作的单位是键，不是行 ——
驳回一个键会消掉所有共用这个键的待处理行。

## `gtmux quiet` —— 中控可以说多少话

```
gtmux quiet on       # 只呈现 CRITICAL，最安静
gtmux quiet off      # 默认：NORMAL 及以上都呈现
gtmux quiet status   # 现在实际生效的是哪档
```

中控会给它发现的事情定级，这条命令定的是「打印给你看」的下限。低于下限的东西仍然**被记录** ——
它进注意力账本（`gtmux tasks --pending`），只是不进你的屏幕，所以调高这个门槛，
失去的只有打扰。`GTMUX_SURFACE_TIER` / `GTMUX_QUIET` 可以在单个进程里覆盖它。

有一样东西永远不会被安静掉：事件日志里的**读取时断裂**。那是中控在告诉你它可能漏了东西，
一个能把这句话消音的设置，会让其余所有读数都不再可信。

## `gtmux knowledge` —— 知识台账（带来源的条目）

```
gtmux knowledge add --topic pitfalls --title "wrangler 会 TLS reset，重试" [--body-file -] [--capture <键>] [--seq-range a..b]
gtmux knowledge supersede <id> --title "…" [--body-file -]   # 替换一条；历史留在台账里
gtmux knowledge retire <id> --why "…"                        # 剪掉，理由会留下来
gtmux knowledge dismiss --capture <键> --why "…"             # 驳回一条候选，并留痕
gtmux knowledge topic <名字> --desc "…"                      # 声明你自己的主题（clients、datasets…）
gtmux knowledge promote <id> --why "…" [--target "…"]        # 够 charter 级 → 导出简报
gtmux knowledge land <id> --ref "<pr/spec>"                  # 落地之后闭环
gtmux knowledge promotions [--json]                          # 待带走队列
gtmux knowledge list [--topic t] [--json]  ·  show <id>  ·  render [--check]
```

知识库的**权威是一份只追加的台账**（`~/.config/gtmux/hq/knowledge/.ledger.jsonl`）；
主题的 `.md` 文件是从它的有效条目**渲染**出来的 —— 归 gtmux 所有、带标记、会查漂移
（`render --check` 抓手改，`render` 复原）。每条都带**来源**：写入时的事件 seq、
可选的沉淀区间，以及消费了某条捕获候选时那条候选的 pane/seq/task 和键 ——
这些是继承下来的，不会在合并时蒸发。`add --capture <键>` 会把**所有**同键的待处理候选
消费进一条（沉淀纪律承诺的那次合并）；`dismiss` 把它们连同一条日志痕迹一起删掉，
于是这道质量闸的「拒绝」不再和它的「接受」一样悄无声息地消失。
每一次写操作都往事件流里追加一条 `gtmux:audit:knowledge`。

**给指挥官留了第二扇更窄的门**（hq-knowledge-on-phone）：`gtmux serve` 把知识库暴露给
经过 OWNER 认证的客户端（`GET /api/hq/knowledge` 是不带正文的索引、
`GET /api/hq/knowledge/entry?id=`、`POST /api/hq/knowledge/act`），并且只接受两个动词：
`land` 和 `retire`。这不是下面那条规则上的窟窿：cwd 那道闸挡的是把 **worker** 拦在质量闸外面，
而这扇门后面站的是指挥官，他的位阶在中控之上；尤其 `land` 是只有他掌握的事实
（中控能判断一条经验够不够 charter 级，只有那个把它带走的人知道它到没到）。
`add`/`supersede` 不在这扇门上 —— 它们要带正文。两扇门写的是同一条
`gtmux:audit:knowledge` 记录。

写操作**只接受来自中控家目录**的调用（和 `gtmux events --ack` 同一条按 cwd 定角色的规则）——
质量闸是中控；worker 用 `gtmux capture` 记候选。`list`/`show` 在哪儿都能用。

**主题词表由你扩展。** 内置六个主题（accounts、workflows、best-practices、pitfalls、
corrections、environment）；`gtmux knowledge topic <名字> --desc "…"` 声明你自己的 ——
和其它写操作一样是一次台账操作，立刻带着描述被渲染出来，`gtmux capture`、
每一个 knowledge 动词、以及派活时的知识回声都认它（自定义主题在那里和 pitfalls/workflows
一起出现；accounts / corrections / environment 刻意不进派活上下文）。
名字是 slug（`a-z 0-9 -`，不超过 40 字节）；内置的、已存在的、以及保留目录名会明确报错。
声明目前只能新增。

**一条 charter 级的经验有一个机械的出口**（中控自己的台账曾经说这个出口是缺的）：
`promote` 把一条有效条目标成 charter 级，并写出一份**晋升简报** ——
`knowledge/promotions/` 下一份自足的交接材料，带着这条经验、为什么、建议落在哪儿，
以及这条条目的完整来源。由人（或者他派出去的 worker）把它带到合适的持久规则载体上：
某个项目的 `AGENTS.md`/`CLAUDE.md`、团队 runbook、`LOCAL.md`（当这条规则管的是中控自己），
或者 gtmux 自己的仓库 —— 开发者提一个 openspec change，其他人就开个附上简报的 GitHub issue。
`land --ref` 闭环（这个 ref 可以是 PR、issue 链接，也可以就是一个 runbook 的名字），
并把简报删掉，而整条生命周期留在台账里。主题渲染会标出状态
（`⚑ promoted (pending)` / `→ landed <ref>`），`promotions` 的列表开头给计数和最老的年龄，
`gtmux doctor` 会标出等过了下限（约两周）的简报 ——
一条没人带走的晋升，正是这个出口要终结的那种腐烂。gtmux 自己绝不往任何仓库里写东西，
也没有任何自动派发：队列浮出来，指挥官定夺。

**迁移是渐进的**：第一次动到某个主题的写操作，会把它那份前台账时代的手写文件原样搬到
`knowledge/legacy/<主题>.md`（种下去之后从没动过的占位文件则直接被替换），
渲染里链过去，派活时的知识回声**两边都查** —— 所以中控按需要逐条迁移经验的过程中，
没有任何东西会失去触达。

## `gtmux spawn` / `gtmux send` / `gtmux tasks` / `gtmux reap` —— 带核验的派活

`gtmux spawn <目标>` 把新活派给一个 coding agent，并且确认它**真的落进去了** ——
这是中控（以及你）不用手搓 tmux 编排就能可靠开一个任务的方式：

```
gtmux spawn "给部署脚本加一个 --dry-run 开关"
gtmux spawn --title fix-auth-mw --worktree feat/dry-run --model opus "加一个 --dry-run 开关"
gtmux spawn --pane %14 "接着干，然后跑测试"
gtmux spawn --json "…"   # → {task_id, pane_id, loc, title, session, delivered, state, judged_by, evidence}
```

**把目标写进文件，这是标准动作。** 只要比一行短句长，就走 `--goal-file <路径>`
（或者 `--goal-file -` 读 stdin）；`gtmux send` 有同一条通道，叫 `--message-file <路径|->`：

```
cat > /tmp/goal.txt <<'EOF'
把 `hqPlaybookVersion` 提到 13，并且：
1. 运行 `make check`
2. 别让 shell 碰 $HOME 或 `for f in *; do echo $f; done`
EOF
gtmux spawn --title bump-playbook --cwd ~/src/gtmux --goal-file /tmp/goal.txt
gtmux send %14 --message-file /tmp/reply.txt
```

理由是结构性的，不是风格：**作为命令行参数传的目标，会先被你的 shell 解析，gtmux 根本没机会看到原文。**
在 `"…"` 里，反引号包住的东西会被**执行**，`$foo` 会被展开，换行直接结束命令 ——
于是上面那样一个目标会死在 `command substitution: syntax error near unexpected token 'done'`，
什么都没派出去。任何足够长的自然语言指令，迟早会含有其中某个字符，
所以「每次都仔细加引号」不是一个你能依赖的性质。文件通道上一个 shell 都没有：
字节从 文件 → gtmux → `tmux load-buffer -`（一个管道）→ agent 的输入框。
中间只做**一次**规范化，而且是写明的：最多剥掉一个结尾换行，因为每个 heredoc 都会加一个。
同时给文件和位置参数是错误，不是优先级规则。位置参数形式对短指令仍然很好用
（`gtmux spawn --pane %14 "接着干"`）。

**重跑一次失败的派活会收敛。** 一次半路死掉的 spawn，过去会留下一个 worktree，
让重试撞上去（`exit status 128`），而且每试一次多一个空会话。现在：`--worktree`
会**复用**已经服务于那个分支的 worktree（同一路径上挂着**另一个**分支仍然是硬错误）；
建会话之前，spawn 会先找自己上一次的尝试 —— 一条拥有自己会话、目标从没送达、
而且 pane 还活着的台账记录 —— 然后接管那个 pane，而不是在旁边再停一个，
并且更新**同一条**台账行；某一步失败且没有可续的东西时，这次调用创建的 worktree/分支会被回滚。
所以重跑同一条命令，落在一个 worktree、一个会话、一条台账记录上。

**窗口标题规范。** `--title` 命名的是这个窗口的**目的**：一个简短的动宾 kebab slug
（`fix-auth-mw`、`review-pr-518`），它会成为 tmux、雷达和 app 里的窗口名和 pane 名。
成功时 `spawn` 报出**标准句柄** `<loc> (%pane) · <title>` —— `loc` 是**实时**的 tmux
定位 `session:window.pane`（窗口号每次读的时候重算，所以在 `renumber-windows` 下仍然正确，
绝不会被写死进名字里）。引用一个派出去的窗口就用那个 `loc %pane · title`，
这样你能按号跳过去。中控的说明书要求每次派活都给一个简洁的 `--title`，每次汇报都带这个句柄。

**中控家目录隔离。** `spawn` 拒绝在中控家目录里跑 worker（显式 `--cwd` 指到那儿、
没给 `--cwd` 时继承的 cwd 落在那儿、或者 `--pane` 复用一个坐在那儿的 pane 都不行）：
那个目录的 `AGENTS.md` 是中控的章程，在那儿起的 worker 会读到它然后冒充中控。
请传 `--cwd <项目目录>` —— 中控的说明书要求每次派活都带上。

它会拉起 agent（默认是一个全新的 detached 会话，或者 `--pane <id>` 复用一个，
或者 `--worktree <分支>` 跑在一个隔离的 git worktree 里），**在构造上就走网络代理**
（绝不会裸启动然后 403），等 agent 起来，然后经 tmux **粘贴缓冲区**投递任务，
并**核验**它落进去了。

**等 agent 起来是一道真的闸，而且超时的时候它现在会说清是什么挡住了。**
pane 只有在这些条件下才算就绪：输入行已经画出来、没有信任闸或选择菜单挡着、
屏幕上没有启动横幅，而且两次抓屏逐字节相同（或者 agent 的会话启动事件已经发出）。
启动横幅是一种**靠等就会消失**的装饰：`Connecting…`、`Loading…`。
**常驻通知**不是：`⚠ N MCP servers need authentication · run /mcp` 说的是一个只有你能做的动作，
而且永远不会自己消失，所以它**不再**挡这道闸（它以前挡，于是任何挂着这条通知的机器上
spawn 都不可能成功，见 TROUBLESHOOTING）。超时的时候，失败信息读起来是
`✗ NOT delivered → <句柄> — evidence: … blocked by: <那行说不的内容>`，
后面跟着 pane 的**底部区域**，而不是整段回滚缓冲。

核验是分层的：对装得上 hook 的 agent（Claude Code、Codex …），它优先用会话事件流上确定性的
`UserPromptSubmit` 事件（不抓屏）—— 事件记下的开头和核验器要找的针来自**同一条**规范化管线，
所以一次真实的提交事件总能对上；否则退回到加固过的、两帧屏幕读取，靠结构定位输入框。
仲裁是**正向单调**的：流确认过的送达是终局（屏幕读取永远推翻不了它），
而且在给出任何 `delivered:false` 之前会再读一次流，所以一个卡在截止线上才到的确认不会被超时吃掉。
JSON 结果会说明是哪一层判的（`judged_by: driver|screen`），
于是一次误判可以被归因，而不用靠时间线去重建。
把回执能力关掉（`~/.config/gtmux/config.json` 里 `driver.<agent>.receipt: false`
或 `driver.enable: false`）会强制走纯屏幕读取那条路 ——
一条刻意更保守的兜底，用来隔离事件通道的故障。

粘贴是**带括号的**（bracketed paste），所以一条多行指令会作为**一个**草稿落下，
再由单独的 Enter 提交一次（如果原样发，每个换行都会作为一个裸回车抵达 TUI，
当场把那一行提交掉）。**唯一**算成功的是确认落地：被吞掉的 Enter 会退避重发，
粘了一半会重试，超时会以屏幕证据报 `delivered:false`，绝不会有无声的成功。
重试永远不会重复：文本最多粘一次，重粘只发生在确认为空的输入框里
（清除键只清一行，所以多行草稿可能扛得住它），而只是渲染晚了的粘贴会被放着不动、
而不是再粘一遍。排队中的提交报为 `state:"queued"`。
一道**重发互锁**会拒绝在一个时间窗内向同一个 pane 发送完全相同的载荷
（这样一次紧张的重复 `/compact` 不会连打两下）；`--force` 可以越过它。
飞行前检查（代理、机器资源、订阅窗口）只是提示，永远不拦。

`--oneshot` 通过 agent driver 的 headless 模式派一个**一次性**、非交互的 worker
（`claude -p … --output-format stream-json`、`codex exec --json`）——
只有支持 headless 的 agent 才接受，其余的会明确拒绝（绝不会悄悄降级成一次交互式 spawn）。
目标是作为**参数**传的，所以没有东西要粘、也没有落地要核验；这次运行仍然活在一个 tmux pane 里
（JSON 流看得见、雷达上有它的行、reap 也适用），而它生命周期的真相（跑完 / 崩了）
来自那条流加退出码，绝不来自屏幕分类。契约写明：一次性 pane 是**只能看**的，
你不能中途跳进去接管。这和 `--headless` 不同，后者只是不开终端标签页：
`--headless` 派出来的仍然是一个完全交互式、你可以 attach 进去操纵的会话。

`gtmux send <pane> <文本>` 现在**默认**用同一套落地核验（一确认就返回，
所以健康的发送仍然很快）；`--no-verify` 退出核验，`--force` 越过互锁，
`--json` 打印核验结果（`{delivered, state, judged_by, evidence}`，仅限核验过的发送）。

**一次发送绝不会写进别人没提交的那一行。** 粘贴是**追加**到输入框的，
所以如果那个 pane 的主人正在键盘前写到一半，投递会把你的载荷接在他的字后面、然后一起提交。
每一条路径 —— CLI、中控、`--no-verify`、还有手机 —— 都会先读草稿，然后拒绝
（`state:"refused-draft"`，pane 上什么都没写），并把草稿原样引回来给你看。
`--force` 可以豁免，而且刻意不豁免手机那把幂等键 ——
来自另一台设备的发送，恰恰是现场没人能撤销的那一种。

这道检查刻意做得很窄，因为**它的职责是保护一次发送，绝不是拦住一次发送**。
它只在一个**已知 agent** 驱动的 pane 上运行 —— 跑着 vim、ssh 或者别的 TUI 的 pane
没有输入框，在那儿读会把整段 transcript 当成「草稿」。它读的是**带颜色**的抓屏，
所以 agent 自己那条暗淡的「建议下一条命令」不会被当成你在打字；
而且它要在**两帧**里看到同样的东西才会拒绝。凡是它判断不了的，都放行：

| 它看到什么 | 它怎么做 |
|---|---|
| pane 里没有 agent（没有输入框） | 发 |
| 抓屏失败：tmux 打了个嗝、pane 没了 | 发 |
| pane 正在回滚（copy-mode） | 发 |
| 一个普通 shell，没有输入框 | 发 |
| 是你自己的文字：同一条消息在重发 | 发（幂等） |
| 别人的文字，而且看到了两次 | **拒绝** |

它最多花两次读取加一个轮询间隔，而且绝不循环 ——
所以它可能让一次发送慢一个已知的量，但不可能让它挂住。

一次以 `failed` 收尾的发送**可以立刻重试**：互锁会丢掉那条从没落地的尝试记录，
所以显而易见的下一步不会再被 `refused-duplicate` 顶回来。`queued` 的发送保留记录 ——
agent 已经收下了。`--no-verify` 和手机端 `POST /api/send` 跳过的是**确认**，不是机制：
每一条文本路径都是先粘贴、再把 Enter 作为独立的键发出去，
所以不核验的发送同样不会把多行消息拆开。`--key` 仍然是单个按键。
只要比一行短句长，就用 `--message-file <路径|->`，理由和 `spawn` 有 `--goal-file` 一样：
作为参数传的文本得先从你的 shell 手里活下来。

**普通终端 pane** —— pane 的进程子树里没有 coding agent，前台是个裸 shell（bash/zsh…）——
是**直接**打进去的：文本然后 Enter，没有输入框确认，也没有重发互锁
（同一条 shell 命令跑两遍是正常用法，不是重复派活）。那儿没有 agent 的输入框可以核验，
而且拿输入框确认去套一个 shell，只要 pane 的回滚缓冲里留着旧的框线字符（比如这个 pane
以前跑过 agent），就会误报失败。`--json` 把这种发送报为 `{"delivered":true,"state":"sent"}`：
发出去了，没有落地可核验。任何不能被证明是普通 shell 的东西（vim、ssh、
进程扫描漏掉的 agent）走的还是 agent 那条管线。

`gtmux tasks [--json]` 是**派活 / 需要你的账本**：每一个你派出去的任务，
带它的实时状态（undelivered / waiting / done / working / gone），需要你的排最前。
`--verbose` 会加上归档条目和注意力列（层级 · 优先级 · 是否呈现 · 处置）。

**`undelivered` 排在最前面，而且这是那个 pane 唯一告诉不了你的状态。**
一次死在就绪闸上的派活，留下的是一个活着的、**空的**、空闲的 agent pane ——
和一个刚跑完一回合的 pane 长得一模一样 —— 所以仅凭 pane 推出来的状态，
会把一个从没开始过的任务渲染成绿色的 `done`，旁边还印着你**本来打算**下的那个目标。
台账记录的是投递判定，而 `tasks` 尊重它：一条目标从没抵达 agent 的记录，
无论它的 pane 看起来多空闲，都读作 `✗ undelivered`。`queued` 的投递不算未送达
（agent 收下了，只是排在当前回合后面），而一次成功的补救会把记录闭合 ——
一条把**同一个目标**送进同一个 pane 的 `gtmux send` 会把这条记录标成已送达，
所以下面那个绕行办法不会留下一条永远错着的行。只认那个目标：
往 pane 里敲一句无关的话，洗不白一次根本没发生的派活。

`gtmux tasks --pending` 是**待决策常驻视图**，也就是「摆在你面前的事」唯一的家：

```
▸ t1kx8p2m9dq3  hq %21                 08-09 14:32  ship v0.48.0 or hold for §4?
▸ t1kx9r4w0aa1  worker-b %8            08-09 09:11  which branch should the migration target?
```

打头的字形是 `▸`（注意级：这张表上的条目在等一个决策，那不是记账）。
排序是全序且稳定的：等得最久的排前，然后按 pane，再按 id。
这个视图**只读台账**（不扫雷达），而且打的是**绝对**时间戳，不是「已等 3h」的倒数，
所以对一张没变过的表读两次，结果逐字节相同 —— 只有集合变了它才动。
正因为如此，一份简报可以指着它说「其余照旧」，而不必每次把整张表再印一遍。

条目进出这张表用 `gtmux tasks --await <task_id>` 和
`gtmux tasks --resolve <task_id> [处置]`（处置记录它是怎么离开的：
`decided` / `withdrawn` / `escalated`；不给就只是清掉）。
在表上的唯一判据是 `awaiting-commander` 这个处置，别的都不是，
所以任何别的处置同样会把条目从表上拿掉，而归档一条记录会把它从这个视图里关掉。

`gtmux reap <pane|task_id>` 安全地回收一次跑完的派活：它**先**跑一道安全闸
（worktree 必须干净、分支必须已合并），过了才杀会话、删 worktree、删掉已合并的分支；
闸没过就精确报告是什么挡住了，并且什么都不碰（`--abandon` 强行越过，
`--keep-branch` 保留分支）。每一步都报告自己的结果：**失败**的那一步会连同 git
自己给的原因一起列在 `⚠ but these steps failed` 下面，命令以非零退出 ——
一个在 reap 之后还活着的分支，绝不会靠「某行没被打印」来让你推断。
`--snooze [--for <时长]` 可以对一个你要留着的派活消掉回收建议。
当一次被跟踪的派活看起来可以回收时，活着的中控会收到一条
`» gtmux·reap-suggest … │ gtmux reap <id>` 唤醒 —— 回收永远是 建议 → 批准 → 执行，
不会自动发生。

## `gtmux usage` —— token 监看

```
● api:0.0        2.1M out · ctx 85% ·  7k/m   ⚠ ctx 85%
● web:0.0         830k out · ctx 60% · 391/m
Σ claude          2.9M out ·  7k/m · 2 sessions
```

按会话的 token 统计，确定性地从 agent 自己的日志里解析出来（零 LLM 调用）：
累计输出/输入、**实时**上下文占用（最后一条消息的 input + cache token，
对着一个由证据推断出来的窗口来判断），以及 10 分钟的消耗速率。
**分层阈值**按 agent 类型写在 `~/.config/gtmux/usage.json`：

```json
{"claude": {"ctxWarn": 0.8, "sessionOutWarn": 20000000,
            "typeRatePerMinWarn": 30000},
 "horizonMin": 30}
```

评估器还会**外推**（`当前 + 速率 × 时间窗`），所以你是在撞墙**之前**被告警的 ——
`ctx→80% in ~9m` —— 而不是撞上的那一刻。告警以琥珀色 `usage_warn` 出现在雷达行上
（`agents --json` / digest）、`gtmux usage` 里，以及作为每层一次的
`» gtmux·usage·warn …` 唤醒敲进活着的中控会话。`--json` 也由 `GET /api/usage` 提供。
hook 在每个生命周期事件上评估 —— 工具驱动的工作期间接近实时，
一次长时间静默的生成则在它下一个事件时结算。

**Claude 和 Codex 都支持，而两者的日志形状不同。** Claude 记的是每条消息花了多少，
所以总量是累加出来的；Codex 每个回合记一次会话的**运行总量**，所以总量就是最后那次读数 ——
把它们加起来会让一个会话的消耗被乘上它的回合数。这也意味着 Codex 的会话根本不需要全量扫描，
也不需要计数文件。有一点上 Codex 给的信息更足：它直接写出 `model_context_window`
（实测一个会话是 258400），所以上下文占比是拿真实窗口算的，
而不是「大于观测值的最小已知档位」。日志里完全没有用量的 agent 仍然有它那一行，
只是这几个字段是空的，和以前一样降级。

> **按网络环境启动：** gtmux 在拉起 agent 时（`gtmux hq` / `adopt` / restore / limits 命令）
> 会按需加上代理前缀，所以你不用在不同网络之间手动来回切。
> `~/.config/gtmux/config.json` 里 `"agentProxy": "auto"`（默认）表示：
> **仅当那个端口在监听时**（也就是你的代理工具在跑，家里挂 VPN 的情形）
> 才加 `http://127.0.0.1:<agentProxyPort，默认 7897>`，否则什么都不加（内网）；
> 写一个明确的 URL 就强制用它，`"off"` 关掉。

## `gtmux events` —— 会话事件流（订阅）

```
22:50:40  working          api:0.0        Claude Code (%7)
22:51:02  waiting·permission  api:0.0     Claude Code (%7)
22:53:19  idle             web:1.0        Codex (%11)
```

hook 把每个会话的生命周期事件（开始 / 结束 / 等待 / 后台）追加进一个会轮转的日志
（`~/.local/share/gtmux/events.jsonl`，活动 20 MB + 1 个轮转 ≈ 40 MB 上限，
配置项 `eventsCapMB`，`0` 关闭）。`gtmux events` 打印最近一小时；
`--since 10m|2h` 给一个时间窗；`--follow` 实时流式输出，并且认得轮转（不会悄悄停掉）。
`--since-seq N` 是一次性的**增量**读取（严格在序号 N 之后的全部，最旧的在前，
可以和 `--severity`/`--json` 组合）—— 这是「醒来就拉」那个原语：
gtmux 中控被一条指明序号区间的信号线唤醒，然后精确拉那一段增量，
任何能跑 CLI 命令的 agent 都做得到（不需要后台 tail）。
这是同一批事件的终端原生**订阅**，各端拿到的是 SSE 版本。

一次不带过滤、**从中控家目录发起**的 `--since-seq` 读取，同时也是中控的消费回写：
它把水位推到它这次返回内容的末尾，这正是让 `unread` 敲门停下来的东西
（见[消费水位](#消费水位--为什么不会有东西丢掉)）。
`--ack N` 显式回写水位，用于那条流是以别的方式对上账的场合，比如一次完整的 `gtmux digest`。
两者都只对中控生效（按 cwd 判定）；一个 worker 在某个仓库里跑 `gtmux events` 什么都不改。

因为规则认的是**那个确切的 cwd**，从中控家目录的**子目录**读取（`notes/`、`knowledge/` ——
中控写完看板之后就落在那儿）不算数。这件事以前是无声的，于是中控读了增量、以为自己消费过了，
然后眼看着同一个游标再敲一次门；现在它会在 stderr 上告警，并指出该在哪个目录里跑，
而 stdout 和退出码完全不变。从一个不相干的目录读取仍然静默 —— 那儿本来就没有水位可以错过。

中控自己的增量拉取也**收敛到那笔债**上：它略掉那些从来不计数的记录
（中控自己 pane 的行、没有 pane 的闪烁、以及 gtmux 的 `gtmux:audit:*` 轨迹），
并在 stderr 上说明扣掉了多少条。`--all` 拿回原始视图；两种形态都算消费，
因为哪一种都没有给中控看**比它欠的更少**的东西 —— 而那正是 `--severity` 读取不够格的原因。
别人的读取一切照旧。

`--severity <档>` 过滤出那一档**及以上**，而这些档排的是**紧急度**，不是相关度 ——
所以它们是三种不同的读法，不是一种：不带过滤的 `--since-seq` 增量是用来**对账**的；
`--severity notable` 是**舰队变化**流（一条指令抵达某个会话，`origin:"instruction"`，
加上回合结束和生命周期）；`--severity important` 是**升级**子集（卡住 · 在问 · 崩了），
用来先分诊。过滤是分诊的捷径，永远不是全貌。

这条流里还带着 gtmux 自己的**控制记录** —— 它替中控发起的那些周期维护触发，
渲染成 `[CONTROL <事件>]` 并带上理由：

```
09:57:16  [CONTROL gtmux:self-check]  due (daily) — review feed/ledger/memory health…
04:33:49  [CONTROL gtmux:distill]     due (weekly) — distil the period into the KB…
```

这让「那轮周期任务到底跑没跑」变成一个可以直接回答的问题：
`gtmux events --since 30d | grep distill`。这件事值得知道，因为两轮任务在设计上都是安静的：
没有记录的话，一个从没沉淀过的中控，看上去和一个「没什么可沉淀」的中控一模一样。
`gtmux doctor` 把同一个答案渲染成结论 —— 它的 **HQ 维护**几行会显示每轮上次跑在什么时候，
并标出已经落后于自己节奏的那一轮（只在有中控家目录的机器上显示）。

## `gtmux resource` —— 本机资源监看

```
disk 40GB free · mem 38% free (warn) · load 0.64×14 cores · power 74% (battery 2:13)   ⚠ disk 40GB free
per-agent (RSS · CPU):
  %26    252MB · 9.2%
reclaim candidates (orphans no live agent owns):
  pid 3015  100MB · 0.0%  iOS Simulator runtime (12 procs) [simulator]
    ↳ leftover iOS Simulator runtime — `xcrun simctl shutdown all`
```

磁盘（`df`）、内存（`memory_pressure -Q` 的空闲百分比，加上内核
`kern.memorystatus_vm_pressure_level` 的 normal/warn/critical 档）、
CPU（loadavg÷核数），以及**电源/电池**（`pmset -g batt`：电量 % · 接电还是在放电 · 剩余时间；
没有电池的机器上不显示 —— 低电量**只有在放电时**才计入告警和档位，接着电时永不计入）。
**按 agent 的 RSS/CPU** 靠走每个 pane 的进程树得到（和 token 统计同构），
以及**可回收候选** —— 那些没有任何活着的 pane 认领的重进程，
带 pid 和回收办法（残留的 iOS 模拟器运行时会聚合成一条，dev server / tmux 游魂各自单列）。
阈值在 `~/.config/gtmux/config.json` 的 `resource` 对象里
（diskAmberGB 50 / diskRedGB 15 / loadAmber 1.0 / loadRed 1.5 /
orphanRssMB 300 / batteryAmberPct 20 / batteryRedPct 10）。
`GET /api/usage` 里带一个 resource 块；serve 的节拍会给中控发 `resource·warn` 提醒
（单写者：每次越线一次）；`gtmux hq`/`new` 在红档时会先警告再加负载。

**告警**有三重阻尼，所以一个卡在阈值上的数值不会反复告警
（读数本身保持原始 —— `gtmux resource` 永远报它量到的东西）：

| 键 | 默认 | 作用 |
|---|---|---|
| `diskHysteresisGB` | 2 | 高出入口线多少 GB 才解除磁盘档（<15 GB 进红，≥17 才解除） |
| `loadHysteresis` | 0.15 | load÷核数低于入口线多少才解除负载档（≥1.0 进琥珀，低于 0.85 才解除） |
| `batteryHysteresisPct` | 3 | 高出入口线几个百分点才解除电池档（<20% 进琥珀，≥23% 才解除） |
| `confirmSamples` | 3 | 连续几次采样一致才相信这次档位变化 |
| `minRestateMinutes` | 30 | 同一档再次告警前的安静期 —— 升到更糟的档不受此限，永远立刻告警 |

## `gtmux limits` —— 订阅窗口真实剩余量

```
● session               16% used   resets Jul 13 at 1:29am
● week (all models)     60% used   resets Jul 17 at 10:59pm
● week (fable)          90% used   resets Jul 17 at 10:59pm
⚠ near the weekly cap: week (fable) 90%
```

本地估算给不了的那个数字：**你的套餐还剩多少** —— 真实的服务端数据，
来自 agent 自己的上报，绝不是逆向出来的接口。
**Claude 和 Codex 报在不同的地方，所以 gtmux 各按各的读：**

- **Claude** 本地什么窗口信息都没有（transcript 里是会话花费，stats 缓存是全时段模型总量），
  所以 gtmux headless 地跑它自己被认可的命令：`claude -p "/usage"`。
- **Codex** 把服务端的限额响应直接写进了它的会话 rollout，就在 token 计数旁边，
  所以 gtmux 读出来就行。不起进程，不用命令。Codex 自己的 `/usage` 打的是活动热力图，
  不是剩余额度，所以那条命令路线在它这儿本来也没有对应物。

日志这条路有两条规矩，命令那条不需要。窗口按**时长**命名，绝不按它在数据里的位置：
实测 Codex 的 `primary` 字段里既出现过 5 小时窗口，也出现过周窗口。
另外一次读数**可能比它自己的窗口活得还久**，因为日志只新到最后一个回合为止 ——
重置时间已经过去的窗口会被丢掉，而不是照报，因为那时候那个百分比是**未知**，不是「低」。
每个窗口都写明属于谁的套餐，这样 `spawn` 的飞行前检查提的建议，
针对的是这活真正要计费的那个套餐。因为这会起一个进程，结果会**缓存**
（`state/limits.json`），TTL 15 分钟，一旦有窗口接近上限就缩短到 5 分钟；
`--refresh` 强制刷一次。配置在 `~/.config/gtmux/usage.json`：

```json
{"limitsCommand": "claude -p /usage", "limitsTTLMin": 15,
 "limitsTTLNearMin": 5, "limitsNearPct": 70, "limitsWarnPct": 85}
```

网络需要的话，可以在 `limitsCommand` 前面带上环境变量前缀
（`"HTTPS_PROXY=… claude -p /usage"`），或者设成 `""` 关掉。
周窗口到达或超过 `limitsWarnPct` 会标成琥珀，并唤醒活着的中控一次
（`» gtmux·limits·warn …`）。`limits` 这一块也随 `gtmux usage` 和 `GET /api/usage` 一起给出。

## `gtmux awake` —— 合上盖子也继续跑

```
gtmux awake on       # 要一次管理员密码，然后核实确实生效了
gtmux awake          # awake = on (clamshell) · up 2h13m · power battery 74%
gtmux awake off      # 不要密码，立即生效
```

合上 MacBook 的盖子，系统就睡了，隧道断掉，每个 agent 冻在回合中间 ——
所以「用手机指挥你的 Mac」以前只在盖子开着时成立。这条命令改变的就是这一点。
（它在 v0.44.0 里叫 `gtmux server-mode`，旧名字仍然能用。这个**功能**还叫服务器模式，
只是命令变短了。）

**打开要一次密码，关掉不要任何代价。** 这个不对称就是整个设计：
提权是本地的、交互的，而且在它持续的整段时间里刻意可见；
降权是免费的、自动的、永远可行 —— 包括 gtmux 已经死掉的时候，而那恰恰是最要紧的时候。

支撑这一点的，是同一次授权里装下的一个 root 属主的小**守卫**。
它内部没有任何一条能禁用睡眠的代码路径，它唯一的权力是把睡眠还回来；
下面任何一件事发生，它都会恢复睡眠并删掉自己：

| 触发 | 意思是 |
|---|---|
| 你把它关掉 | 一个不需要特权的标记；守卫大约一秒内就会醒来处理 |
| 电量掉到 20% | 30% 时就会在 Mac 和手机上提醒你 |
| gtmux 不再运行 | 崩溃、强杀、`brew uninstall` —— 不需要任何东西活下来 |
| 重启后没人登录 | 有一段启动宽限期，所以一次正常的重启不会杀掉你的会话 |

**它永不过期。** 你不关它就一直跑。取代计时器的是：菜单栏图标在整段时间里带着一个
缓慢呼吸的红点 —— 和屏幕录制同一种视觉语言，因为这个功能真正的风险是被忘掉。

**用电池是被支持的场景，不是隐患**：合着盖子在房间之间走动照样工作（实测：拔掉电源、
合盖、零次睡眠）。终结它的是**剩余电量**，不是失去适配器。

**gtmux 只回退 gtmux 设过的东西。** 一个不是它盖的章的 `disablesleep`，
它会报告出来并给出手动撤销的命令，绝不替你改 —— 和 `gtmux reap` 对一个不干净的 worktree
用的是同一套「只报告」纪律。`gtmux doctor` 会呈现同一个发现，
在从没碰过这个设置的机器上则保持沉默。

**该从哪儿读状态** —— 这是微妙的地方，读错了就是这个功能最糟的失败
（在一台根本睡不了的 Mac 上宣布「睡眠已恢复」）：

| 来源 | 能不能用 |
|---|---|
| `pmset -g` / `-g custom` / `-g live` | ❌ 两种状态下都**从不**报告 `disablesleep` |
| 电源管理 plist | ⚠️ 落后于写入；它回答的是「重启后还在不在」 |
| `ioreg -r -c IOPMrootDomain` → `SleepDisabled` | ✅ 实时、不需要特权的真相 |

`gtmux awake --json` 会把两种读数一并报出来，外加 `owned_by_gtmux`、`guard` 和一个
`platform` 结论。在项目没有验证过的 macOS 上，`on` 会直说，而不是悄悄失败；
在机制根本不存在的地方，它会在**要密码之前**就拒绝。

**两条边界，明说而不是糊过去：**

- `gtmux serve` 是按用户的 LaunchAgent，所以重启之后要有人登录它才会起来。
  在一台开了 FileVault、又没人在场的 Mac 上，心跳永远不会恢复，睡眠会被还回去 ——
  这是正确的失效安全，但也意味着服务器模式**扛不过**一次无人值守的重启。
  gtmux 不会通过去动 FileVault 或者自动登录来「修好」这件事。
- 底层那个设置**苹果没有文档**。它在 macOS 26 上验证过，并在运行时探测；
  如果将来某个 macOS 去掉了它，`on` 会带着理由拒绝，而不是悄无声息什么都不做。

你的手机能**看到**这个状态（连接点上的一个环，以及服务器 / 管理 Mac 里的一行），
但永远改不了它 —— 每一条改它的路径最终都通向在 Mac 上敲一次密码，
所以一个只能关不能开的远程开关，反正还是会把你送回笔记本前面。

## `gtmux restore`

**一次只能恢复一次。** 一次恢复会把整个工作集打开，所以两次并行就打开两遍 ——
而人们触发第二次的方式，正是慢动作最容易招来的那种：因为看不出有事发生，就再问一遍。
一次运行会持有一把锁（pid + 启动时间），第二次运行会说明情况然后什么都不做。
锁的进程已经没了、或者锁超过 10 分钟，就会被接管：一次崩溃不能把这个功能卡死。
`--plan` 和 `--dry-run` 不受此限 —— 恢复正在跑的时候拒绝**展示**计划，
那是守卫挡住了答案。

退出终端不会杀掉 tmux 服务器和任何会话，没了的只是标签页。重新打开之后，
在任意一个标签页里跑**一次**：

```sh
gtmux restore            # 每个 tmux session 一个终端标签页，全部接回
gtmux restore --pick     # 挑要哪些：「1 3」/「1,3」，回车 = 全部，q = 取消
gtmux restore --one      # 在当前标签页接回下一个没接的会话
gtmux restore <名字>     # 在这里接回指定会话
gtmux restore --dry-run  # 打印会发生什么，什么都不改
gtmux restore --plan     # 预览：哪些会话 + 哪些 agent 对话会回来（只读）
gtmux restore --plan --json   # 同一份计划的 JSON（菜单栏那行可展开的恢复项就读它）
```

真正的 `gtmux restore` 会在开头打印同一份计划 —— 它即将带回来的会话，
以及每个 pane 下面那段 agent 对话（目标）—— 所以恢复的过程中你看得见在恢复什么，
事后也有一份可核对的清单。`--plan` 就是把这份预览单独拿出来：
它读最后一次 resurrect 存档加上 resume 记录，不启动任何 tmux（随时跑、随时轮询都安全）。
标着 `×` 的 agent 行，是那段 transcript 已经从磁盘上消失、恢复不了的对话。

**哪些 pane 会把 agent 带回来。** 只有存布局时**确实在跑** agent 的那些 ——
恢复是从存档里每个 pane 自己的命令记录读出来的，不是从「这里曾经住过一个 agent」。
存档时是普通 shell 的 pane，回来还是普通 shell，哪怕你上周在里面跑过 agent。
（它以前会被塞进一句 `claude --resume …`：resume 记录是 agent 的 hook 写的、从不清理，
于是每一个曾经承载过对话的 pane 都成了永久目标 —— 一次重启把 10 个活的 agent pane 变成 16 个。）
一个活着的 pane 拿到哪段对话，仍然按那个 pane 的 resume 记录来；记录丢了的话，
恢复会从存档记下的那条 `--resume` 命令里把 id 抠出来。

**存档到底有多新。** 恢复总会打印它正在放回去的那个时刻 ——
「Restoring the layout saved at 09:57 (37m ago)」—— 因为悄悄让你丢工作的不是一份古老的存档，
而是一份**看起来很新**的存档。写那个文件的自动保存挂在 tmux 的状态栏上，
所以它只在有终端接着、并且在重绘的时候才跑：合上盖子它什么都不保存，配置得再正确也一样。
`gtmux serve` 会盯着那个**文件**而不是配置来兜底 —— 如果大约 10 分钟没有任何东西写过存档
（有自动保存触发器时是约 20 分钟，先把第一手让给它），serve 就自己跑一次保存。
`gtmux doctor` 的 `resurrect autosave` 一行报告的是同一件事：
一个已经武装、但几个小时没保存过的触发器会被标出来，不会被称作 OK。

**回来了什么，会拿去和存过什么对账。** 恢复之后，gtmux 把每个存下来的窗口的 pane 数量
和排布，跟活着的那个比一遍，把对不上的点名出来，打在终端上并写进
`~/.local/share/gtmux/restore.log`。这件事要紧，因为过去两边都是无声的：
gtmux 只数会话**名字**，而 tmux-resurrect 会把自己的布局错误丢掉 ——
于是当一个窗口回来时比存档多出一个 pane，tmux 会拒绝套用那个布局
（「have 3 panes but need 2」），那个窗口就悄悄保持一个默认的堆叠排布，
而任何地方都没有一句话提到这件事。

**pane 记录会和真实存在的 pane 对账。** tmux 的 pane id 是每个服务器的序号：
重启服务器之后，`%25` 会发给另一个 pane。gtmux 有很多状态是按这个号索引的，
所以恢复时（以及 `gtmux serve` 每隔几分钟）会丢掉那些 pane 已经不在的按 pane 索引的记录 ——
否则昨天的目标、派活和唤醒记录，会开始描述那个继承了它们号码的家伙。
对话记录（`resume/`、`usage/`）按定位和对话 id 索引，永远不动。

诊断它不需要重启 —— 把 `XDG_DATA_HOME` 指到任意一份存档的副本上，只读地预览：

```sh
mkdir -p /tmp/probe/tmux/resurrect && cd /tmp/probe/tmux/resurrect
cp ~/.local/share/tmux/resurrect/tmux_resurrect_<时间戳>.txt . && ln -sf tmux_resurrect_<时间戳>.txt last
XDG_DATA_HOME=/tmp/probe gtmux restore --plan     # 从**那份**存档里恢复会带回什么
```

第一次跑会弹一个自动化权限对话框（「想要控制 Ghostty」，或者 iTerm2/Warp，
看你的标签页由谁托管），点允许。**重启之后** tmux 服务器也没了；`gtmux restore`
会启动 tmux 并显式驱动
[tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) 恢复最后一次自动保存
（它会等恢复完成 —— 大布局要 30 秒以上 —— 如果存档在但恢复不了，它拒绝覆盖那份存档）。
跑着的程序不会被重启，请自己重新拉起，比如 `claude --resume`。

只要 resurrect 配了抓取，每个 pane 之前的输出（回滚缓冲）也会作为快照回来。
推荐写进 `tmux.conf`：

```tmux
set -g @resurrect-capture-pane-contents 'on'   # 快照每个 pane 的回滚缓冲
set -g history-limit 50000                     # 保留/恢复多少回滚
```

> shell 的 **↑ 命令历史**是另一回事 —— 它住在你 shell 的 histfile 里，不在 resurrect 里。
> 默认只在 shell 退出时写盘，所以一次重启会丢掉最近的命令。想立刻落盘（bash）：
> 在 `~/.bashrc` 里 `shopt -s histappend; PROMPT_COMMAND='history -a'`（zsh 是
> `setopt INC_APPEND_HISTORY`）。

## `gtmux overview`

```
gtmux overview — 2 sessions · 3 windows · 5 panes

▶ web-api              1 window · 1 pane
    0: web-api *  (1 pane)
● worker               2 windows · 4 panes
    0: editor  (1 pane)
    1: claude *  (3 panes)

▶ current  ● attached  ○ detached   * active  Z zoomed  • new output
```

在任何 shell 里给出的 session/window/pane 汇总。`--popup` 按 tmux `display-popup`
的尺寸排好版，所以你可以绑一个键，让它浮在一个全屏程序上面而不打断它。

## `gtmux new`

```
gtmux new                    # 用当前目录名给会话命名
gtmux new api                # 叫 api
```

新建一个 tmux session 并开一个接上它的终端标签页，走的是 `focus` 和 `restore`
用的同一个终端驱动 —— 所以标签页落在你看得见的地方，
而不是变成一个你之后还得去找的 detached 会话。

## `gtmux adopt`

```
gtmux adopt 4f0c1a2b                 # 在一个新的 tmux 会话里恢复那个原生会话
gtmux adopt 4f0c1a2b 91de77c4        # 一次接几个
```

在 tmux **之外**起的 agent 是只读感知的（**不在 tmux** 那一节 ——
它的 hook 触发时没有 `$TMUX_PANE`，所以 gtmux 知道它存在，但没有 pane 可以显示、
可以跳、可以输入）。`adopt` 用唯一可行的方式解决这件事：按会话 id
在一个全新的 tmux 会话里恢复那段对话，从此这一行就是完整的一行。
id 从 `gtmux agents --json`（`session_id`）或者雷达行上取。
只有 CLI 支持按 id 恢复的 agent 才能被接管，其余的会被列出来但不动，
而不是接管到一半。

## `gtmux focus`

```sh
gtmux focus web          # 把显示 session「web」的终端标签页调到最前
gtmux focus %11          # 跳到那个确切的 window+pane，然后聚焦它的标签页
```

每个标签页的标题是 `session — window`，所以 `focus` 找到匹配的标签页并把它调到最前
（通过终端的 AppleScript）。给 pane id（`%N`）还会在会话内部选中那个 window+pane，
于是你正好落在 agent 所在的位置 —— 点通知之所以能把你直接放到刚跑完的那个 agent 上，
就是这个机制。

**一个没有窗口开着的会话** —— `--headless` 派出来的，或者你自己 detach 掉的 ——
没有标签页可以调到前面，所以 `focus` 会**开一个**并接上去。
它以前会在 tmux 内部选中那个 pane，然后去找一个不可能存在的标签页，
看起来就跟跳转坏了一模一样。判据是客户端数量（`session_attached`），
不是这个会话当初是怎么起的：一个后来被人接上的 headless 会话就是一次普通跳转。
各端会把这样的行标出来（`no window` / `无窗口`），所以你点之前就知道会开一个新标签页。

> 需要 `set-titles on` 配 `set-titles-string '#S — #W'`，标签页标题才会保持
> `focus` 匹配的那个格式。如果还有别的工具也在写标签页标题，把它关掉，让标题保持权威。

宿主终端：**Ghostty** 和 **iTerm2** 是完整驱动的（AppleScript 精确到标签页）。
**Warp** 是尽力而为 —— 它没有 AppleScript 词典，所以只有当那个标签页的 Warp 会话 uuid
被记进了 tmux 的会话环境时，`focus` 才能跳到确切的标签页
（gtmux 自己的 restore/new 接入会做这件事；或者把 `WARP_TERMINAL_SESSION_UUID`
加进 tmux 的 `update-environment` 以覆盖手开的标签页），否则就只是把 Warp 激活到前台；
`restore`/`new` 通过启动配置来开 Warp 标签页。其它终端回退到 Ghostty 驱动。
宿主是自动探测的，`GTMUX_TERMINAL=ghostty|iterm2|warp` 可以覆盖探测结果。

## `gtmux attach` —— 在另一台机器的终端里，进远端会话干活

`focus` 跳到的是**本地**标签页；`attach` 把一个**远端** pane 开在你当前的终端
（Ghostty / iTerm2 / Terminal）里，作为原始、可交互的透传 ——
本地终端变成那个远端 tmux 会话，走的是同一个 `gtmux serve` 面
（一个 WebSocket，`GET /api/attach`），并遵守 owner/访客的 token 范围。

```sh
# owner：带 serve token 的完全访问
gtmux attach http://<mac>:8765 --token <serve-token> %12

# 访客：一条受限的分享链接（来自 `gtmux share new`，或者菜单栏的 分享 → 新建链接）；
# 主人放开什么就 attach 什么：
gtmux attach 'https://<mac>.example/#g=<token>' %12

gtmux attach <目标>              # 不给 pane：只有一个就自动接，否则让你选
gtmux attach <目标> --read-only  # 只看，永不发送输入
gtmux attach <目标> --predict    # 实验性：打字时把往返延迟藏起来
```

**`--predict`（实验性，默认关）** —— 预测性本地回显，把 mosh 的想法搬到我们这条
WS/TCP 桥上。在慢链路上，每一次击键否则都要等一个完整往返才能回显
（跨洲隧道实测约 340 ms）。开了 `--predict` 之后，你自己打的可打印字符和退格会
**立刻出现，带下划线**表示尚未确认，权威输出一到就被抹掉 —— **服务端屏幕永远赢**，
所以一次猜错会在一个往返之内被纠正，而不是留在那儿。真实击键原样转发给那个 pane，
预测只在本地画。它刻意保守：**自适应**（快链路/局域网上什么都不画，回显本来就是瞬时的）、
**全屏 TUI 里绝不预测**（服务端会告诉客户端这个 pane 进了备用屏幕），
任何会改变状态的键（Enter、ESC、方向键、Ctrl-C、Tab）都会结束这一段预测，
而不是跨过一次不可预测的屏幕变化去猜。客户端的光标位置是从服务端学的（tmux 知道），
不是自己模拟一个终端，见 `docs/design/mosh-predictive-echo-research.md`。

不给 `%N` 而又有多个 pane 可接时，`attach` 会给一个**带编号的菜单**
（每行是 会话 · agent · 状态 · 任务），你选哪个就连哪个 —— 回车取第一行，`q` 取消。
被管道接走或者在脚本里跑时（stdin 不是 TTY），它改成把列表打印出来并以非零退出，
所以自动化不会卡在提示上。

- **`<目标>`** 是一个地址（加 `--token`，→ **owner**，完全权限），
  或者一条 `…/#g=<token>` 分享链接（→ **访客**，受限于主人的可见/可输入白名单：
  只可见的 pane 就是只读，不可见的 pane 直接拒绝）。
- **`%N`**（可选）是要 attach 的 tmux pane id —— 它选中的是那个 pane 所在的**会话**。
  不给的话，只有一个会话时自动接，否则（在 TTY 上）从带编号的菜单里选；
  非交互时列出来并以非零退出。
- **退出**用 tmux 自己的 `<prefix> d`，或者 **`Ctrl-]`**（本地的逃生口）。
- 范围由服务端强制；`--read-only` 是方便，不是安全边界。
  设计与取舍见 `docs/design/remote-attach-research.md`
  （WS/TCP 上的原始透传、流控、resize、延迟）。

> 前提是主机可达：局域网里直连，或者通过 `gtmux tunnel` 从任意网络连
> （WebSocket 和雷达走同一条隧道）。访客那边完全在菜单栏里配置
> （按 pane 的 👁 可见 / ⌨️ 可输入 + 新建链接），或者用 `gtmux share`。

## `gtmux pair` —— 接入你自己的设备（全权）

```
gtmux pair                  # 生成一个一次性配对码，三种形态一起打印：
                            #   手机二维码 · 浏览器 https://…/#c=<码> ·
                            #   给另一台终端的一行 `gtmux attach '<url>/#c=<码>'`
gtmux pair list             # 你已配对的设备（访客住在 `gtmux share` 那边）
gtmux pair revoke <id>      # 立即切断某台设备
```

PAIR 是 pair/share 模型里的 owner 轨：接进来的设备**就是你** ——
所有会话上完整的可见 + 可输入，和坐在 Mac 前面一样。三种介质兑换的是同一个短命配对码
（5 分钟，一次性），进的是同一份可吊销的名册。终端那一种会把 token 持久化到
`~/.config/gtmux/remotes.json`（0600），所以之后裸敲 `gtmux attach <地址>` 就能用；
`pair revoke` 立刻让它失效。`gtmux tunnel` 开着的时候链接带的是隧道地址，
否则是局域网地址。`gtmux devices` 仍然是这份名册的别名。

### `gtmux devices --push` / `--forget-push` —— 查看与清理推送 token

```
gtmux devices --push                          # 名册上标出每台设备的推送 token
                                              #   （✓ env·kinds），以及任何未绑定的 token
gtmux devices --forget-push <id|orphans|all>  # 删掉推送 token（仅主机）
```

推送 token 绑在注册它的那台已接入设备上，所以 `gtmux devices revoke <id>`
本身就已经停掉了那台设备的通知。`--push` 显示这个绑定关系；
`--forget-push` 按选择器清理：一个设备 `id`、`orphans`（只清未绑定的历史 token，
来自还没有设备绑定的时代）、或者 `all`。用 `orphans` 清掉那种从一个从没注销过的旧 app
遗留下来的孤儿 token（就是「删掉的手机还在收通知」那种情形）。
仅主机可用（本地主 token），远端设备/访客会被拒绝。

## `gtmux share` —— 给协作者的受限、可吊销访问

```
gtmux share new --label Alice --view %1,%2 --type %1 --expires 24h
gtmux share set a1b2c3d4 --type %2            # 只改**一条**链接（没给的参数不动）
gtmux share link a1b2c3d4 [--json]            # 重新显示某条已有链接的地址（+ 二维码）
gtmux share on|off                            # 所有访客输入的总同意开关
gtmux share status [--json]                   # 每条链接的范围摘要
gtmux share revoke a1b2c3d4
```

SHARE 是 pair/share 模型里的协作者轨（PAIR 是你自己的设备、全权；SHARE 是一条访客链接、最小权限）。
**每条链接带自己的范围**：访客可以**看**哪些 pane（`--view`）、可以**输入**哪些
（`--type`，必须是 view 的子集），外加一个可选的过期时间
（`--expires 45m|24h|7d`，默认永不过期；过期的链接和被吊销的一样认证失败）。
输入还额外需要主人的同意（`share on`，默认关）。这一切都由服务端强制，
网页和 app 只是把它镜像出来。

一条不带 `--view/--type` 生成的链接，会复制当前的全局列表（那是模板）。
历史上的全局写法（`share add/remove`、`share view add/remove/clear`）仍然能用，
但会**扇出**到每一条已有链接 —— 要给单条链接裁剪，请用 `share set`。
`status --json` 带每个访客的 `view_panes`/`panes`/`expires_at`，永远不含裸 token。
它还会报告**谁用过这条链接**：`last_seen`、`platform`（`Chrome 141 · macOS`）、`last_ip`，
在有人用过之前它们都不存在，而这本身就是答案。一条链接是交到别人手上的常驻授权，
所以「有没有人走过这道门、从哪儿走的」才是值得回答的问题；
它**允许**什么，上面那一行已经写了。链接地址在生成时打印一次；之后想再复制，
用 `gtmux share link <id>`（或者菜单栏那行的复制按钮）——
它会把同一条 `#g=` 地址再给你一次（只对全权调用方，访客不行）。

## `gtmux whatsnew` —— 对**你**来说变了什么

```sh
gtmux whatsnew                 # 比你正在跑的版本新的全部
gtmux whatsnew --since v0.36.0 # 从某个版本起
gtmux whatsnew --all           # 我们有记录的所有发布
```

每个版本里**写给用户**的那几行，不是 commit 标题。`gtmux update` 装完之后会自动打印
开头几条，这里是完整列表。

来源是发布 **tag 信息**里的一个 `user:` 块，goreleaser 会把它拷进 release 正文。
可选的 `user-zh:` 孪生块用中文写同样的内容：

```
git tag -a v0.40.0 -m "v0.40.0 — …

user:
- spawn --title now names the session
- restore returns you to the window you were on

user-zh:
- spawn --title 现在会命名会话
- restore 会把你带回原来的窗口
"
```

`gtmux update` 和 `gtmux whatsnew` 都会打印与你语言匹配的那一块（`GTMUX_LANG`）：
中文优先 `user-zh:`，英文优先 `user:`，某个 tag 只写了一块时另一种语言回退到它 ——
所以只有单块的老 tag 照样能用，不管当初是用哪种语言写的。两块的先后顺序随意，
每一块在空行、标题行，或者另一块的标记处结束。

一个没有 `user:` 块的发布什么都不贡献，这是刻意的。
一个对用户来说什么都没变的版本，应该什么都不说，
而不是把一句 commit 标题改写成开发者词汇。

## `gtmux config` —— 少数几个不是每次运行都给的设置

```
gtmux config agent-proxy [<url>|off]   # gtmux **拉起** agent 时用的代理
gtmux config tab-alert  [on|off]       # 给有 agent 在等你的会话，在终端**标签页**上打标记
gtmux config lang       [en|zh|auto]   # 机器级输出语言（auto = 跟随系统 locale）
```

不带参数调用时，每一条都打印它当前的值。

### `lang` —— 让每一个 gtmux 进程说同一种语言

进程之间不共享环境：launchd 起的 `gtmux serve` 和一个 hook 子进程，
既没有 `GTMUX_LANG` 也没有你 shell 的 locale，所以没有机器级选择的话，
它们发出的唤醒后缀和桌面通知，可能和你终端里显示的语言不一致。
`gtmux config lang zh` 给它们一次性做了这个选择；`auto` 则跟随系统 locale。
按进程的 `GTMUX_LANG` 和按次调用的 `--lang` 仍然优先。

### `tab-alert` —— 不用把九个标签页都点开就找到对的那个

一个 tmux 会话一个终端标签页的话，每个标签页标题长得都差不多，
想知道哪个 agent 卡在你这儿，只能一个个去看。`tab-alert on` 会在任何有 agent 在等待的
会话的标签页标题前面放一个 **●**。

- **只有 `waiting` 会标。** `working` 和 `idle` 永远不标。
  一个大部分时间出现在大部分标签页上的标记，是没人会读的标记 ——
  这和红色只留给决策是同一个道理。
- **与终端无关。** 标题由 tmux 渲染并推给终端，终端只负责显示，
  所以 Ghostty、iTerm2、Warp、Apple Terminal 上都一样有效。
  它是一个**字形**，不是颜色：给标签页上色是各家终端各自的能力，带不走。
- **保留你的标题格式。** 打开时它会读你的 `set-titles-string` 存下来，
  只在前面接上自己那一段；`off` 会原样还原它当初读到的东西。
  如果你在这期间改过格式，`off` 会**拒绝**，而不是拿一份过期的快照覆盖你的修改 ——
  那种情况下请自己把开头的 `#{@gtmux_alert}` 删掉。
- **由 agent 自己的 hook 事件驱动**，不是靠读屏：agent 一报告等待，标记立刻落下，
  serve 的节拍做兜底对账。中控不在这个回路里 ——
  这是对 gtmux 已有状态的一次机械投影，不是一个判断。
- 也可以在菜单栏 app 的**偏好设置 → 通知**里开关。

### `hqWake` —— 调中控的唤醒通道

`~/.config/gtmux/config.json` 里 `"hqWake"` 下面手写的键（全部可选；
缺失或非法的键保持默认）。每一项管什么，写在对应行为所在的地方：
[唤醒通道](#唤醒通道--中控是怎么知道事情的)、
[消费水位](#消费水位--为什么不会有东西丢掉)、
[自轮换](#自轮换--当中控自己的会话成了问题)：

| 键 | 默认 | 管什么 |
| --- | --- | --- |
| `done` | `"unattended"` | done 唤醒模式：`unattended` \| `always` \| `tick` |
| `paneMinGapSec` | 120 | `done` 唤醒的按 pane 合并窗口（秒） |
| `tickMinutes` | 10 | 摘要节拍的最小间隔 |
| `tickBurst` | 5 | 攒到多少个结果就提前触发节拍 |
| `unreadDebounceSec` | 120 | 未消费事件要压多久才发一次 `unread` 敲门 |
| `unreadRepeatSec` | 300 | 水位不动时 `unread` 的重敲间隔 |
| `selfRotateCtx` | 0.75 | 上下文占比的越界线（0 关闭） |
| `selfRotateHours` | 12 | 会话年龄的越界线（0 关闭） |
| `selfRotateTurns` | 300 | 中控回合数的越界线（0 关闭） |
| `selfRotateRepeatSec` | 1800 | 越界持续期间的重敲节奏 |
| `selfRotateFloorSec` | 43200 | 一个没有变化的越界最长能沉默多久 |
| `selfRotateCheckSec` | 300 | 自轮换传感器多久评估一次 |

## tmux 集成

gtmux 就是一个 CLI，你在 `tmux.conf` 里想绑什么键都行。推荐：

```tmux
set -g set-titles on
set -g set-titles-string '#S — #W'
bind g run-shell -b "gtmux overview --popup"
bind a display-popup -E -w 80% -h 60% "gtmux agents --watch --popup"
bind J run-shell "gtmux focus --last"
```

### `gtmux status` —— 把舰队放进 tmux 状态栏

```tmux
set -g status-right '#(gtmux status)'
```

一小段带色的计数 —— `●2 ✓14` —— 给 tmux 状态行用，
于是你本来就有的那条栏会主动告诉你谁需要你，不用你去问。
`--plain` 去掉 tmux 的颜色转义，给任何别的状态栏（或者 shell 提示符）用同样的数字。

### 把 pane id 放进标签页标题（可选）

gtmux 在每一块屏上都用 tmux 的 id 称呼一个 pane —— `%23` —— 而 `gtmux focus %23`
直接吃这个 id。两行配置能让这个 id 在**另一边**也可见，
于是 app 里的一行和你终端里的一个标签页说的是同一个东西：

```tmux
set -g automatic-rename-format '#{b:pane_current_path} #{P:#{pane_id} }'
set-hook -g pane-exited 'set-window-option automatic-rename off ; set-window-option automatic-rename on'
```

窗口名于是会列出那个窗口里的**每一个** pane（`gtmux %23 %24`），
而因为 `set-titles-string` 是 `#S — #W`，标签页会继承它 ——
标题格式没有变，所以 `focus` 的标签页匹配不受影响。

- **`#{P:…}` 遍历的是这个窗口的 pane**，所以它不跟随焦点。
  第一版设计用的是活动 pane 的 id，被实测否掉了：tmux 按它自己的节奏重算格式，
  于是那个名字落后于真实的活动 pane —— 一个打扮成实时的过期指针。
- **那个 hook 是必需的，不是装饰。** 加一个 pane 会立刻重算窗口名；关掉一个**不会**。
  没有这个 hook，你的标签页会一直宣传一个已经不在的 pane，
  那比一个 id 都不显示更糟。
- **在分屏窗口里**，如果你还想让每个 pane 在屏幕上也戴着自己的 id：

  ```tmux
  set -g pane-border-status top
  set -g pane-border-format ' #{pane_id} #{pane_current_command} '
  ```

  `doctor` **不会**建议这一条。`pane-border-status` 默认是 `off`，
  打开它的代价是每个分屏里每个 pane 永久占掉一行屏幕 ——
  为一件窗口名已经承载了的事情付出真实的价钱。你自己定，这不是推荐。

### 让打印出来的链接可点（可选）

程序把超链接打印成一个 OSC 8 转义，而 **tmux 只把它转发给声称支持这个能力的终端** ——
默认没有终端声称，所以 tmux 会把这个转义丢掉，链接渲染成纯文本。

```tmux
set -as terminal-features ',*:hyperlinks'
```

- **需要 tmux 3.4+。** `terminal-features` 从 3.2 起就有，但 `hyperlinks` 这个 feature
  是 3.4 才有；在更老的 tmux 上这一行是启动错误，所以 `doctor` 会检查正在跑的版本，
  低于它就不建议这一条。
- **只有生效之后打印的输出**才带链接，屏幕上已经画出来的东西当初就没有。
- Ghostty 和 iTerm2 都处理 OSC 8。加了之后链接还是点不动的话，
  下一个嫌疑是终端自己的点击修饰键（有些要 ⌘ 或 ⌥ + 点）。

### 给普通 pane 一个值得读的名字（可选）

agent 会自己写 pane 标题，这就是为什么一行 agent 读作 `提炼本周研发周报质量部分汇总`，
而旁边的 shell pane 读作 `bash`。shell 什么都不写，于是 gtmux 回退到命令名，
每一行普通 pane 看起来都一样。

两个 shell hook 能解决：跑命令时标题是那条命令，回到提示符时是目录名。

```bash
# bash —— 写进你**登录** shell 真正会读的那个文件（见下）
if [ -n "$TMUX" ]; then
  trap 'printf "\033]2;%s\007" "$BASH_COMMAND"' DEBUG
  PROMPT_COMMAND='printf "\033]2;%s\007" "${PWD##*/}"'"${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
```

```zsh
# zsh —— ~/.zshrc
if [ -n "$TMUX" ]; then
  autoload -Uz add-zsh-hook
  gtmux_pane_title_preexec() { print -rn -- $'\e]2;'"$1"$'\a' }
  gtmux_pane_title_precmd()  { print -rn -- $'\e]2;'"${PWD:t}"$'\a' }
  add-zsh-hook preexec gtmux_pane_title_preexec
  add-zsh-hook precmd  gtmux_pane_title_precmd
fi
```

- **不是 `~/.bashrc`。** 一个 tmux pane 跑的是**登录** shell，而登录 bash 读的是
  `.bash_profile` / `.bash_login` / `.profile` 里第一个存在的那个，**从不**读 `.bashrc`。
  写进 `.bashrc`（大多数教程都这么写）在 tmux 里什么都不会发生。
  `doctor --fix` 会挑你的 shell 真正会读的那个文件，而且只往**已经存在**的文件里追加
  （在你用 `.profile` 的机器上创建一个 `.bash_profile`，会把它盖住）。
- **用 `add-zsh-hook`，不要裸的 `preexec()`。** 定义那个函数会替换掉你（或者 oh-my-zsh）已有的。
- **用 `$TMUX` 把关。** 在 tmux 之外，这会和你终端自己的标签页标题打架，
  而 gtmux 正是靠那个标题来跳转的。
- **`gtmux doctor` 量的是结果，不是配置**：它数有多少个普通 pane 的标题说出了点什么，
  因为一个 hook 有一百种写法，只有结果才算数。只对新开的 shell 生效，
  已经开着的 pane 保持原标题。

- **`gtmux doctor` 会报这一行，`--fix` 会主动提议** —— 作为一个由你拍板的建议，
  绝不是一次无声的写入。如果你已经有自己的 `automatic-rename-format`，
  `--fix` 会把 id **追加**上去，而不是替换掉。gtmux 不会去重命名你的窗口：
  `rename-window` 会把那个窗口的 `automatic-rename` 关掉，并永久覆盖你的格式。

## 通知 hook

`⏸ waiting`、`✓ latest` 和「点通知跳过去」都依赖一个把状态文件写到
`~/.local/share/gtmux/` 下的 hook。gtmux 内置了这个 hook，不需要外部脚本：

```sh
gtmux install                       # 问你：hooks | app | all
gtmux install hooks                 # Claude，一次性配置（macOS）
gtmux install hooks --agent codex   # 或者 cursor|gemini|copilot|kiro|opencode
gtmux uninstall [hooks|app|all]     # 反过来卸掉（不给目标就问你）
```

`install hooks` 把 `gtmux hook` 注册到 `~/.claude/settings.json` 的
`Stop`、`Notification`、`UserPromptSubmit` 三个事件上（幂等；保留其它 hook，并给文件留备份）。
`gtmux hook` 是产出方 —— 由 Claude Code 来跑，不是你来跑 ——
它纯粹按事件**时序**写状态，不读消息文本就能把一次权限请求和一次空闲提醒区分开。

**其它 agent：** `--agent codex|cursor|gemini|copilot|kiro|opencode` 改成接那个 agent
自己的 hook 文件。**Codex** 用的是它可叠加的 hooks 系统
（`~/.codex/hooks.json` + `features.hooks`），所以它**与你已有的 `notify` 共存**
（比如 computer-use），而不是替换掉。**opencode** 没有命令 hook 文件，
所以 gtmux 装一个转发它事件的小 JS 插件（`~/.config/opencode/plugin/gtmux.js`）。
`gtmux doctor --fix` 会针对它探测到的 agent 逐个提议接上。

通知由菜单栏 app 投递，不需要 `terminal-notifier`。hook 把请求排到
`~/.local/share/gtmux/notify/` 下，`Gtmux.app` 弹一条原生横幅
（显示为 **Gtmux**，带 agent 图标和一个 **Jump** 动作；「跑完了」是安静的，
「需要你输入」会响）。点它就落到那个确切的 pane 上。
第一次运行时请允许通知，并让 app 一直开着才收得到。

## 权限

gtmux 只要它需要的：

- **自动化（控制你的终端 —— Ghostty / iTerm2 / Warp）** —— `focus` / `restore` / `new`
  和点通知跳转需要它。gtmux 第一次通过 AppleScript 驱动终端时，macOS 会弹窗，点**允许**。
- **通知** —— 菜单栏 app 才能弹 agent 横幅。第一次运行时允许。
- **开机自启**（可选）—— 只在你到偏好设置里打开时才要。

下面这些它**不需要** —— macOS 要是弹了，你可以放心**拒绝**，不会损失任何功能：

- **App 管理（「修改你 Mac 上的 app」）** —— gtmux 从不修改别的 app，
  它的代码只会碰自己的 bundle（更新/卸载时）。这个弹窗可能出现在 macOS 通过
  responsible-process 链，把**另一个** app 的自更新归因到 gtmux 那个长跑的后台进程上时。
  拒绝对 gtmux 没有任何影响。
- **文件与文件夹（下载 / 桌面 / 文稿）** —— gtmux 不读这些。
  这个弹窗可能出现在 `restore` 重建一个工作目录恰好在其中之一的 tmux 会话时 ——
  那是 `tmux`（由 gtmux 启动）在打开那个文件夹。可以放心拒绝。
