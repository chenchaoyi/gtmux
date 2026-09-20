# CLI 与命令

[English](cli.md) · **中文**

| 命令 | 做什么 |
| --- | --- |
| `agents [--watch\|--json]` | 你所有 pane 里的 coding agent：谁在等你、谁在跑、谁空闲，在哪儿，跳过去用哪个 pane id |
| `panes [--json]` | 每一个 tmux pane（不只是 agent），分 agent / 普通两档，是 pane 浏览器背后的全集 |
| `overview [--popup]` | session / window / pane 汇总；`--popup` 适配 tmux 弹窗尺寸 |
| `restore [--pick\|--one\|<name>\|--dry-run\|--plan[ --json]] [--resume-agents=auto\|type\|off]` | 每个 session 一个终端标签页并全部接回；可选地把记下来的 agent 对话也重新拉起；`--plan` 先看会恢复出什么 |
| `focus <name\|pane-id\|--last>` | 跳到某个 session 的标签页；给 pane id（`%N`）就落到那个 pane；`--last` 是最近刚跑完的 agent |
| `new [name]` | 新建一个 tmux session，并开一个终端标签页 |
| `adopt <session_id>…` | 把感知到的非 tmux（native）agent 会话转进 tmux |
| `doctor [--fix [--yes] \| --bundle]` | 按主题分组的体检；在 TTY 上会当场问你要不要修可改进的项；`--fix` 是一站式配置（hook、set-titles、重启恢复、菜单栏 app）；`--bundle` 打包一份问题报告 |
| `install [hooks\|app\|all]` | 装 gtmux 需要的东西；不给目标就问你。`install hooks --agent codex\|cursor\|gemini\|copilot\|kiro\|opencode\|kimi` 接入另一个 agent |
| `uninstall [hooks\|app\|all]` | 反过来卸掉；不给目标就问你（两者后果差很远） |
| `serve [--port N]` | 给手机 app / 网页镜像用的只读 HTTP+SSE 雷达（放在 VPN 或隧道后面） |
| `tunnel [--backend cloudflare\|self] [--quick] [--service] [--redeem <code>]` | 把雷达开到任意网络：Standard（Cloudflare）或 Direct（自托管 / 付费），见 [phone.zh.md](phone.zh.md) |
| `pair [list\|revoke <id>]` | 接入你自己的设备（全权）：一个一次性配对码，手机扫、浏览器开，或者一行 `gtmux attach` |
| `share [new\|set\|link\|on\|off\|revoke <id>\|status]` | 给协作者的受限、可吊销链接，每条链接单独的可见 / 可输入白名单（见下） |
| `attach <host\|pair-link\|share-link> [%pane]` | 把远端 tmux pane 的 PTY 经 serve 的 WebSocket 接到你本地终端（owner 或访客） |
| `devices [revoke <id>\|--push\|--forget-push <id\|orphans\|all>]` | 已配对设备清单（`pair list`/`pair revoke` 的别名）；`--push` 查看、`--forget-push` 清理推送 token |
| `app`（别名 `menubar`） | 启动菜单栏 app（`Gtmux.app`） |
| `update [--check\|--cli-only]` | 自更新 CLI + 菜单栏 app |

直接敲 `gtmux` 出的是一屏：你会用到的所有命令，按跑起来会对这台机器做什么分组——
哪些只读、哪些会动你的终端、哪些会往 pane 里写字、哪些会开端口、哪些会改这台 Mac。
`gtmux <命令> --help` 只印那一个命令和它的参数，每个参数会写清能填什么、什么会被拒、
必须跟谁一起用。`gtmux --help --json` 是同一张表的数据版，给读它的程序用：每条命令带
分组、中英两半、以及会不会改东西，每个参数带 `values`、`max_bytes`、`requires`。
`gtmux --version` 打印版本。

输出语言依次看 `--lang=en|zh`、`$GTMUX_LANG`、`gtmux config lang`，都没设就看系统
locale（`LC_ALL`/`LANG`：`zh*` 出中文，默认 `en`）。所有东西都是显式调用，不装
shell hook，任何 shell 都能用。

## `gtmux agents`

列出你 tmux pane 里在跑的 coding agent，按紧急程度排序。

```
gtmux agent · 7 agent · 1 等输入 · 2 运行中 · 4 空闲

‖ 等输入   Claude Code  api:0.0                permission to run tests %7
⠿ 运行中   Claude Code  hq:0.0                 api is waiting on you · rest normal %1
⠿ 运行中   Claude Code  web:0.0                refactor auth middleware %11
✓ 空闲     Claude Code  app:0.0                wire up the dashboard %9
✓ 空闲     Codex        worker:0.0             add retry backoff %8  ✓ 最近完成
✓ 空闲     Gemini       docs:0.0               draft the API reference %3
● 运行中   Claude Code  infra:0.0              — %5

跳转：gtmux focus <pane>   （例如 gtmux focus %7）
```

每行是状态 · agent · 位置 · 任务 · pane id。

- ‖ waiting（红）：干到一半，卡在你这儿等一个授权或批准；永远排最上面，也是这屏上唯一
  表示「现在就得动」的颜色。
- ⠿ working（青）：在忙，别打扰。
- ✓ idle（绿）：这一回合结束了，你想动的时候再动，不急。
- ● running（灰）：这个 pane 里没有 agent 的回合可言，就是个普通 shell。

这几个记号特意选的是文本呈现的字符。被它们换掉的 `⏸` 和 `✳` 带 emoji 呈现：终端可以用彩色
emoji 字体去画它们，那样你给的颜色会被忽略，红色只落在「waiting」这个词上，落不到旁边
那个记号上。
- ⚠ errored（琥珀色）：一个空闲会话，但结束在 API 或工具报错上（比如
  `Unable to connect to API`），没有干净地跑完。它仍然算空闲（该你动），行上带错误摘要。
  `--json` 里是 `error: true` 加 `error_text`。

`gtmux agents --watch` 是自动刷新的实时看板（用
[bubbletea](https://github.com/charmbracelet/bubbletea) 做的）：约 1.5 秒一轮，
↑/↓ 选择，Enter 跳到那个 pane，r 刷新，q 退出。它按「等你 · 运行中 · 空闲 · 只有 shell」
分组，带一列「有多久」，最后一行是额度：最紧的三个窗口，用满的那个标琥珀色。终端变窄时
列会依次让位（先是 agent 名字，再是任务），所以任何一行都不会折。`--json` 输出同样的数据，
给脚本和菜单栏 app 用。

### 它是怎么认出来的（不只支持 Claude）

- 状态取自 agent 自己写的 pane 标题。开头是盲文旋转符（`⠋⠙⠹…`，大多数 agent TUI
  都在转这个）就是 working；Claude Code 屏幕上那个 `✳` 是 idle。
- 是哪个 agent，靠前台命令匹配（`claude`、`codex`、`gemini`、`cursor`、`opencode` …），
  或者靠标题里的名字。
- 用 `~/.config/gtmux/agents.json` 扩展或覆盖：一个 `{"name","commands","idleGlyph"}`
  的 JSON 数组，你写的优先于内置。
- 只有 agent 进程真的在跑，那个 pane 才会被列出来。普通 shell 顶着残留的 agent 标题
  （比如 resurrect 恢复出来但从没重新拉起的会话）不算。
- 跑在 tmux 之外的 agent（终端里裸跑的 `codex`/`claude`）通过同一个 hook 被只读感知，
  列在「不在 tmux」分区里，`source:"native"`。它们没有 pane，不能跳也不能回；
  能 resume 的可以用 `gtmux adopt <session_id>` 拉进 tmux。

`‖ waiting` 和 `latest` 来自[通知 hook](#通知-hook)写的状态文件。没装 hook，
agent 永远不会显示 `‖`，其余功能照常。

## `gtmux panes`

`gtmux panes` 列出每一个 tmux pane，有没有 coding agent 都列。它是 pane 浏览器背后那个
只读的数据源：一棵 session → window → pane 的树，或者用 `--json` 拿结构化数组。
每个 pane 带一个 `tier`：`"agent"`（coding agent 的 pane，判定方式和雷达一致）或
`"plain"`（shell、编辑器、dev server、日志，其余都算），另外还有位置、cwd、当前命令、
标题、是否活动、是否在 copy-mode。

```sh
gtmux panes            # session → window → pane tree; ▸ marks agent panes
gtmux panes --json     # [{pane_id, loc, session, window, pane, cwd, command, title, active, in_mode, tier, agent}]
gtmux panes watch %N   # opt a PLAIN pane onto the radar as a watched row
gtmux panes unwatch %N # remove it
gtmux panes --watched  # list watched pane ids
```

`gtmux agents --json` 是一份锁死的契约，含义就是「coding agent」；`panes` 是给能够到
任意 pane 的浏览器用的全集。`gtmux focus`、`send`、`attach` 接受任何 pane id，所以
`tier:"plain"` 的 pane 也是一等的 focus、输入和 attach 目标，只是拿不到那些只对 agent
成立的智能（digest、1/2/3 批准、派活、HQ）。

分档能力对照：

| 档 | 在雷达上 | 查看/抓屏 | focus/跳转 | 输入/send | attach | 智能（digest / 1·2·3 / 派活 / HQ） |
|---|---|---|---|---|---|---|
| agent（tmux） | 自动 | ✓ | ✓ | ✓ | ✓ | ✓ |
| 普通 tmux pane | 手动挂（`panes watch`） | ✓ | ✓ | ✓ | ✓ | — |
| 感知到的非 tmux agent | 「不在 tmux」 | — | — | — | — | —（只读） |

普通 pane 只有你用 `gtmux panes watch %N` 主动挂上去才会出现在 agent 雷达上，
作为一条独立的关注行（没有 agent 状态），pane 关掉就自动摘掉。访客的分享范围
在任何 pane 上同样把关可见与可输入。

## `gtmux digest` + `gtmux hq`：HQ，也就是中控会话

`gtmux digest` 把整支舰队连同含义一眼摆出来，是一张排好列的表：

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

先是一行按状态的计数，然后每个状态一节（需要你、working、completed，有报错的话
最后是 errored）。每行是状态字形 · 名字 · 目标/最近/在问什么 · 右侧徽标
（派活状态 / 选项个数 / 用量告警） · 相对时间。每个字段都是确定性拼出来的，零 LLM
token：goal 是这个会话最后一条用户提示，last 是它最后一条回复的尾巴（两者都来自
agent 自己的 transcript），asks 是等待提示里解析出来的选项。`--json` 输出机器形态
（也由 `GET /api/digest` 提供）。gtmux 没有 transcript 的会话，仅凭雷达信号也照样渲染。
JSON 每行都自报感知档位 `sense`：`driver`（agent 的 hook 供状态，transcript 供
goal/last）、`partial`（hook 通了但没解出结构化内容）、`screen`（纯抓屏和进程推断）。

`gtmux hq` 打开 HQ（中控，你的监督会话；已经在跑就聚焦，绝不重复起）：你的 coding
agent 跑在 `~/.config/gtmux/hq/` 下一个专属 tmux 会话里，第一次会种下一份说明书：
读 `gtmux digest --json`，做判断，值得时才钻进某个 pane（`tmux capture-pane`），
用 `gtmux send` 驱动，向你汇报。说明书是 `AGENTS.md`，配一个 `@AGENTS.md` 导入的
`CLAUDE.md`，所以 HQ 可以是任何 CLI agent。它按你的语言下发（`GTMUX_LANG` en/zh），
版本变了会用同一种语言重新生成（旧文件留备份，LOCAL.md 不动）；只有
`gtmux hq --lang en|zh` 能换语言。

全新启动且没给 `--agent` 时，`gtmux hq` 会问你用哪个已装好 hook 的 agent，并记住选择；
`gtmux hq --agent codex` 或 `GTMUX_HQ_AGENT` 直接指定，脚本里不弹问、走默认。
个性化写在同目录的 `LOCAL.md`：优先级、汇报口味、免打扰时段。这个文件扛得过每一次
说明书升级；直接改受管的 `AGENTS.md`，改动会被挪进备份。HQ 在那个目录里记的笔记
跨会话保留。雷达里它那一行带 `role:"supervisor"`。

在哪儿跑。不带参数时，`gtmux hq` 沿用 HQ 已有的窗口：在跑就切过去，退出了就在原窗口
重新拉起，两者都没有才自己建一个 tmux 会话并开标签页。跑过 HQ 的窗口带着标记，
所以在任何地方敲 `gtmux hq` 都会把 HQ 拉回那里。要换地方：`--pane %N` 在那个 pane 里
启动（得是空着的 shell，会先 `cd` 到 HQ 目录），`--here` 在你正敲命令的这个 pane 里，
`--new-pane` 在当前窗口拆一个新 pane。三者都会把 HQ 的身份挪到新 pane；只要有 HQ
在跑，三者都拒绝，HQ 只有一个。

在你自己正在用的这个 pane 里启动 HQ 是特殊情况：这时终端在 gtmux 手里，所以它不往自己
这个终端里打字，而是直接把终端交给 agent；启动简报改由一个后台进程在 agent 的输入框
出来之后送进去。这期间不会往 pane 里打任何东西，以前简报卡在输入框里没发出去就是这么来的。

`gtmux hq --board [--json]` 打印态势板，不打开 HQ（菜单栏 app 的态势板阅读器就用它）。
从没写过态势板时返回 `exists:false`。

`gtmux hq --home` 打印 HQ 目录的路径。`gtmux knowledge` 的写操作只接受从那个目录发起的
调用，所以提供这些操作的端要先到那儿再执行动词。还没有 HQ 目录时它照样打印路径，
并以非零退出。

`gtmux hq --rotate` 就地把在跑的 HQ 会话退役、换一个新的：它找到 HQ 的 pane，把那个
agent 自己的重置命令敲进去。`self-rotate` 的敲门说会话磨损了之后，HQ 的说明书会不等
你吩咐就执行它（见[自轮换](#自轮换hq-自己的会话成了问题)），而且先把 `notes/board.md`
和知识库更新到位。没有 HQ 在跑就没什么可轮换的，它会这么说。

### 唤醒通道：HQ 怎么知道事情

决策密度高的事件会往活着的 HQ pane 里敲进一行信号。格式固定，也刻意不像对话，
一屏信号扫一眼就能读：

<!-- gtmux:rendered wake-lines -->
```
» ◆ gtmux·waiting·permission  api:0.0 (%7) │ title:"run the tests?"
» ▸ gtmux·done  web:2.0 (%11) │ 3m │ goal:"fix the login bug" │ tail:"tests pass" · #a3f1c2
```

`» <grade> gtmux·<class>  <loc> (<pane>) │ <field> │ …`，其中每一段由 agent 或用户
写出的内容都加引号并带标签（`title:` / `goal:` / `tail:` / `ask:` / `err:`）。
它是 HQ 转述的数据，永远不是它要照办的指令。

等级打头、位置固定，所以一屏信号先按分量读、再按字读：`◆` 决策（需要你：不可逆、
代价大，或者对方明确在问）、`▸` 注意（某条线卡住了，或者变化到值得你知道）、
`·` 台账（记账：记下来、可拉取，不值得打扰）。等级由类别决定，只说这一行该读得多响。
类别如下：

| 类别 | 等级 | 什么时候发 |
| --- | :---: | --- |
| `waiting·<kind>` | ◆ | 一个 agent 卡在你这儿（授权 / 计划 / 提问） |
| `resolved` | ▸ | 那个等待解除了：你在 pane 里回了，或者 agent 自己继续了；HQ 会撤掉过期的追问 |
| `asks` | ◆ | 回合末尾的回复里问了个问题，但没有菜单（只看菜单的传感器会漏掉） |
| `done` | ▸ | 任何会话干完活进入空闲，不限于派出去的任务。完成发生在你正看着的那个 pane 里就抑制（`hqWake.done`：默认 `unattended` \| `always` \| `tick`），并按 pane 合并限流 |
| `crash` | ◆ | 这一回合死在 agent / API 报错上，绝不会被读成「完成」 |
| `goal-changed` | ◆ | 你直接往某个 agent 自己的窗口里提交了提示（包括斜杠命令），HQ 于是感知到一件不是它派的活 |
| `new-session` | ▸ | 新感知到一个 agent pane，去建联 |
| `reap-suggest` | ▸ | 某次派活看起来可以回收了，行里带着可直接用的 `gtmux reap <id>` |
| `stuck·waiting` | ▸ | 一个 pane 等你超时了。每次等待只发一次，而且只在是 agent 在问时才发（gtmux 仅凭屏幕推断出来的等待永不升级） |
| `resource·warn` / `limits·warn` | ▸ | 机器 / 订阅越过了某条线（有阻尼，见 `gtmux resource`） |
| `usage·warn` | ▸ | 某个会话越过了上下文 / 消耗的某一层（见 `gtmux usage`） |
| `wake-degraded` | ◆ | 感知本身坏了：唤醒不再落到 HQ pane 上 |
| `tunnel` | ▸ | 远程访问变了：`down`（手机连不上这台 Mac，附隧道自己报的错误）或重新 `up`。读的是隧道自己的状态，只在切换时发，并记入事件流 `gtmux:tunnel` |
| `tick` | · | 周期简报，只在真的有变化时才发（安静的那一轮零成本） |
| `distill` | · | 该做一轮知识沉淀了（约每周一次；`gtmux capture` 攒够 5 条候选会提前）：HQ 把这段时间的教训折进知识库，并剪掉过期的 |
| `self-check` | · | HQ 该做自己的家务了（约每天一次）：清理陈旧的待办，检查记忆和日志健康 |
| `unread` | · | 有事件压在 HQ 的消费水位之外：给一个计数和拉取游标，不对重要性下判断 |
| `self-rotate` | ◆ | HQ 自己的会话磨损了（上下文 / 时长 / 回合数）：它交接完自己轮换，见下 |

要等一个动作才能清掉的类别（`self-rotate`、`unread`、各种 warn）在重复之前会自我复核：
前提集合和世界都没变的重复会被抑制，一有漂移就重新武装，另有一条安全下限保证常驻的债
不会被忘掉。排队中的唤醒在敲进去之前会重新采样，前提已经不成立的会被丢弃。

`distill` 和 `self-check` 是维护类：以最低优先级敲门，永远不插到卡住的 agent 前面，
两轮都只在 HQ 真的做了事情时才出声。它们也是仅有的纯靠时钟触发的类别；HQ 自己没有
定时器，所以是 `gtmux serve` 让它们发生。

其余一切都在拉取侧：HQ 醒来，自己读 `gtmux events --since-seq <n>` 或
`gtmux digest`。普通的进展回合永远不碰它的屏幕。

HQ 对信号线的回复也是信号线：一行，以 `⟣` 加一个字形开头，所以它的 pane 和收件箱
是同一种扫法。图例：

| 回复 | 含义 |
| --- | --- |
| `⟣ ✅ <pane> <judgment> → <next step>` | 一次值得知道的完成 |
| `⟣ ▪ noted: <one clause>` | 例行结果，记进态势板，什么都不欠 |
| `⟣ 📓 captured: <topic>` | 一条持久经验写进了知识库 |
| `⟣ ⚠ <escalation>` | 有事需要你，按升级策略 |
| `⟣ ◈ 简报 <time> │ <counts> │ top item` | 周期简报（后面最多 5 行缩进的 `· `） |

### 消费水位：为什么不会漏掉东西

上面的类别是优先级标签，说的是先读哪个。HQ 到底能不能知道某个事件，靠的是一条
消费水位：gtmux 记录 HQ 把事件流读到了哪儿。有事件压在水位之外超过
`hqWake.unreadDebounceSec`（默认 120 秒），gtmux 就敲一次门，只给计数：

<!-- gtmux:rendered unread-line -->
```
» · gtmux·unread  7 unconsumed (%21 ×4 · %13 ×2 · control) │ pull: gtmux events --since-seq 6653 --json
```

这一行会说清它数的是什么，按来源分（`control` 是 gtmux 自己的维护触发，`native`
是非 tmux 的 agent 会话），多的排前面，超过三项折成 `+N more`。（为什么光靠类别不够，
见 [TROUBLESHOOTING](TROUBLESHOOTING.md#the-consumption-watermark)。）

这笔债只有 HQ 消费才会清，而且只有两件事算数：从 HQ 目录发起的、不带过滤的
`gtmux events --since-seq <n>`（醒来之后的日常拉取），或者一次显式的
`gtmux events --ack <seq>`。带 `--severity` 过滤的读不算（只看到子集），从水位前面
开始的读不算（跳过了中间那段），检测到序号断裂的读也不算（有事件在没被读之前就轮转
掉了）：拉取会重新告警，敲门行带上 `· sequence gap` 标记和一条重建提示（先
`gtmux digest --json`，再显式 `--ack`），直到 HQ 刻意 ack 过去。在那之前，敲门每
`hqWake.unreadRepeatSec`（默认 300 秒）以常驻优先级重复一次。

三类记录不计入计数：HQ 自己写的行；没有 pane 的生命周期闪烁（一条没有 pane 的
`SessionStart`，几秒内就跟上了 `SessionEnd`）；gtmux 自己的审计轨迹（`gtmux:audit:*`：
每批唤醒送达或丢弃及原因、每次 `gtmux send` 的结算、回收、HQ 会话的轮换链）。
闪烁规则认的是 Start/End 配对，不只认空 pane：native（非 tmux）agent 的回合和 gtmux
非审计的 `gtmux:*` 触发同样没有 pane，照样计数，而且对它们来说这条敲门是唯一的通道，
因为按类别的唤醒都要求有 pane。HQ 自己的拉取给出的也是这个集合；`--all` 拿回原始视图，
两者都算消费。日志里什么都不会被删。

`gtmux doctor` 的 HQ 维护一节报告这个滞后（`event consumption`），落后 20 条以上或
30 分钟以上就标出来。

投递会守住你的草稿，并自己确认：一行永远不会被敲进非空的 HQ 输入框（它会排到磁盘上，
等框清了再落），一批内容只有在屏幕上被看到之后才离开队列，所以失败的发送会重试。
投递因此是至少一次，行尾才有 `#<id>`：同一个 id 出现两次就是重发，HQ 的说明书要求
忽略它。每一种结果都进审计（`gtmux:audit:wake-delivered` 带完整批次，
`gtmux:audit:wake-dropped` 带原因：被挤掉 / 未确认 / 被取代），用 `gtmux events --all`
能读到。

### 自轮换：HQ 自己的会话成了问题

`self-rotate` 说的是 HQ 本身：在一个又长又接近满的会话里，HQ 会开始把自己的产出读成
来自外部的输入，而它从里面察觉不到这一点。所以由 `gtmux serve` 从外面盯着并敲门：

```
» gtmux·self-rotate  ctx 82% · 14h · 380 turns │ over: ctx 82% ≥ 75% │ board+KB current → hand off → gtmux hq --rotate
```

三个事实被感知，任意一个越线就算越界：

| `hqWake` 键 | 默认 | 量的是什么 |
| --- | --- | --- |
| `selfRotateCtx` | 0.75 | 实时上下文占比，和 `gtmux digest` 报的 `ctx` 是同一个 |
| `selfRotateHours` | 12 | 会话年龄，从 transcript 的第一条消息算起（`serve` 重启也归不了零） |
| `selfRotateTurns` | 300 | 会话开始以来 HQ pane 上的提示提交次数 |

任意一项设成 0 就单独关掉那条判据，另外两条照常。`selfRotateRepeatSec`（默认 1800 秒）
控制重敲的节奏，`selfRotateCheckSec`（默认 300 秒）控制评估的节奏。

这笔债只有会话真的轮换了才清，判据是出现了新的 agent 会话 id；把唤醒送到不算。
过了 `hqWake.selfRotateRepeatSec` 之后，只有越界集合或舰队变了才会再敲，
`hqWake.selfRotateFloorSec`（12 小时）是什么都没变时它能沉默的上限。HQ 的说明书要求它
按顺序做三件事，不先问你：把 `notes/board.md` 和知识库更新到位，记下交接，然后跑
`gtmux hq --rotate`，把那个 agent 自己的重置命令（`/clear`，codex 是 `/new`）敲进
HQ 的 pane。轮换之后又收到 `self-rotate`，意思是那次轮换没成。`gtmux doctor` 的
HQ 对话健康一行给的是同一组数字。（这个类别背后的事故见
[TROUBLESHOOTING](TROUBLESHOOTING.md#self-rotation)。）

在 `~/.config/gtmux/config.json` 里写 `"hqNudge": false` 可以整条通道关掉（没有 HQ
pane，没有唤醒，没有开销）。唤醒只告知：gtmux 从不替另一个 agent 回答提问，从不往
TUI 里发导航键，默认策略也是让 HQ 把决策交到你面前。

## `gtmux capture`：往 HQ 知识库里丢一条便签

```
gtmux capture "<one-line lesson> @<topic>"   # topic ∈ the knowledge vocabulary: six built-ins + topics hq declared
gtmux capture --list                         # show the pending-distill queue
gtmux capture --list --json                  # the same queue, with each line's dedup key
```

```
last distill: 3d ago
2 pending-distill candidate(s):
  [pitfalls] wrangler TLS-resets from the office network — retry
```

任何 worker（不只是 HQ）都可以把一条一行的候选丢进待沉淀池
（`~/.config/gtmux/hq/knowledge/.pending-distill.jsonl`）。每行带这条经验、主题标签、
一个去重键（`<topic>/<lesson-slug>`，沉淀轮会把同键的候选合并），以及自动采集的上下文
（当前 pane、事件水位、时间戳，调用方是被跟踪的派活时还有 `$GTMUX_TASK_ID`）。
候选不等于知识库条目：HQ 的沉淀轮判断什么算持久、归到哪个主题、剪掉什么。

`--list` 开头先说队列上次什么时候被清空。攒够五条候选会把下一次沉淀提前，一条捕获到的
经验一天之内就归档，不用等满一周。`--list --json` 是同一个队列，一个数组（绝不会是
`null`），每行带主题、时间戳、来源和去重键，也就是
`gtmux knowledge dismiss --capture <key>` 要点的那个名字；驳回一个键会消掉所有共用它的
待处理行。

## `gtmux knowledge mine`：不用模型，把会话日志里的线索投进同一个队列

```
gtmux knowledge mine                 # one pass: read what the agents' logs gained, queue the leads
gtmux knowledge mine --dry-run       # show what a pass would queue, write nothing
gtmux knowledge mine --since all     # widen the correction window from 30 days to the whole stock
gtmux knowledge mine --status        # the ledger: last pass, sources, emitted, recurring errors
```

```
read 14 file(s), 3.2 MB, 61 human line(s) → 3 candidate(s) queued, 0 already emitted
  [corrections] this is wrong, do it again
      ↳ after: …moved the toggle into the header and shipped it.
  [pitfalls] bash: wrangler: command not found  (×6, 3 sessions)
```

采矿器每天跑一次（`hqWake.mineIntervalHours`，`0` 关掉）。它从上次停下的字节偏移起读
各 coding agent 的会话日志，减掉机器自己写的一切（工具输出、harness 注入块、gtmux 的
唤醒行和对着审计日志核出来的 `gtmux send` 内容、压缩摘要、粘贴），只留两种形状当线索：
人在 agent 回话之后紧接着打的那句纠正，连同那段回话的尾巴；以及规范化之后跨会话反复
出现的报错签名，连同次数。全程不跑模型。HQ 的沉淀轮用 `knowledge add --capture` /
`dismiss --capture` 逐条处理这些线索。

`~/.local/share/gtmux/mine/` 下的台账记着每个偏移和每条发过的 id，所以不会重复读、
重复投；已经写进知识库的坑再被踩到时计数照样涨，`--status` 看的就是这个。来源：
Claude Code、Codex、Kimi Code，以及 gtmux 替 opencode 写的记录。只有 Claude Code 和
Codex 记了工具输出，所以报错线索来自这两种，纠正线索四种都有。

## `gtmux quiet`：HQ 可以说多少

```
gtmux quiet on       # CRITICAL only — the quietest setting
gtmux quiet off      # the default: NORMAL and above are surfaced
gtmux quiet status   # what is in effect right now
```

HQ 会给它发现的事情定级，这条命令定的是打印给你看的下限。低于下限的东西仍然被记录，
进注意力账本（`gtmux tasks --pending`），只是不进你的屏幕，所以调高门槛失去的只有打扰。
`GTMUX_SURFACE_TIER` / `GTMUX_QUIET` 可以在单个进程里覆盖它。

有一样东西永远不会被安静掉：事件日志里的读取时断裂。那是 HQ 在告诉你它可能漏了东西。

## `gtmux knowledge`：知识台账（带来源的条目）

```
gtmux knowledge add --topic pitfalls --title "wrangler TLS-resets; retry" [--body-file -] [--capture <key>[,<key>…]] [--seq-range a..b]
gtmux knowledge supersede <id> --title "…" [--body-file -]   # replaces an entry; history stays in the ledger
gtmux knowledge retire <id> --why "…"                        # prune, with a reason that survives
gtmux knowledge dismiss --capture <key>[,<key>…] --why "…"   # reject candidates WITH a trace
gtmux knowledge promote <id> --why "…" --for <hq|machine|repo:<path>|everyone>   # WHO must know it
gtmux knowledge land <id>                                  # gtmux carries it: LOCAL.md / every agent's block / the repo file
gtmux knowledge land <id> --ref "<issue url>"              # everyone: you opened the issue the brief links to
gtmux knowledge withdraw <id> --why "…"                    # the entry was right, the promotion was not
gtmux knowledge sync [--force] [--repo <path>]             # refresh the knowledge block in each agent's instruction file
gtmux knowledge carriers                                   # each agent's instruction file and whether it is in sync
gtmux knowledge lint [--json]                              # audit: orphans, broken/outdated links, near-duplicates, stale, assumed kinds, ai-voice (reports, never edits)
gtmux knowledge neighbours <id> | --capture <key> | --text "…"   # the closest live entries; `add` shows them before writing; `capture --list` groups the pool by them
gtmux knowledge kind <id> <facts|howto|pitfalls|judgment|decisions>   # confirm or correct what an entry IS
gtmux knowledge hit <id> [--n N] [--why "…"]               # the lesson was hit again (its count is the feedback)
gtmux knowledge confirm <id>                               # a hypothesis held up
gtmux knowledge alt <id> --lang <zh|en> --title … [--body-file -]   # write or replace the entry's other-language half
gtmux knowledge add … --sensitive --confirmed "<their words>"   # the commander's own detail: stays on this machine, written after asking
gtmux knowledge sensitive <id> [--off] --confirmed "<their words>"   # mark (or unmark) an existing entry
# add/supersede also take --kind, --tags a,b, --provenance <correction|recurrence|mined|capture|self>, --hypothesis
gtmux knowledge topic <name> --desc "…"                      # declare your own topic (clients, datasets, …)
gtmux knowledge promote <id> --why "…" [--target "…"]        # charter-level → export brief
gtmux knowledge land <id> --ref "<pr/spec>"                  # close the loop when it lands
gtmux knowledge promotions [--json]                          # the pending export queue
gtmux knowledge list [--topic t] [--json]  ·  show <id>  ·  render [--check]
```

写给使用者的那一份导览（存在哪、一条经验怎么走完、你能改什么）在 [docs/knowledge.zh.md](knowledge.zh.md)。

知识库的权威是一份只追加的台账（`~/.config/gtmux/hq/knowledge/.ledger.jsonl`）；
主题的 `.md` 文件从有效条目渲染出来，归 gtmux 所有、会查漂移（`render --check` 抓手改，
`render` 复原）。每条都带来源：写入时的事件 seq、可选的沉淀区间，以及消费了捕获候选时
那条候选的 pane/seq/task 和键。`add --capture <key>` 把所有同键的待处理候选消费进一条；
`dismiss` 把它们连同一条日志痕迹一起删掉。每一次写操作都往事件流追加一条
`gtmux:audit:knowledge`。

写操作只接受从 HQ 目录发起的调用（和 `gtmux events --ack` 同一条按 cwd 定角色的规则）；
worker 用 `gtmux capture` 记候选。`list`/`show` 在哪儿都能用。给指挥官另留了一扇更窄
的门：`gtmux serve` 把知识库暴露给经 owner 认证的客户端（`GET /api/hq/knowledge` 是
不带正文的索引、`GET /api/hq/knowledge/entry?id=`、`POST /api/hq/knowledge/act`），
只接受 `land` 和 `retire` 两个动词。两扇门写的是同一条 `gtmux:audit:knowledge` 记录。

内置六个主题（accounts、workflows、best-practices、pitfalls、corrections、environment）；
`gtmux knowledge topic <name> --desc "…"` 声明你自己的，立刻带着描述渲染出来，
`gtmux capture`、每一个 knowledge 动词和派活时的知识回声都认它（自定义主题在那里和
pitfalls/workflows 一起出现；accounts / corrections / environment 不进派活上下文）。
名字是 slug（`a-z 0-9 -`，不超过 40 字节）；内置的、已存在的和保留目录名会报错。
声明目前只能新增。

每条都有语言（`lang`），可以带另一种语言的那一半：`add`/`supersede` 接 `--lang` 与
`--alt-lang --alt-title [--alt-body-file -]`，事后用 `alt <id>` 补。读者拿到的是：
源语言对上用源，否则用另一半，都没有就用源并打 `[zh]`/`[en]` 标。`list`/`show` 随
`GTMUX_LANG`（`--lang` 覆盖），本机文件按台账的多数语言渲染，`everyone` 的简报和
issue 用英文。`lint` 报 `monolingual` 计数；gtmux 自己不翻译，两半都由 HQ 写。

敏感条目是指挥官自己的信息（账号、个人情况、他自己决定放进来的凭据）。写入必须带
`--confirmed "<their own words>"`，也就是 HQ 把条目给他看过、他点头的记录。它永远不
离开本机（`promote` 只接受 `hq`，`machine.md` 和仓库块跳过它），Mac 和手机上显示一把锁。
`lint` 的 `unmarked-sensitive` 报长得像凭据却没打标的条目。别人的密钥照旧不进库：
只记在哪，不记本身。

`lint` 还核对脚本和条目的配对：`knowledge/tools/` 下没有条目指向的脚本报
`orphan-tool`，条目指向了不存在的 `tools/<script>` 报 `broken-tool`。

每条条目落在三条轴上：`kind`（`facts`、`howto`、`pitfalls`、`judgment`、`decisions`）；
`provenance`（`correction`、`recurrence`、`mined`、`capture`、`self`，计数由
`knowledge hit` 和采矿器往上涨）；`audience`，晋升时填。`topic` 保留为 id 前缀和自由
标签。轴出现之前写的条目读取时按固定表映射、带 `?` 标记，直到
`knowledge kind <id> <kind>` 确认；台账文件永不改写。`--hypothesis` 把没证实的线索搁在
单独一节，不分发，直到 `confirm`。

`promote <id> --why … --for <hq|machine|repo:<path>|everyone>` 在
`knowledge/promotions/` 写一份晋升简报，带教训、理由、读者及其出口、条目的出处。
`hq`、`machine`、`repo` 三种由 gtmux 替你搬：`land <id>` 把条目写到那个读者看的地方
（LOCAL.md · 正本 `~/.config/gtmux/knowledge/machine.md` 加每个 agent 全局指令文件里的
索引块 · 仓库的 `AGENTS.md`，不提交），并以那个路径为 ref 闭环。`everyone` 的简报里带
预填好的 GitHub issue 链接，人开完用 `--ref <issue url>` 落地。`withdraw <id> --why`
撤回晋升，条目留着。`gtmux doctor` 会标出滞留超过两周左右的简报；`everyone` 不计超期。
`knowledge sync` 刷新各 agent 的块，`carriers` 看每家状态，被手改过的块不给 `--force`
不覆盖。

`knowledge lint` 报孤儿、断链和过时的 `[[links]]`、疑似重复、超期的猜想和晋升、
待确认的种类，以及 `ai-voice`：读起来像机器写的条目。判法借了 humanizer 的分级，
聊天残留、装饰性的 `⇒`、只有自己人懂的自造词，出现一次就算；破折号、粗体、
「不是 X，是 Y」要凑够两类才算；代码、表格和引文原样跳过不读。只报不改，
一行摘要随 self-check 的敲门送到。`neighbours` 按种类和
关键词重合找最相近的有效条目（不用模型）；`add` 写入前先列出最像的三条，
`capture --list` 把候选池按家族分组，一次 `add --capture k1,k2,…` 收成一条。

台账出现之前手写的主题文件，在第一次动到那个主题的写操作时被搬到
`knowledge/legacy/<topic>.md`；渲染里链过去，派活时的知识回声仍然查它（细节见
[TROUBLESHOOTING](TROUBLESHOOTING.md#knowledge-base-migration-and-the-phone-door)）。

## `gtmux spawn` / `gtmux send` / `gtmux tasks` / `gtmux reap`：带核验的派活

`gtmux spawn <goal>` 把新活派给一个 coding agent，并确认它落进去了：

```
gtmux spawn "add a --dry-run flag to the deploy script"
gtmux spawn --title fix-auth-mw --worktree feat/dry-run --model opus "add a --dry-run flag"
gtmux spawn --pane %14 "keep going, then run the tests"
gtmux spawn --json "…"   # → {task_id, pane_id, loc, title, session, delivered, state, judged_by, evidence}
```

只要比一行短句长，就走 `--goal-file <path>`（`--goal-file -` 读 stdin）；
`gtmux send` 有同一条通道，叫 `--message-file <path|->`：

```
cat > /tmp/goal.txt <<'EOF'
把 `hqPlaybookVersion` 提到 13，并且：
1. 运行 `make check`
2. 别让 shell 碰 $HOME 或 `for f in *; do echo $f; done`
EOF
gtmux spawn --title bump-playbook --cwd ~/src/gtmux --goal-file /tmp/goal.txt
gtmux send %14 --message-file /tmp/reply.txt
```

作为命令行参数传的目标会先被你的 shell 解析：反引号包住的会被执行，`$foo` 会被展开，
换行直接结束命令。文件通道上没有 shell：字节从文件 → gtmux → `tmux load-buffer -`
（一个管道）→ agent 的输入框，最多剥掉一个结尾换行（每个 heredoc 都会加一个）。
同时给文件和位置参数是错误。短指令用位置参数没问题（`gtmux spawn --pane %14 "keep going"`）。

重跑一次失败的派活会收敛到一个 worktree、一个会话、一条台账记录：`--worktree` 复用
已经服务于那个分支的 worktree（同一路径挂着另一个分支仍是硬错误）；spawn 会接管自己
上一次的尝试（一条拥有自己会话、目标从没送达、pane 还活着的台账记录）并更新同一条
台账行；某一步失败且没有可续的东西时，这次调用创建的 worktree 或分支会被回滚。

`--title` 命名这个窗口的目的，一个简短的动宾 kebab slug（`fix-auth-mw`、
`review-pr-518`），它会成为 tmux、雷达和 app 里的窗口名和 pane 名。成功时 `spawn`
报出句柄 `<loc> (%pane) · <title>`；`loc` 是实时的 tmux 定位 `session:window.pane`，
每次读时重算，在 `renumber-windows` 下仍然正确。引用派出去的窗口就用
`loc %pane · title`，这样能按号跳过去；HQ 的说明书要求每次派活都给 `--title`，
每次汇报都带这个句柄。

`spawn` 拒绝在 HQ 目录里跑 worker（显式 `--cwd` 指到那儿、没给 `--cwd` 时继承的 cwd
落在那儿、或者 `--pane` 复用一个坐在那儿的 pane）：那个目录的 `AGENTS.md` 是 HQ 的
章程，在那儿起的 worker 会读到它然后冒充 HQ。请传 `--cwd <project dir>`。

`spawn` 拉起 agent（默认是一个全新的 detached 会话，`--pane <id>` 复用一个，
`--worktree <branch>` 跑在隔离的 git worktree 里），从构造上就走网络代理，等 agent
起来，然后经 tmux 粘贴缓冲区投递任务并核验它落地。

等 agent 起来是一道真的闸。pane 要满足这些条件才算就绪：输入框已经画出来，没有信任闸
或选择菜单挡着，屏幕上没有启动横幅，而且两次抓屏逐字节相同（或者 agent 的会话启动
事件已经发出）。启动横幅（`Connecting…`、`Loading…`）会挡住这道闸；常驻通知
（如 `⚠ N MCP servers need authentication · run /mcp`）说的是只有你能做的动作，永远
不会自己消失，所以不挡。超时时失败信息是
`✗ NOT delivered → <handle> — evidence: … blocked by: <the line that said no>`，
后面跟着 pane 的底部区域。

核验是分层的。装了 hook 的 agent（Claude Code、Codex …）以 agent 自己的
`UserPromptSubmit` 事件为回执，不抓屏；其余的用两帧屏幕读取按结构定位输入框。
回执确认过的落地是终局，而且在给出任何 `delivered:false` 之前会再查一次回执，
卡在截止线上才到的确认不会丢。JSON 结果会说明是哪一层判的
（`judged_by: driver|screen`）。在 `~/.config/gtmux/config.json` 里写
`driver.<agent>.receipt: false` 或 `driver.enable: false` 会强制走屏幕读取，
用来隔离事件通道的故障。

粘贴是 bracketed paste，所以多行指令作为一个草稿落下，再由单独的 Enter 提交一次。
唯一算成功的是确认落地：被吞掉的 Enter 会退避重发，粘了一半会重试，超时会带屏幕证据
报 `delivered:false`。重试永远不会重复：重粘只发生在确认为空的输入框里（清除键只清
一行，多行草稿可能扛得住它），只是渲染晚了的粘贴会被放着不动。排队中的提交报为
`state:"queued"`。重发互锁会拒绝在一个时间窗内向同一个 pane 发完全相同的载荷
（重复的 `/compact` 不会连打两下）；`--force` 越过它。飞行前检查（代理、机器资源、
订阅窗口）只是提示，从不拦。

`--oneshot` 通过 agent 的 headless 模式派一个一次性、非交互的 worker
（`claude -p … --output-format stream-json`、`codex exec --json`）；只有支持 headless
的 agent 才接受，其余的拒绝，不会降级成交互式 spawn。目标作为参数传入，没有东西要粘、
也没有落地要核验；这次运行仍然活在一个 tmux pane 里（JSON 流看得见、雷达上有它的行、
reap 也适用），跑完或崩了来自那条流加退出码。一次性 pane 只能看，你不能中途接管。
`--headless` 只是不开终端标签页，派出来的仍然是完全交互式、可以 attach 进去操纵的会话。

### `gtmux send`

`gtmux send <pane> <text>` 默认用同一套落地核验（一确认就返回）；`--no-verify` 跳过
核验，`--force` 越过互锁，`--json` 打印核验结果（`{delivered, state, judged_by, evidence}`，
仅限核验过的发送）。

**一次发送绝不会写进别人没提交的那一行。** 粘贴是追加到输入框的，落在写到一半的行上
会把对方的字连同你的载荷一起提交。每一条路径（CLI、HQ、`--no-verify`、手机）都先读
草稿，然后拒绝（`state:"refused-draft"`，什么都没写），并把草稿引回来给你看。
`--force` 可以豁免；手机那把幂等键不豁免，因为来自另一台设备的发送现场没人能撤销。

这道草稿检查只在已知 agent 驱动的 pane 上运行（跑着 vim、ssh 或别的 TUI 的 pane 没有
输入框，在那儿读会把 transcript 当成「草稿」）。它读带颜色的抓屏，所以 agent 自己那条
暗淡的「建议下一条命令」不会被当成你在打字；而且要在两帧里看到同样的东西才拒绝。
凡是它判断不了的，都放行：

| 它看到什么 | 它怎么做 |
|---|---|
| pane 里没有 agent（没有输入框） | 发 |
| 抓屏失败（tmux 打了个嗝、pane 没了） | 发 |
| pane 正在回滚（copy-mode） | 发 |
| 一个普通 shell，没有输入框 | 发 |
| 你自己的文字，同一条消息在重发 | 发（幂等） |
| 别人的文字，而且看到了两次 | **拒绝** |

它最多花两次读取加一个轮询间隔，从不循环。

以 `failed` 收尾的发送可以立刻重试：互锁会丢掉从没落地的尝试记录，重试不会被
`refused-duplicate` 顶回来。`queued` 的发送保留记录，agent 已经收下了。`--no-verify`
和手机端 `POST /api/send` 跳过的只是确认：每一条文本路径都先粘贴、再把 Enter 作为
独立的键发出去，所以不核验的发送同样不会把多行消息拆开。`--key` 仍然是单个按键。
比一行短句长的内容，用 `--message-file <path|->`。

普通终端 pane（进程子树里没有 coding agent，前台是 bash、zsh 这样的裸 shell）是直接
打进去的：文本然后 Enter，没有输入框确认，也没有重发互锁。`--json` 把这种发送报为
`{"delivered":true,"state":"sent"}`。不能证明是普通 shell 的（vim、ssh、进程扫描漏掉的
agent）走的还是 agent 那条管线。

### `gtmux tasks`

`gtmux tasks [--json]` 是派活 / 需要你的账本：每一个你派出去的任务，带实时状态
（undelivered / waiting / done / working / gone），需要你的排最前。`--verbose` 加上
归档条目和注意力列（层级 · 优先级 · 是否呈现 · 处置）。

`undelivered` 排在最前面。台账记的是投递判定，所以目标从没抵达 agent 的记录，无论它的
pane 看起来多空闲，都读作 `✗ undelivered`。`queued` 的投递不算未送达（agent 收下了，
排在当前回合后面）。一条把同一个目标送进同一个 pane 的 `gtmux send` 会把记录标成
已送达；往 pane 里敲一句无关的话不会。

`gtmux tasks --pending` 是待决策的常驻视图，也就是摆在你面前的事：

```
▸ t1kx8p2m9dq3  hq %21                 08-09 14:32  ship v0.48.0 or hold for §4?
▸ t1kx9r4w0aa1  worker-b %8            08-09 09:11  which branch should the migration target?
```

打头的字形是 `▸`（这张表上的条目在等一个决策）。排序是等得最久的在前，然后按 pane，
再按 id。视图只读台账，打的是绝对时间戳，所以对一张没变的表读两次，结果逐字节相同。

条目进出这张表用 `gtmux tasks --await <task_id>` 和
`gtmux tasks --resolve <task_id> [disposition]`（处置记录它是怎么离开的：
`decided` / `withdrawn` / `escalated`；不给就只是清掉）。在表上的判据是
`awaiting-commander` 这个处置，所以任何别的处置同样会把条目拿掉，归档一条记录也会把它
从视图里关掉。

### `gtmux reap`

`gtmux reap <pane|task_id>` 安全地回收一次跑完的派活。先过一道安全闸（worktree 必须
干净、分支必须已合并），过了才杀会话、删 worktree、删掉已合并的分支；闸没过就精确报告
是什么挡住了，什么都不碰（`--abandon` 强行越过，`--keep-branch` 保留分支）。失败的步骤
连同 git 自己给的原因列在 `⚠ but these steps failed` 下面，命令以非零退出。
`--snooze [--for <dur>]` 对一个你要留着的派活消掉回收建议。被跟踪的派活看起来可以回收时，
活着的 HQ 会收到一条 `» gtmux·reap-suggest … │ gtmux reap <id>` 唤醒。回收永远是
建议 → 批准 → 执行，不会自动发生。

## `gtmux usage`：token 监看

```
额度   已用多少，以及窗口什么时候回来
  claude 5 小时              22% ██░░░░░░░░░░   1h后回来  Jul 13 at 1:29am
  claude 本周（全部模型）    74% ████████░░░░   3h后回来  Jul 17 at 10:59pm

对话  8                                        输出    ctx   速率
  每段对话自它开始以来
  ⠿ api:0.0                                    2.1M    85%   7k/m   ⚠ ctx 85%
  ⠿ web:0.0                                    830k    60%  391/m
    … 另外 5 段空闲对话，合计 50k

合计   这台 Mac 上的全部 agent
  今天                      2.8M
  本周                     16.2M   claude 15.1M · codex 1.1M
  自 6月12日                233M   最多的一天 9.4M · 连续 23 天，最长 31 天
```

额度排在最前：它是本地数不出来的那个数，也是决定你还能不能接着干的那个。对话列表只留头部，
尾巴收成一行；每行重复的列名收进一行抬头，抬头里还写明每列是哪段时间。一段对话自己的总量
可以比整周的还大，因为这段对话比这一周还老，没有那句话，这两个数字就是互相打架的。

这里以前有三样东西都叫「会话」。tmux 的 session 是 `overview` 数的、`restore` 接回来的那个；
这张表列的是 agent 的一段对话；Claude 那个滚动五小时的额度，现在按时长叫 `claude 5 小时`。
`--json` 里仍然是 agent 自己写的那个标签。

`今天` 和 `本周` 两行把 token 按本地日期、跨全部 agent 加总，每条消息记到它
发生的那一天。`--json` 在 `history` 里带最近七天，在 `history.activity` 里带账本这
一整年（每个有输出的日子、自账本第一天起的累计、峰值、连续天数）。再一行
`Σ all … since … · peak … · streak …` 一句话说这一年；`gtmux usage --activity` 把它画成
手机和 Mac 阅读器上那张日历热力格（周在横向，周一到周日在纵向，GitHub 那五档绿；
终端多宽就画多少周，认 `COLUMNS`）：

```
Token activity   last 26 weeks
all 70.8M · peak 5.3M · streak 14d (best 25d)

      Apr     May       Jun       Jul       Aug       Sep
Mo  · · · · · · · · · · · · · · · · · · ░ ░ ░ ▓ ░ · █ ▓
    · · · · · · · · · · · · · · · · · · ░ ░ ░ ▒ ▒ · ▒ ░
…
  Less · ░ ▒ ▓ █ More
```

按会话的 token 统计确定性地从 agent 自己的日志里解析出来（零 LLM 调用）：累计输出/
输入、实时上下文占用（最后一条消息的 input + cache token，对着一个由证据推断的窗口
判断），以及 10 分钟的消耗速率。分层阈值按 agent 类型写在 `~/.config/gtmux/usage.json`：

```json
{"claude": {"ctxWarn": 0.8, "sessionOutWarn": 20000000,
            "typeRatePerMinWarn": 30000},
 "horizonMin": 30}
```

评估器还会外推（`current + rate × horizon`），所以你在撞墙之前就被告警
（`ctx→80% in ~9m`）。告警以琥珀色 `usage_warn` 出现在雷达行上（`agents --json` /
digest）、`gtmux usage` 里，以及作为每层一次的 `» gtmux·usage·warn …` 唤醒敲进活着的
HQ 会话。`--json` 也由 `GET /api/usage` 提供。hook 在每个生命周期事件上评估：工具驱动的
工作期间接近实时，一次长时间静默的生成在下一个事件时结算。

Claude 记的是每条消息花了多少，所以总量是累加出来的。Codex 每个回合记一次会话的运行
总量，所以总量就是最后那次读数；它还直接写出 `model_context_window`，上下文占比是拿
真实窗口算的。日志里完全没有用量的 agent 仍然有它那一行，这几个字段留空。

> 按网络环境启动：gtmux 拉起 agent 时（`gtmux hq` / `adopt` / restore / limits 命令）
> 会按需加上代理前缀，你不用在不同网络之间手动切。`~/.config/gtmux/config.json` 里
> `"agentProxy": "auto"`（默认）表示仅当那个端口在监听时（你的代理工具在跑，家里挂
> VPN 的情形）才加 `http://127.0.0.1:<agentProxyPort, 7897>`，否则什么都不加（内网）；
> 写明确的 URL 就强制用它，`"off"` 关掉。

## `gtmux events`：会话事件流（订阅）

```
22:50:40  working          api:0.0        Claude Code (%7)
22:51:02  waiting·permission  api:0.0     Claude Code (%7)
22:53:19  idle             web:1.0        Codex (%11)
```

hook 把每个会话的生命周期事件（开始 / 结束 / 等待 / 后台）追加进一个会轮转的日志
（`~/.local/share/gtmux/events.jsonl`，活动 20 MB + 1 个轮转 ≈ 40 MB 上限，配置项
`eventsCapMB`，`0` 关闭）。`gtmux events` 打印最近一小时；`--since 10m|2h` 给一个
时间窗；`--follow` 实时流式输出，认得轮转。`--since-seq N` 是一次性的增量读取
（严格在序号 N 之后的全部，最旧的在前，可与 `--severity`/`--json` 组合）：HQ 被一条
指明序号区间的信号线唤醒，然后精确拉那一段增量，任何能跑 CLI 命令的 agent 都做得到。
这是同一批事件的终端原生订阅，各端拿到的是 SSE 版本。

从 HQ 目录发起、不带过滤的 `--since-seq` 读取同时把水位推到它返回内容的末尾，这就是
让 `unread` 敲门停下来的东西（见[消费水位](#消费水位为什么不会漏掉东西)）。`--ack N`
显式回写水位，用于那条流以别的方式对上账的场合，比如一次完整的 `gtmux digest`。
两者都只对 HQ 生效（按 cwd 判定）；worker 在某个仓库里跑 `gtmux events` 什么都不改。
规则认的是那个确切的 cwd：从 HQ 目录的子目录（`notes/`、`knowledge/`）读不算数，
并在 stderr 上告警、指出该在哪个目录里跑；stdout 和退出码不变。

HQ 自己的增量拉取略掉那些从来不计数的记录（HQ 自己 pane 的行、没有 pane 的闪烁、
gtmux 的 `gtmux:audit:*` 轨迹），并在 stderr 上说明扣掉了多少条。`--all` 拿回原始
视图；两种形态都算消费。别人的读取一切照旧。

`--severity <tier>` 过滤出那一档及以上。档位排的是紧急度，所以它们是三种不同的读法：
不带过滤的 `--since-seq` 增量用来对账；`--severity notable` 是舰队变化流（一条指令
抵达某个会话，`origin:"instruction"`，加上回合结束和生命周期）；`--severity important`
是升级子集（卡住 · 在问 · 崩了），用来先分诊。

`--acts` 只留 HQ 自己做的事（派活、回收、记知识、轮换、自检、蒸馏），去掉唤醒的投递
记录（`wake-delivered` / `wake-dropped`）。这跟手机「HQ 的工作」一节走的
`GET /api/hq/events?acts=1` 是同一道分割，菜单栏「HQ 做了」那一行数的也是这一批；
「HQ 今天干了什么」就是 `gtmux events --since 24h --acts`。和所有过滤读法一样，
它不算消费。

这条流里还带着 gtmux 自己的控制记录，也就是它替 HQ 发起的周期维护触发，渲染成
`[CONTROL <event>]` 并带上理由：

```
09:57:16  [CONTROL gtmux:self-check]  due (daily) — review feed/ledger/memory health…
04:33:49  [CONTROL gtmux:distill]     due (weekly) — distil the period into the KB…
```

所以「那轮周期任务到底跑没跑」就是 `gtmux events --since 30d | grep distill`。
`gtmux doctor` 的 HQ 维护几行显示每轮上次跑在什么时候，并标出落后于自己节奏的那一轮
（只在有 HQ 目录的机器上显示）。

## `gtmux resource`：本机资源监看

```
disk 40GB free · mem 38% free (warn) · load 0.64×14 cores · power 74% (battery 2:13)   ⚠ disk 40GB free
per-agent (RSS · CPU):
  %26    252MB · 9.2%
reclaim candidates (orphans no live agent owns):
  pid 3015  100MB · 0.0%  iOS Simulator runtime (12 procs) [simulator]
    ↳ leftover iOS Simulator runtime — `xcrun simctl shutdown all`
```

磁盘（`df`）、内存（`memory_pressure -Q` 的空闲百分比，加上内核
`kern.memorystatus_vm_pressure_level` 的 normal/warn/critical 档）、CPU（loadavg÷核数），
以及电源/电池（`pmset -g batt`：电量 % · 接电还是放电 · 剩余时间；没有电池的机器
不显示；低电量只在放电时才计入告警和档位，接着电时不计）。按 agent 的 RSS/CPU 靠走
每个 pane 的进程树得到，可回收候选是没有活着的 pane 认领的重进程，带 pid 和回收办法
（残留的 iOS 模拟器运行时聚合成一条，dev server 和 tmux 游魂各自单列）。阈值在
`~/.config/gtmux/config.json` 的 `resource` 对象里（diskAmberGB 50 / diskRedGB 15 /
loadAmber 1.0 / loadRed 1.5 / orphanRssMB 300 / batteryAmberPct 20 / batteryRedPct 10）。
可回收候选只跟着它救得了的那种告警走。候选是进程，它的体积是内存：这条建议只跟着内存和负载
的告警出现，并写明它占的是多少、哪一种资源；磁盘和电量的告警不带它。杀掉一个 902MB 的进程还不回
一个字节磁盘，而磁盘告急时给出这条建议，前后被照做过两次才有人发现（#1109）。

`GET /api/usage` 里带一个 resource 块；serve 的节拍会给 HQ 发 `resource·warn` 提醒
（每次越线一次）；`gtmux hq`/`new` 在红档时先警告再加负载。

告警有三重阻尼，卡在阈值上的数值不会反复告警（读数本身保持原始）：

| 键 | 默认 | 作用 |
|---|---|---|
| `diskHysteresisGB` | 2 | 高出入口线多少 GB 才解除磁盘档（<15 GB 进红，≥17 才解除） |
| `loadHysteresis` | 0.15 | load÷核数低于入口线多少才解除负载档（≥1.0 进琥珀，低于 0.85 才解除） |
| `batteryHysteresisPct` | 3 | 高出入口线几个百分点才解除电池档（<20% 进琥珀，≥23% 才解除） |
| `confirmSamples` | 3 | 连续几次采样一致才相信这次档位变化 |
| `minRestateMinutes` | 30 | 同一档再次告警前的安静期；升到更糟的档不受此限，立刻告警 |

## `gtmux limits`：订阅窗口的真实剩余

```
已用多少，以及窗口什么时候回来
  claude 5 小时              22% ██░░░░░░░░░░   1h后回来  Jul 13 at 1:29am
  claude 本周（全部模型）    74% ████████░░░░   3h后回来  Jul 17 at 10:59pm
  claude 本周（Fable）      100% ████████████   3h后回来  Jul 17 at 10:59pm
  codex 本周                  0% ░░░░░░░░░░░░   2d后回来  Jul 20 at 10:02am

  claude 本周（Fable）用完了，Jul 17 at 10:59pm 才回来。Claude Code 照常还能用，
  那个窗口用了 74%。
刚读的
```

一个窗口一根条，「已用」只在抬头说一次；只有某个窗口到顶时才有收尾那句话，说的是它什么
时候回来、在那之前什么还能用，而不是把上一行的数字再念一遍。

你的套餐还剩多少，来自 agent 自己上报的真实服务端数据。Claude 和 Codex 报在不同的地方：

- Claude 本地没有任何窗口信息（transcript 里是会话花费，stats 缓存是全时段模型总量），
  所以 gtmux headless 地跑它自己的命令：`claude -p "/usage"`。
- Codex 把服务端的限额响应写进了会话 rollout，就在 token 计数旁边，gtmux 读出来就行。
  不起进程，不用命令。

日志这条路有两条规矩。窗口按时长命名，不按它在数据里的位置（Codex 的 `primary` 字段里
既出现过周窗口也出现过 5 小时窗口）。重置时间已经过去的窗口会被丢掉，因为日志只新到
最后一个回合为止。

Codex 读不到窗口、而你这一周里又用过它时，它会得到自己的一行：

```
● claude 5h                   9% used   resets Sep 7 at 9:09pm
● claude week (all models)   50% used   resets Sep 11 at 10:59pm
○ codex  the window it last reported has ended — codex writes its plan into its own log, so one turn brings the figure back
```

`gtmux usage` 的页脚把同一件事标成 `codex unknown`。一周都没用过的 agent 完全不出声。

`gtmux limits` 列出每一个窗口。其余每个地方（`gtmux usage` 的页脚、手机头部那一行）
每个套餐只显示最紧的那一个。告警的规矩不同：它忽略 5 小时窗口，那种窗口自己会重置。

每个窗口都写明属于谁的套餐，第一个 agent 的也不例外：`claude 5 小时`、
`codex week`，绝不会出现光秃秃的一个窗口名。`spawn` 的飞行前检查打印的就是这条告警，
说的是这活真正要计费的套餐。Claude 这条路要起进程，所以结果会缓存，TTL 15 分钟，
有窗口接近上限时缩短到 5 分钟；`--refresh` 强制刷一次。配置在 `~/.config/gtmux/usage.json`：

```json
{"limitsCommand": "claude -p /usage", "limitsTTLMin": 15,
 "limitsTTLNearMin": 5, "limitsNearPct": 70, "limitsWarnPct": 85,
 "limitsTimeoutSec": 60}
```

网络需要的话，在 `limitsCommand` 前面带环境变量前缀（`"HTTPS_PROXY=… claude -p /usage"`），
或者设成 `""` 关掉。跑超过 `limitsTimeoutSec` 会被杀掉。失败的一次不会被当成新鲜数据
缓存，已有的额度数字留着不清空，命令会退避（1、2、5 分钟，之后按 TTL），不会被每个
调用方各刷一遍。周窗口到达或超过 `limitsWarnPct` 会标成琥珀，并唤醒活着的 HQ 一次
（`» gtmux·limits·warn …`）。`limits` 这一块也随 `gtmux usage` 和 `GET /api/usage`
一起给出。

## `gtmux logs`：gtmux 看到了什么、做了什么

所有 gtmux 进程都写进同一个本地日志库，思路和 macOS 的系统日志一样：serve、隧道客户端、
hook、每条命令、菜单栏。记录分两类。诊断记下 gtmux 看到了什么；操作记下它对这台 Mac 做了
什么、谁发起的、作用在什么上、结果如何（`ok`、`refused` 加原因、`failed` 加错误）。发起者
是 `user`（你敲的命令）、`hq`（HQ 跑的）、`agent:%7`（%7 里的 agent 跑的）、`menubar`、
哪台手机或哪个浏览器（`phone:3f9c20e1`）、分享链接（`guest:…`），或者 `system`，即 serve
和 hook 自己做的事。

<!-- gtmux:rendered log-lines -->
```
09:36:05 serve   serve.start  serve started · backend=direct port=8765
09:41:12 serve   act.send  phone:3f9c20e1 → %7 ok · bytes=42 via=tunnel
09:44:02 serve   warn  act.pair  anonymous refused · a pairing code was not accepted · reason=expired via=tunnel
```

```sh
gtmux logs                                   # 最近一小时
gtmux logs --since 1d --acts --actor phone   # 今天手机做过的所有事
gtmux logs --event 'act.pair' --since 2h     # 每一次配对，以及被拒的原因
gtmux logs --level warn --since 3d           # 警告和报错，被拒的操作也在内
gtmux logs --follow                          # 持续打印新记录
gtmux logs --json --since 10m                # 原样输出，给脚本和 agent 用
gtmux logs --since 1d --stats                # 存了多少，今天出了多少问题
```

`--stats` 不打印条目，只回答日志库本身：多大、最早哪天、保留多久，以及这段时间里有多少
条是警告或报错。加 `--json` 就是一个对象，菜单栏的诊断那一节读的就是它。

配对被拒会写明三种原因之一：`expired`（码的 5 分钟过了）、`used`（码只能用一次）、
`unknown`（这个 serve 没发过这个码，重启前发的码就是这样）。

每种操作都有固定的事件名，`--event` 按它匹配。下面是全部事件，以及会记录它的命令
（`serve` 指 serve 替手机、浏览器、分享链接或命令行做的事，`hook` 指 agent 的 hook，`app`
指菜单栏 app）：

<!-- gtmux:rendered act-catalog -->
```
act.adopt            adopt
act.app.launch       app
act.attach           attach, serve
act.awake.off        awake
act.awake.on         awake
act.capture          capture
act.cleanup          doctor, serve
act.config.set       config, quiet
act.doctor.bundle    doctor
act.doctor.fix       doctor
act.focus            focus, serve
act.hq.brief         hq
act.hq.export        hq
act.hq.import        hq
act.hq.rotate        hq
act.hq.start         hq
act.install.app      install
act.install.hooks    install
act.knowledge        knowledge, serve
act.knowledge.sync   knowledge, doctor
act.mint             pair, serve
act.narrow           serve
act.new              new
act.notify           hook
act.notify.post      app
act.pair             serve
act.push.forget      devices, serve
act.push.register    serve
act.reap             reap
act.reap.snooze      reap
act.restore          restore
act.resume           restore
act.revoke           pair, devices, share, serve
act.send             send, serve
act.share.config     share, serve
act.share.create     share, serve
act.share.set        share, serve
act.spawn            spawn
act.tunnel.off       tunnel
act.tunnel.on        tunnel
act.tunnel.redeem    tunnel
act.uninstall.app    uninstall
act.uninstall.hooks  uninstall
act.unwatch          panes
act.update           update
act.upload           serve
act.wake.delivered   serve, hook
act.wake.dropped     serve, hook
act.watch            panes
```

restore 也一直往这里写它的判断过程：选了哪份存档、每个 pane 接回了哪段对话
（`gtmux logs --component restore --since 1d`）。

记录只用英文，不包含消息正文，一次发送只记长度和一个短哈希。token、配对码和
`Authorization` 的值在写入时就被替换掉。日志库在 `~/.local/share/gtmux/logs/`，每天一个
文件，只有你能读。保留 30 天或 100MB，先到哪个算哪个（`~/.config/gtmux/config.json` 里的
`logs.retainDays` 和 `logs.maxMB`）。每天第一条记录由哪个进程写，就由它顺手删掉过期的，
所以不开 serve 也不会越积越多。某天超过 20MB 会另起一个文件，并用一条 `log.runaway`
点名是谁写满的。`GTMUX_DEBUG=serve,tunnel`（或 `all`）让这一次运行多记 debug；
`config.json` 里写 `"debug": "hook"` 则对所有进程生效，launchd 拉起的也算。守护进程在能写
日志之前打印的东西（比如崩溃信息）进 `logs/<组件>.stderr`，由 serve 的定期清理控制大小。

菜单栏里同样的三件事不用开终端。偏好设置的「诊断」一节会说日志库有多大、今天有几条出了
问题；**打开**是一个列表，最近三天、新的在上面，可以切到只看出问题的；**打包…** 跑的就是
下面那条 bundle 命令，跑完会说文件落在哪；**多记一些细节**就是 `gtmux config debug`（各个
进程下次启动才生效，追完了记得关掉）。

`gtmux doctor` 有「日志」一节：日志库的大小和最早的日期、一周内有没有组件刷屏、一天内的
报错、gtmux 存的文件有没有被这台 Mac 上别的账号读到的可能、其他数据有没有超出上限。
`gtmux doctor --fix` 会执行清理并收紧权限。这里的内容不会上传到任何地方。

要报告问题，用 `gtmux doctor --bundle` 打一个文件：日志库、状态文件、每份 launchd 输出的
最后 256KB、文字版的 doctor 报告，以及 gtmux、app、macOS、tmux 的版本。打完会列出装了
哪些东西。gtmux 保存的每个 token 和配对码在打包时会再替换一遍，没经过日志库的 launchd
输出也一样。事件流默认不放，因为里面有你的提示词开头；要带上就加 `--with-events`。文件
只有你本人能读，发给谁由你决定。

```sh
gtmux doctor --bundle                  # 在当前目录生成 gtmux-diagnostics-20260920-0930.tgz
gtmux doctor --bundle ~/Desktop/r.tgz  # 自己指定路径；已存在的文件不会被覆盖
```

手机也留着同一类记录：它请求 Mac 失败的情况、每次配对以及失败原因、推送注册、实时连接
的断开和恢复，最多 500 条或 200KB，只存在手机上。「设置 → 诊断记录」点进去就能看：每条
都是一句话（「联系不上 Mac · GET /api/agents 等了 6s 没有回应 · 之后一分钟内又失败了 4
次」），按天分组，可以只看出问题的。「拷贝」或「分享」交出去的是没翻译过的原始记录，
JSON 行，和 `gtmux logs --json` 一样，两边同一段时间的记录可以对着看。

## `gtmux awake`：合上盖子也继续跑

```
gtmux awake on       # asks for your admin password once, then verifies it took effect
gtmux awake          # awake = on (clamshell) · up 2h13m · power battery 74%
gtmux awake off      # no password — immediate
```

合上 MacBook 的盖子系统就睡了，隧道断掉，每个 agent 冻在回合中间。`gtmux awake on`
让 Mac 合着盖子也不睡（旧名字 `gtmux server-mode` 仍然能用）。打开要一次密码，
关掉不要任何代价，gtmux 已经死掉时也能关。

同一次授权里会装下一个 root 属主的小守卫。它唯一的权力是把睡眠还回来：下面任何一件事
发生，它都会恢复睡眠并删掉自己：

| 触发 | 意思是 |
|---|---|
| 你把它关掉 | 一个不需要特权的标记；守卫大约一秒内就会醒来处理 |
| 电量掉到 20% | 30% 时就会在 Mac 和手机上提醒你 |
| gtmux 不再运行 | 崩溃、强杀、`brew uninstall`：不需要任何东西活下来 |
| 重启后没人登录 | 有一段启动宽限期，所以一次正常的重启不会杀掉你的会话 |

**它永不过期。** 你不关它就一直跑，整段时间里菜单栏图标带着一个缓慢呼吸的红点，
和屏幕录制同一种视觉语言。

用电池是被支持的场景：合着盖子在房间之间走动照样工作。终结它的是剩余电量，拔掉适配器
不会。

gtmux 只回退 gtmux 设过的东西。一个不是它盖章的 `disablesleep`，它会报告出来并给出
手动撤销的命令，不替你改。`gtmux doctor` 呈现同一个发现，在从没碰过这个设置的机器上
保持沉默。

状态从哪儿读：

| 来源 | 能不能用 |
|---|---|
| `pmset -g` / `-g custom` / `-g live` | ❌ 两种状态下都从不报告 `disablesleep` |
| 电源管理 plist | ⚠️ 落后于写入；它回答的是「重启后还在不在」 |
| `ioreg -r -c IOPMrootDomain` → `SleepDisabled` | ✅ 实时、不需要特权的真相 |

`gtmux awake --json` 报两种读数，外加 `owned_by_gtmux`、`guard` 和一个 `platform`
结论。在项目没有验证过的 macOS 上，`on` 会直说；机制根本不存在的地方，它在要密码之前
就拒绝。

两条边界：

- `gtmux serve` 是按用户的 LaunchAgent，重启之后要有人登录它才会起来。在一台开了
  FileVault、没人在场的 Mac 上，心跳不会恢复，睡眠会被还回去，所以服务器模式扛不过
  一次无人值守的重启。gtmux 不会去动 FileVault 或自动登录来「修好」这件事。
- 底层那个设置苹果没有文档。它在 macOS 26 上验证过，运行时探测；将来某个 macOS 去掉了它，
  `on` 会带着理由拒绝。

你的手机能看到这个状态（连接点上的一个环，以及服务器 / 管理 Mac 里的一行），但改不了它：
每一条改它的路径都通向在 Mac 上敲一次密码。

## `gtmux restore`

退出终端不会杀掉 tmux 服务器和任何会话，没了的只是标签页。重新打开之后，在任意一个
标签页里跑一次：

```sh
gtmux restore            # one terminal tab per tmux session, all attached
gtmux restore --pick     # choose which sessions: "1 3" / "1,3", Enter = all, q = cancel
gtmux restore --one      # attach the next unattached session in this tab
gtmux restore <name>     # attach a specific session here
gtmux restore --dry-run  # print what would happen, change nothing
gtmux restore --plan     # preview: which sessions + agent conversations would come back (read-only)
gtmux restore --plan --json   # the same plan as JSON (the menu bar's source for its expandable restore row)
```

一次只能恢复一次：一次运行会持有一把锁（pid + 启动时间），第二次运行会说明情况然后
什么都不做。锁的进程没了、或者锁超过 10 分钟，就会被接管。`--plan` 和 `--dry-run`
不受锁的限制。

真正的 `gtmux restore` 会在开头打印计划：它即将带回来的会话，以及每个 pane 下面那段
agent 对话（目标）。`--plan` 就是单独的预览：读最后一次 resurrect 存档加上 resume 记录，
不启动任何 tmux（随时跑、随时轮询都安全）。标着 `×` 的 agent 行，是 transcript 已经从
磁盘上消失、恢复不了的对话。

只有存布局时确实在跑 agent 的 pane 才会把 agent 带回来；恢复是从存档里每个 pane 自己的
命令记录读出来的。存档时是普通 shell 的 pane，回来还是普通 shell，哪怕你上周在里面跑过
agent。一个 pane 拿到哪段对话，按它的 resume 记录来；记录丢了的话，恢复会从存档记下的
那条 `--resume` 命令里把 id 抠出来。

恢复总会打印它正在放回去的那个时刻，比如「Restoring the layout saved at 09:57 (37m ago)」。
写那个文件的自动保存挂在 tmux 的状态栏上，只在有终端接着并在重绘时才跑：合上盖子它
什么都不保存。`gtmux serve` 会盯着那个文件兜底：大约 10 分钟没有任何东西写过存档
（有自动保存触发器时约 20 分钟），serve 就自己跑一次保存。`gtmux doctor` 的
`resurrect autosave` 一行会标出已经武装、但几个小时没保存过的触发器。

恢复之后，gtmux 把每个存下来的窗口的 pane 数量和排布跟活着的那个比一遍，把对不上的
点名出来，打在终端上并写进日志库（`gtmux logs --component restore`），因为 tmux-resurrect
会把自己的布局错误丢掉。

tmux 的 pane id 是每个服务器的序号：重启服务器之后，`%25` 会发给另一个 pane。gtmux 有
很多状态按这个号索引，所以恢复时（以及 `gtmux serve` 每隔几分钟）会丢掉那些 pane 已经
不在的按 pane 索引的记录。对话记录（`resume/`、`usage/`）按定位和对话 id 索引，永远不动。

诊断一次恢复不需要重启：把 `XDG_DATA_HOME` 指到任意一份存档的副本上，只读地预览：

```sh
mkdir -p /tmp/probe/tmux/resurrect && cd /tmp/probe/tmux/resurrect
cp ~/.local/share/tmux/resurrect/tmux_resurrect_<stamp>.txt . && ln -sf tmux_resurrect_<stamp>.txt last
XDG_DATA_HOME=/tmp/probe gtmux restore --plan     # what restore would bring back from THAT save
```

第一次跑会弹一个自动化权限对话框（「想要控制 Ghostty」，或者 iTerm2/Warp，看你的
标签页由谁托管），点允许。重启之后 tmux 服务器也没了；`gtmux restore` 会启动 tmux 并
显式驱动 [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) 恢复最后一次
自动保存（它会等恢复完成，大布局要 30 秒以上；存档在但恢复不了时，它拒绝覆盖那份存档）。
跑着的程序不会被重启，请自己重新拉起，比如 `claude --resume`。

只要 resurrect 配了抓取，每个 pane 之前的输出（回滚缓冲）也会作为快照回来。
推荐写进 `tmux.conf`：

```tmux
set -g @resurrect-capture-pane-contents 'on'   # snapshot each pane's scrollback
set -g history-limit 50000                     # how much scrollback to keep/restore
```

> shell 的 ↑ 命令历史是另一回事：它住在你 shell 的 histfile 里，不在 resurrect 里。
> 默认只在 shell 退出时写盘，所以一次重启会丢掉最近的命令。想立刻落盘（bash）：
> 在 `~/.bashrc` 里 `shopt -s histappend; PROMPT_COMMAND='history -a'`（zsh 是
> `setopt INC_APPEND_HISTORY`）。

这些规则背后的事故（重启后的幽灵 agent、无声的布局失败）见
[TROUBLESHOOTING](TROUBLESHOOTING.md#restore-phantom-agents-and-silent-layout-failures)。

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
的尺寸排好版，你可以绑一个键，让它浮在全屏程序上面而不打断它。

## `gtmux new`

```
gtmux new                    # a session named for the current directory
gtmux new api                # …named api
```

新建一个 tmux session 并开一个接上它的终端标签页，走的是 `focus` 和 `restore` 用的
同一个终端驱动，所以标签页落在你看得见的地方，省得之后再去找一个 detached 会话。

## `gtmux adopt`

```
gtmux adopt 4f0c1a2b                 # 把那段对话接进一个新的 tmux session
gtmux adopt 4f0c1a2b 91de77c4        # several at once
```

在 tmux 之外起的 agent 是只读感知的（「不在 tmux」那一节：它的 hook 触发时没有
`$TMUX_PANE`，gtmux 知道它存在，但没有 pane 可以显示、跳转或输入）。`adopt` 按会话 id
在一个全新的 tmux 会话里恢复那段对话，从此这一行就是完整的一行。id 从
`gtmux agents --json`（`session_id`）或雷达行上取。只有 CLI 支持按 id 恢复的 agent
才能被接管，其余的列出来但不动。

## `gtmux focus`

```sh
gtmux focus web          # bring the terminal tab showing session "web" to front
gtmux focus %11          # jump to that exact window+pane, then focus its tab
```

每个标签页的标题是 `session — window`，所以 `focus` 找到匹配的标签页并把它调到最前
（通过终端的 AppleScript）。给 pane id（`%N`）还会在会话内部选中那个 window+pane，
你正好落在 agent 所在的位置；点通知能把你直接放到刚跑完的那个 agent 上，就是这个机制。

一个没有窗口开着的会话（`--headless` 派出来的，或者你自己 detach 掉的）没有标签页可以
调到前面，所以 `focus` 会开一个并接上去。判据是会话的客户端数量，跟它当初怎么起的无关：
后来被人接上的 headless 会话就是一次普通跳转。各端会把这样的行标出来
（`no window` / `无窗口`），你点之前就知道会开一个新标签页。

> 需要 `set-titles on` 配 `set-titles-string '#S — #W'`，标签页标题才会保持 `focus`
> 匹配的那个格式。如果还有别的工具也在写标签页标题，把它关掉，让标题保持权威。

宿主终端：Ghostty 和 iTerm2 是完整驱动的（AppleScript 精确到标签页）。Warp 是尽力而为：
它没有 AppleScript 词典，只有当那个标签页的 Warp 会话 uuid 被记进了 tmux 的会话环境时，
`focus` 才能跳到确切的标签页（gtmux 自己的 restore/new 接入会做这件事；或者把
`WARP_TERMINAL_SESSION_UUID` 加进 tmux 的 `update-environment` 以覆盖手开的标签页），
否则只是把 Warp 激活到前台；`restore`/`new` 通过启动配置来开 Warp 标签页。其它终端
回退到 Ghostty 驱动。宿主自动探测，`GTMUX_TERMINAL=ghostty|iterm2|warp` 可以覆盖。

## `gtmux attach`：在另一台机器的终端里进远端会话

`focus` 跳到的是本地标签页；`attach` 把一个远端 pane 开在你当前的终端（Ghostty /
iTerm2 / Terminal）里，作为原始、可交互的透传：本地终端变成那个远端 tmux 会话，
走同一个 `gtmux serve` 面（一个 WebSocket，`GET /api/attach`），遵守 owner/访客的
token 范围。

```sh
# owner — full access with the serve token:
gtmux attach http://<mac>:8765 --token <serve-token> %12

# guest — a scope-restricted share link (from `gtmux share new`, or the menu bar's
# Sharing → New link); attach exactly what the host allowed:
gtmux attach 'https://<mac>.example/#g=<token>' %12

gtmux attach <target>            # omit the pane: auto-attach the only one, else pick
gtmux attach <target> --read-only  # watch only, never send input
gtmux attach <target> --predict    # experimental: hide round-trip lag while typing
```

`--predict`（实验性，默认关）是预测性本地回显，把 mosh 的想法搬到 WebSocket 桥上。
慢链路上每一次击键否则都要等一个完整往返才回显（跨洲隧道约 340 ms）。开了 `--predict`，
你自己打的可打印字符和退格立刻出现，带下划线表示尚未确认，权威输出一到就被抹掉；
服务端屏幕永远赢。真实击键原样转发给那个 pane。快链路/局域网上什么都不画，全屏 TUI
里也不画（服务端会告诉客户端这个 pane 进了备用屏幕），任何会改变状态的键（Enter、ESC、
方向键、Ctrl-C、Tab）都会结束这一段预测。客户端的光标位置从服务端学；见
`docs/design/mosh-predictive-echo-research.md`。

- `<target>` 是一个地址（加 `--token` 你就是 owner，完全权限），或者一条
  `…/#g=<token>` 分享链接（访客，受限于主人的可见/可输入白名单：只可见的 pane 是只读，
  不可见的 pane 直接拒绝）。
- `%N`（可选）是要 attach 的 tmux pane id，它选中的是那个 pane 所在的会话。不给的话，
  只有一个会话时自动接，否则（在 TTY 上）从带编号的菜单里选（每行是会话 · agent ·
  状态 · 任务；回车取第一行，`q` 取消）。被管道接走或在脚本里跑（stdin 不是 TTY）时，
  它打印列表并以非零退出。
- 退出用 tmux 自己的 `<prefix> d`，或者 `Ctrl-]`（本地逃生口）。
- 范围由服务端强制，`--read-only` 只是本地的方便开关。设计与取舍见
  `docs/design/remote-attach-research.md`。

> 前提是主机可达：局域网里直连，或者通过 `gtmux tunnel` 从任意网络连（WebSocket 和
> 雷达走同一条隧道）。访客那边完全在菜单栏里配置（按 pane 的 👁 可见 / ⌨️ 可输入 +
> 新建链接），或者用 `gtmux share`。

## `gtmux pair`：接入你自己的设备（全权）

```
gtmux pair                  # mint ONE one-time code, printed three ways:
                            #   phone QR · browser https://…/#c=<code> · a one-line
                            #   `gtmux attach '<url>/#c=<code>'` for another terminal
gtmux pair list             # your paired devices (guests live under `gtmux share`)
gtmux pair revoke <id>      # cut one device off, effective immediately
```

pair 是 pair/share 模型里的 owner 轨：接进来的设备就是你，所有会话上完整的可见和
可输入。三种介质兑换的是同一个短命配对码（5 分钟，一次性），进的是同一份可吊销的名册。
终端那一种把 token 持久化到 `~/.config/gtmux/remotes.json`（0600），之后裸敲
`gtmux attach <host>` 就能用；`pair revoke` 立刻让它失效。`gtmux tunnel` 开着时链接带
隧道地址，否则是局域网地址。`gtmux devices` 仍是这份名册的别名。

### `gtmux devices --push` / `--forget-push`：查看与清理推送 token

```
gtmux devices --push                       # roster annotated with each device's push
                                           #   token (✓ env·kinds) + any UNLINKED tokens
gtmux devices --forget-push <id|orphans|all>  # drop push tokens (host-only)
```

推送 token 绑在注册它的那台已接入设备上，所以 `gtmux devices revoke <id>` 本身就停掉了
那台设备的通知。`--push` 显示这个绑定；`--forget-push` 按选择器清理：一个设备 `id`、
`orphans`（只清未绑定的历史 token，来自还没有设备绑定的时代），或者 `all`。删掉的手机
还在收通知，那是旧 app 从没注销的陈旧 token，用 `orphans` 清。仅主机可用（本地主 token），
远端设备和访客会被拒绝。

## `gtmux share`：给协作者的受限、可吊销访问

```
gtmux share new --label Alice --view %1,%2 --type %1 --expires 24h
gtmux share set a1b2c3d4 --type %2            # edit ONE link (omitted flags untouched)
gtmux share link a1b2c3d4 [--json]            # re-show an existing link's URL (+ QR)
gtmux share code a1b2c3d4 [--json]            # 给粘不了链接的人一个短码
gtmux share on|off                            # consent master switch for ALL guest typing
gtmux share status [--json]                   # per-link scope summaries
gtmux share revoke a1b2c3d4
```

share 是 pair/share 模型里的协作者轨：一条最小权限的访客链接。每条链接带自己的范围：
访客可以看哪些 pane（`--view`）、可以输入哪些（`--type`，必须是 view 的子集），外加
可选的过期时间（`--expires 45m|24h|7d`，默认永不过期；过期的链接和被吊销的一样认证
失败）。输入还额外需要主人的同意（`share on`，默认关）。这一切由服务端强制，网页和
app 只是镜像。

不带 `--view/--type` 生成的链接会复制当前的全局列表（模板）。历史上的全局写法
（`share add/remove`、`share view add/remove/clear`）仍然能用，但会扇出到每一条已有
链接；要改单条链接用 `share set`。`status --json` 带每个访客的
`view_panes`/`panes`/`expires_at`，永远不含裸 token，还有谁用过这条链接：`last_seen`、
`platform`（`Chrome 141 · macOS`）、`last_ip`，有人用过之前都不存在。链接地址在生成时
打印；`gtmux share link <id>`（或菜单栏那行的复制按钮）会把同一条 `#g=` 地址再给你一次
（只对全权调用方）。

### 一条分享链接是什么，两端各自会发生什么

一条分享链接就是一个协作者对这台 Mac 的访问权：能看哪些 pane、能往其中哪几个里打字、以及
一个可选的期限。权限跟着链接走，所以你可以发出去三条，单独吊销其中一条。

你要交出去的就是这条链接。凭证在链接里，它一直有效，直到你吊销它或者到了期限：

```
https://tunnel.example.dev/p35047/#g=<64 个字符>
```

如果对方那边粘不了，`gtmux share code <id>` 会把同一条链接变成你能念出口的东西：一个不含任何
秘密的地址，加一个短码。

```
https://tunnel.example.dev/p35047
96Z-NCC
```

在电话里跟人说、对方在电视浏览器上、或者他那台电脑被锁死了，才用得上它。短码在十分钟内能
打开那条链接一次，用过就废，链接本身不受影响。

浏览器这一端：他打开链接，或者打开地址再输那个码。之后凭证就留在这个浏览器里，明天再来直接
就进。他看得到「可见」清单里的那些 pane，在你的总闸开着的时候（`gtmux share on`）能往更短的
那份「可输入」清单里打字。这台 Mac 上别的东西他碰不到。

终端这一端做同一件事，`gtmux attach <链接>` 或者 `gtmux attach <host> --code 96Z-NCC`，并且
会为那台 host 把 token 记下来，之后直接 `gtmux attach <host>`。他范围里只有一个 pane 就直接
附上去，有好几个就问他要哪个。只能看、不能输入的 pane 会以只读方式附着，并在会话上面那行写
明白。

`gtmux share revoke <id>` 两端一起断：浏览器下一次请求就退回门口页，终端存着的 token 立刻
失效。到了期限也一样，只是时间由期限说了算。你其他的链接不受影响。

浏览器存的东西可能会丢：清了网站数据、开了无痕、换了个浏览器，或者 Safari 那条「一周没人来
就清掉脚本存储」的规则。重新打开那条链接就能恢复，短码不行，因为短码只能用一次。这正是「要
发出去的是链接」的原因：它才是对方以后能自己回来的那个东西。终端把 token 存在
`~/.config/gtmux/remotes.json` 里，你不吊销、他不删，它就一直在。

## `gtmux whatsnew`：对你来说变了什么

```sh
gtmux whatsnew                 # everything newer than the version you're running
gtmux whatsnew --since v0.36.0 # from a specific version
gtmux whatsnew --all           # every release we have notes for
```

每个版本里写给用户的那几行。`gtmux update` 装完之后打印开头几条，这里是完整列表。
来源是发布 tag 信息里的一个 `user:` 块，goreleaser 会把它拷进 release 正文。可选的
`user-zh:` 孪生块用中文写同样的内容：

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

`gtmux update` 和 `gtmux whatsnew` 都打印与你语言匹配的那一块（`GTMUX_LANG`）：中文
优先 `user-zh:`，英文优先 `user:`，某个 tag 只写了一块时另一种语言回退到它。两块先后
顺序随意，每一块在空行、标题行或另一块的标记处结束。没有 `user:` 块的发布什么都不贡献。

## `gtmux config`：少数几个不按次给的设置

```
gtmux config agent-proxy [<url>|off]   # proxy applied when gtmux LAUNCHES an agent
gtmux config tab-alert  [on|off]       # mark the terminal TAB of a session that needs you
gtmux config lang       [en|zh|auto]   # machine-level output language (auto = system locale)
```

不带参数调用时，每一条都打印当前值。

### `lang`：让每个 gtmux 进程说同一种语言

launchd 起的 `gtmux serve` 和 hook 子进程既没有 `GTMUX_LANG` 也没有你 shell 的 locale，
没有机器级选择的话，它们发出的唤醒后缀和桌面通知可能和你终端里的语言不一致。
`gtmux config lang zh` 给它们一次性做了选择；`auto` 跟随系统 locale。按进程的
`GTMUX_LANG` 和按次调用的 `--lang` 仍然优先。

### `tab-alert`：不用挨个点开就找到对的标签页

`tab-alert on` 会在任何有 agent 在等待的会话的标签页标题前面放一个 `●`。

- 只有 `waiting` 会标；`working` 和 `idle` 永远不标。
- 标题由 tmux 渲染、终端只负责显示，所以 Ghostty、iTerm2、Warp、Apple Terminal 上都
  一样有效。它是一个字形，因为给标签页上色是各家终端各自的能力。
- 保留你的标题格式。打开时它读你的 `set-titles-string` 存下来，只在前面接上自己那一段；
  `off` 原样还原。如果你在这期间改过格式，`off` 会拒绝覆盖你的修改，那种情况下请自己
  把开头的 `#{@gtmux_alert}` 删掉。
- 由 agent 自己的 hook 事件驱动：agent 一报告等待，标记立刻落下，serve 的节拍做兜底
  对账。HQ 不在这个回路里。
- 也可以在菜单栏 app 的偏好设置 → 通知里开关。

### `debug`：追问题的时候多记一点

```sh
gtmux config debug            # 现在在记什么
gtmux config debug on         # gtmux 的每个部分都写 debug 条目
gtmux config debug serve,hook # 只记这几个
gtmux config debug off        # 回到常规条目
```

它写在 `config.json` 而不是环境变量里，因为真正需要调高的那几个进程，shell 够不着：
launchd 拉起的 serve、隧道客户端、hook。各个进程下次启动时生效。`gtmux logs --stats`
会说它是不是开着，菜单栏里的「多记一些细节」就是这个设置。

### `hqWake`：调 HQ 的唤醒通道

`~/.config/gtmux/config.json` 里 `"hqWake"` 下面手写的键（全部可选；缺失或非法的键
保持默认）。每一项管什么，写在对应行为所在的地方：
[唤醒通道](#唤醒通道hq-怎么知道事情)、[消费水位](#消费水位为什么不会漏掉东西)、
[自轮换](#自轮换hq-自己的会话成了问题)：

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
| `selfRotateTurns` | 300 | HQ 回合数的越界线（0 关闭） |
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

### `gtmux status`：把舰队放进 tmux 状态栏

```tmux
set -g status-right '#(gtmux status)'
```

一小段带色的计数（`●2 ✓14`）给 tmux 状态行用，你本来就有的那条栏会告诉你谁需要你。
`--plain` 去掉 tmux 的颜色转义，给别的状态栏（或 shell 提示符）用同样的数字。

### 把 pane id 放进标签页标题（可选）

gtmux 在每一块屏上都用 tmux 的 id（`%23`）称呼一个 pane，`gtmux focus %23` 直接吃这个
id。两行配置能让这个 id 在另一边也可见，app 里的一行和你终端里的一个标签页说的就是
同一个东西：

```tmux
set -g automatic-rename-format '#{b:pane_current_path} #{P:#{pane_id} }'
set-hook -g pane-exited 'set-window-option automatic-rename off ; set-window-option automatic-rename on'
```

窗口名于是会列出那个窗口里的每一个 pane（`gtmux %23 %24`），因为 `set-titles-string`
是 `#S — #W`，标签页会继承它。标题格式没变，`focus` 的标签页匹配不受影响。

- `#{P:…}` 遍历的是这个窗口的 pane，所以名字不跟随焦点（原因见
  [TROUBLESHOOTING](TROUBLESHOOTING.md#pane-ids-in-tab-titles)）。
- 那个 hook 是必需的：加一个 pane 会立刻重算窗口名，关掉一个不会。
- 在分屏窗口里，如果你还想让每个 pane 在屏幕上也戴着自己的 id：

  ```tmux
  set -g pane-border-status top
  set -g pane-border-format ' #{pane_id} #{pane_current_command} '
  ```

  `doctor` 不会建议这一条：`pane-border-status` 默认是 `off`，打开它的代价是每个分屏里
  每个 pane 永久占掉一行屏幕。
- `gtmux doctor` 会报这一行，`--fix` 会提议。如果你已经有自己的
  `automatic-rename-format`，`--fix` 会把 id 追加上去。gtmux 不会重命名你的窗口：
  `rename-window` 会把那个窗口的 `automatic-rename` 关掉，并永久覆盖你的格式。

### 让打印出来的链接可点（可选）

程序把超链接打印成 OSC 8 转义，tmux 只把它转发给声称支持这个能力的终端。默认没有终端
声称，链接就渲染成纯文本。

```tmux
set -as terminal-features ',*:hyperlinks'
```

- 需要 tmux 3.4+（`terminal-features` 从 3.2 起就有，`hyperlinks` 这个 feature 是 3.4
  才有）；更老的 tmux 上这一行是启动错误，所以 `doctor` 在低于它的版本上不建议这一条。
- 只有生效之后打印的输出才带链接。
- Ghostty 和 iTerm2 都处理 OSC 8。链接还是点不动的话，下一个嫌疑是终端自己的点击
  修饰键（有些要 ⌘ 或 ⌥ + 点）。

### 给普通 pane 一个值得读的标题（可选）

agent 会自己写 pane 标题（一行 agent 读作 `整理这周的发布清单和回归结果`）；shell
什么都不写，gtmux 回退到命令名，每一行普通 pane 都读作 `bash`。两个 shell hook 能解决：
跑命令时标题是那条命令，回到提示符时是目录名。

```bash
# bash — in the file your LOGIN shell reads (see below)
if [ -n "$TMUX" ]; then
  trap 'printf "\033]2;%s\007" "$BASH_COMMAND"' DEBUG
  PROMPT_COMMAND='printf "\033]2;%s\007" "${PWD##*/}"'"${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
```

```zsh
# zsh — ~/.zshrc
if [ -n "$TMUX" ]; then
  autoload -Uz add-zsh-hook
  gtmux_pane_title_preexec() { print -rn -- $'\e]2;'"$1"$'\a' }
  gtmux_pane_title_precmd()  { print -rn -- $'\e]2;'"${PWD:t}"$'\a' }
  add-zsh-hook preexec gtmux_pane_title_preexec
  add-zsh-hook precmd  gtmux_pane_title_precmd
fi
```

- tmux pane 跑的是登录 shell，登录 bash 读的是 `.bash_profile` / `.bash_login` /
  `.profile` 里第一个存在的那个，从不读 `.bashrc`，所以 bash 版本要写进那三个之一。
  `doctor --fix` 会挑你的 shell 真正会读的那个文件，而且只往已经存在的文件里追加
  （在你用 `.profile` 的机器上创建一个 `.bash_profile` 会把它盖住）。
- 用 `add-zsh-hook`；定义裸的 `preexec()` 会替换掉你（或 oh-my-zsh）已有的。
- 两者都用 `$TMUX` 把关：在 tmux 之外这会和终端自己的标签页标题打架，而 gtmux 正靠
  那个标题跳转。
- `gtmux doctor` 数有多少个普通 pane 的标题说出了点什么，不管是哪个 hook 产生的。
  只对新开的 shell 生效，已经开着的 pane 保持原标题。

## 通知 hook

`⏸ waiting`、`✓ latest` 和点通知跳转都依赖一个把状态文件写到 `~/.local/share/gtmux/`
下的 hook。gtmux 内置了这个 hook：

```sh
gtmux install                       # asks: hooks | app | all
gtmux install hooks                 # Claude, one-time setup (macOS)
gtmux install hooks --agent codex   # or cursor|gemini|copilot|kiro|opencode
gtmux uninstall [hooks|app|all]     # reverse it (asks when no target)
```

`gtmux install hooks` 把 `gtmux hook` 注册到 `~/.claude/settings.json` 的 `Stop`、
`Notification`、`UserPromptSubmit` 三个事件上（幂等；保留其它 hook，给文件留备份）。
`gtmux hook` 是产出方（由 Claude Code 来跑，你不用跑），它纯按事件时序写状态，不读
消息文本就能把权限请求和空闲提醒区分开。

其它 agent：`--agent codex|cursor|gemini|copilot|kiro|opencode|kimi` 改成接那个 agent
自己的 hook 文件。Codex 用的是可叠加的 hooks 系统（`~/.codex/hooks.json` +
`features.hooks`），所以和你已有的 `notify`（比如 computer-use）共存。opencode 没有
命令 hook 文件，gtmux 装一个转发它事件的小 JS 插件（`~/.config/opencode/plugin/gtmux.js`）。
Kimi Code 的 hook 是你自己 `~/.kimi-code/config.toml` 里的 `[[hooks]]` 条目，gtmux 只在
文件末尾追加一整块带标记的内容，其余一个字节不动；卸载也只删这一块。
`gtmux doctor --fix` 会针对探测到的 agent 逐个提议接上。

通知由菜单栏 app 投递，不需要 `terminal-notifier`。hook 把请求排到
`~/.local/share/gtmux/notify/` 下，`Gtmux.app` 弹一条原生横幅（显示为 Gtmux，带 agent
图标和一个 Jump 动作；「跑完了」安静无声，「需要你输入」会响）。点它就落到那个确切的
pane 上。第一次运行时允许通知，并让 app 一直开着才收得到。

### 备份 HQ 的档案

HQ 的档案（整个 HQ 目录）是 gtmux 手上唯一不可再生的东西：态势板、知识库，以及一份
种一次、永不覆盖的 `LOCAL.md`（丢了不会自愈）。

```sh
gtmux hq --export ~/gtmux-hq.tar.gz   # the whole records folder as one LOCKED file (asks for a passphrase → .tar.gz.age)
gtmux hq --import ~/gtmux-hq.tar.gz.age   # put one back (asks for the passphrase)
gtmux hq --export ~/gtmux-hq.tar.gz --plain   # the unlocked form
```

`gtmux hq --records [--json]`（`--memory` 是旧写法）说档案有多大，以及有没有任何东西
把它带离这块盘。菜单栏的阅读器显示同一行、来自同一条命令，两块屏不会对同一个数字说
不同的话。

导出的是普通 tar.gz，用 [age](https://age-encryption.org) 格式加口令上锁（1.0.21 起），
任何 age 工具都能解开。锁上在离开这台机器的那份副本上；机器里面有 FileVault 管着磁盘，
知识库本身照旧不动。口令在终端里输两次、不回显、至少 8 位；脚本用
`GTMUX_HQ_PASSPHRASE`，app 用 `--passphrase-stdin` 从标准输入第一行递，永远不走命令行
参数（那儿 `ps` 看得见）。口令丢了文件就打不开：gtmux 不留副本。`--plain` 写不上锁的
tar.gz。`--import` 看文件头就知道是哪种，需要时才问口令，口令不对什么都不动。
`--records` 还会说上次导出是什么时候、有没有上锁；菜单栏读窗口在一张表单里问口令，
可以记进 Mac 的钥匙串。

`--import` 绝不就地覆盖：已有的档案会被挪到 `hq.replaced-<timestamp>`，路径会打印出来。

`gtmux serve` 每天快照到 `~/.local/share/gtmux/hq-snapshots/`，保留 14 份，只在档案
真的变了时才写。快照放在状态目录里，在 HQ 目录之外。

**快照挡的是误操作，挡不住硬盘坏掉。** 它们和数据在同一块盘上。`gtmux doctor` 的
`HQ 档案` 那行会说有多少东西、积累了多久、有没有任何东西把它带离这块盘。

## 权限

gtmux 只要它需要的：

- 自动化（控制你的终端：Ghostty / iTerm2 / Warp），`focus` / `restore` / `new` 和
  点通知跳转需要它。gtmux 第一次通过 AppleScript 驱动终端时 macOS 会弹窗，点允许。
- 通知，菜单栏 app 才能弹 agent 横幅。第一次运行时允许。
- 开机自启（可选），只在你到偏好设置里打开时才要。

下面这些它不需要；macOS 要是弹了，可以放心拒绝，不损失任何功能：

- App 管理（「修改你 Mac 上的 app」）。gtmux 从不修改别的 app，它的代码只碰自己的
  bundle（更新/卸载时）。这个弹窗可能出现在 macOS 通过 responsible-process 链把另一个
  app 的自更新归到 gtmux 那个长跑的后台进程上时。拒绝对 gtmux 没有影响。
- 文件与文件夹（下载 / 桌面 / 文稿）。gtmux 不读这些。弹窗可能出现在 `restore` 重建
  一个工作目录恰好在其中之一的 tmux 会话时，那是 `tmux`（由 gtmux 启动）在打开那个
  文件夹。可以放心拒绝。
