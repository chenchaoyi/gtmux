# Change: agent-drivers — tmux 降级为 UI,感知/驱动走 per-agent driver 分层

## Why

gtmux 对 agent 的感知与驱动今天**过度依赖"抓屏 + 键入"**:投递靠 paste+Enter、
校验靠 `capture-pane` 两帧比对、就绪靠 banner/信任门正则、wake 回执靠在 scrollback
里找 `#id`。这条腿是通用的(任何终端 agent 零集成即可管理),但它天生是**启发式**
的,而近两周的可靠性工单几乎全部是在给它打补丁,且同类问题仍在复发:

- **send 落地校验反复误判 NOT delivered。** `dispatch.Deliver` 的分层校验里
  hook 事件(`UserPromptSubmit`)本应是 hook-equipped agent 的真值
  (`internal/dispatch/deliver.go:126-133`),但超时判 `failed` 前没有对事件流做
  终局复查,事件与屏读之间也没有明确的仲裁优先级——实践中出现"事件流里
  `UserPromptSubmit` 早已证明落地,校验却以 tmux buffer 比对为真值报 NOT
  delivered"的误判。`send-submit-reliability`、`hq-send-delivery-reliability`
  两个归档 change 加固的都是屏读侧,结构性缺口(真值仲裁)没有动。
- **wake 通道的 ack 只有屏读一种证据。** `hq-wake-reliability` 已给 wake 加上
  ack+重试+claim 回收(`internal/hqnudge`),但 ack 本身仍是"在 200 行 scrollback
  里找 `#id`"。今天实锤一条信号线回车被吞:paste 落进 HQ composer、Enter 未生效,
  batch 搁浅在草稿区,后续每次 drain 都被 draft-guard 挡住,fail 计数不涨,只能靠
  10 分钟的 stale 兜底才升级 `wake-degraded`。而 HQ 自己就是一个装满 hook 的
  Claude 会话——它的 `UserPromptSubmit` 事件本可以给出**确定性的**投递回执,并把
  "回车被吞"从猜测变成精确诊断(paste 事件在、submit 事件不在)。
- **draft 检测/就绪探测是屏幕启发式的持久战场。** dim 幽灵建议(SGR 2)误判草稿
  刚修过(`draft-detect-excludes-ghost-suggestion`);spawn 首发靠 banner/信任门
  正则表 + 两帧稳定判就绪(`dispatchbridge.WaitAgentReady`),每换一个 agent、
  每改一版 TUI 就要补一张表。
- **每次 tmux exec 在本机有 ~145ms 的安全软件税。** 校验循环每 300ms 一次
  `capture-pane`、wake ack 一次 full capture、draft-guard 两帧——全是独立
  exec(`internal/tmux` 无复用机制)。结构化通道(读 events.jsonl / transcript
  文件)天然绕开这笔税。

同时,gtmux 的两大优势**必须无损保留**:

1. **双通道**:用户随时跳进 pane 亲自接管——agent 永远活在 tmux pane 里,
   gtmux 的驱动与用户的键盘是同一个入口。
2. **agent 无关性**:任何终端 agent 丢进 pane 即被管理,零集成。

关键事实是:gtmux 其实**已经**长出了半套结构化通道——hook 事件流
(`events.jsonl`,投递回执与状态真值)、per-agent transcript parser
(`internal/transcript/{claude,codex}.go`,goal/last 已来自 JSONL 而非抓屏)、
per-agent 事件语义表(`internal/hook/classify.go`)、agents.json profile、
per-agent 门/banner 表(`internal/prompt`)。它们散落各处、各自为政,没有统一的
能力模型,也没有"结构化证据 > 抓屏证据"的仲裁规则。本 change 把它们收拢成显式的
**agent driver 分层**,而不是新建一套感知系统。

## What Changes

把感知/驱动分成两层,tmux 从"感知与驱动的真值来源"降级为"UI 与通用底座":

- **第 1 层 — 通用底座(永久保留,永不移除):** tmux pane 生命周期 + 屏幕抓取 +
  键入。任何 agent 零集成即用;是所有通道的兜底路径,并有配置开关可强制全量回退。
- **第 2 层 — per-agent driver:** 有结构化接口的 agent 按能力逐通道升级质量。
  driver 是一组**可选能力**(每项能力缺席即落回第 1 层):
  - **投递回执(Receipt)**:hook 事件流确认 payload 已提交——正向证据单调
    (driver 确认过落地,屏读不得再翻案为 NOT delivered);判 failed 前必须对
    事件流做终局复查;判定结果标注来源层。
  - **状态真值(State)**:hook 状态机(waiting/working/idle)优先于屏幕分类
    ——radar 今天已大体如此,予以显式化。
  - **内容(Content)**:JSONL transcript 提供 goal/last/ask——digest 今天已
    如此,予以显式化。
  - **就绪(Ready)**:会话启动事件(如 Claude 的 `SessionStart` hook)作为
    spawn 就绪的正向信号,banner/信任门屏读降为兜底。
  - **一次性 worker(Headless)**:`claude -p --output-format stream-json` /
    `codex exec --json` 跑一次性任务,结构化输出即状态真值——仍寄宿在 tmux
    pane 内(保留可见性与 radar 一行),显式 opt-in。
- **Claude Code driver**(能力最全:Receipt/State/Content/Ready/Headless)、
  **Codex driver**(exec 非交互 + JSON 输出;hook 部分沿用现有 classify 表)、
  其它 agent 自动落回第 1 层——**对外行为与今天完全一致**。
- **wake 通道的 ack 升级为 driver 回执优先**(本部分是已归档
  `hq-wake-reliability` 的扩展,不重建其队列/claim/重试机制):HQ 会话的
  `UserPromptSubmit` 事件含 `#id` 即 ack;无事件 + 屏读见 `#id` 在草稿区 =
  精确判定"回车被吞",只补 Enter、不重投 payload。
- **对外模型统一不变**:digest 字段语义 / tasks / spawn / send / wake 类别与
  语义全部照旧,driver 是内部实现。唯一的对外加法:digest 行新增可选字段
  `sense`(`driver` / `partial` / `screen`),标注每船的感知档位,让 HQ 与用户
  知道一行数据的可信度来自结构化通道还是抓屏。

## Capabilities

### New Capabilities

- `agent-driver` — driver 分层模型本身:能力注册表、第 1 层永久兜底不变式、
  正向证据单调性、配置回退开关、对外模型不变约束。

### Modified Capabilities

- `agent-dispatch` — 落地校验的分层仲裁收紧(终局事件复查、判定来源标注、
  driver 正向证据不可被屏读推翻);spawn 就绪增加 driver 正向信号;新增
  一次性 headless dispatch。
- `hq-wake-protocol` — wake ack 允许由 driver 回执(HQ 会话的 submit 事件含
  batch id)确认,屏读降为兜底;"paste 落地而 Enter 被吞"成为可精确判定、
  只补 Enter 的状态。
- `agent-digest` — 新增可选 `sense` 字段(感知档位标注),纯加法。

## Non-goals(防止范围膨胀)

- **不移除、不绕过第 1 层任何路径。** 抓屏 + 键入是永久兜底;每个 driver 能力
  都有配置开关可单独关闭,关闭后行为与今天逐字节一致。
- **不改变任何既有对外契约字段的语义。** `agents --json` / `digest --json` /
  `tasks` / wake 类别 / CLI 语义照旧;只做加法(`sense` 字段、`--oneshot`)。
- **不做常驻 agent 代理进程 / PTY 中间层。** 先前调研(RESEARCH-prior-art,
  Omnara 教训)已否决 wrapper 路线:太脆。driver 只消费 agent 已经产出的
  结构化事实(hook 事件、transcript 文件、exec JSON),不插入进程中间层。
- **不做交互式 SDK 长连接接管。** Headless 仅限一次性 worker;交互式会话
  永远是 TUI in pane。
- **不做 native(非 tmux)会话的 driver 升级。** native 行保持 sense-only。
- **不动 `POST /api/send` 快路径**(手机延迟预算,维持不做落地校验)。
- **不为无 hook 的 agent 发明检测。** no hook = no signal 仍成立;它们完整地
  活在第 1 层。
- **不在本 change 里做 tmux exec 批量化 / radar 批量化 / restore 回归锁 /
  doctor 归一化**——这些在另一会话在途;driver 减少 exec 次数是副产品,不是
  本 change 的目标,也不触碰其代码。

## Impact

- 新包 `internal/driver`(能力注册表;纯收拢,首期零行为变化)。
- `internal/dispatch` / `internal/dispatchbridge`(校验仲裁、就绪信号、
  判定来源标注)。
- `internal/hqnudge`(ack 的 driver 回执优先;队列/claim/重试机制不动)。
- `internal/radar`(digest `sense` 字段;状态真值优先级显式化)。
- `internal/app`(`spawn --oneshot`;driver 配置开关)。
- 契约:`digest --json` / `GET /api/digest` 加可选 `sense`;`spawn --json`
  增量字段。CLI 面:`spawn` 新 flag,无新命令。
- 配置(全部可选、可回退):`driver.enable`(总开关,默认 on)、
  `driver.<agent>.<capability>` 细粒度开关。
- 文档:`docs/cli.md`(sense 档位、`--oneshot`)、CLAUDE.md 契约行、
  `api/contract.md`(digest 加字段)。
- 分期发布:6 个独立可发布、可回退的阶段(见 `tasks.md`);任何一期出问题,
  关对应开关即回到今天的行为。
