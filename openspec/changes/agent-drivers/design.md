# Design — agent-drivers(tmux 降级为 UI + per-agent driver 分层)

本文回答四件事:①现状调查(每条通道今天的真值是什么、弱在哪);②分层架构与
driver 接口定义;③每条通道的升级路径(含与已归档 change 的边界);④风险、回退
与在途工作对齐。**指导原则(司令拍板):考虑全面,不贸然改核心机制——每一期都
只是给既有机制换一个更硬的证据来源,机制本身(队列、claim、重试、两帧、interlock)
一律不动。**

---

## 0. 现状调查 — 四条通道的真值与弱点

### 0.1 send 投递与落地校验(`internal/dispatch` + `internal/dispatchbridge`)

现状(已核对代码):

- 投递:paste(`load-buffer` + `paste-buffer -p`)与 Enter 严格分步
  (`internal/tmux/tmux.go:187-200`, `deliver.go:104-114`);paste 前有片段守卫
  (`pasteWithGuard`/`confirmPaste`,head+tail 双指纹)与 re-send interlock
  (FNV 指纹 + 90s 窗口,`resend.go`)。
- 校验分层已存在:hook-equipped agent(`dispatchbridge.go:20-23` 白名单)优先读
  events.jsonl,`UserPromptSubmit` 且 `headsMatch` 即判 landed
  (`deliver.go:126-133`);fallback 是 200 行 scrollback 全屏两帧一致判定
  (`deliver.go:145-160`),超时 15s 判 failed(`deliver.go:163-165`)。

弱点(误判 NOT delivered 的三个具体来源):

1. **无终局复查。** 超时判 `failed` 时不再读一次事件流——事件在最后一轮轮询与
   超时之间到达即被丢弃,校验以屏读为最终真值,与 spec("stream 是权威")相悖。
2. **head 归一化双轨。** hook 侧写入的 `Summary` 是 `NormalizeHead(CleanUserPrompt(prompt))`
   (`hook/summary.go:47-56`),Deliver 侧比对的是 `NormalizeHead(原始 payload)`;
   清洗规则(剥 harness 注入、slash 包装)只作用于一侧,长 payload 或含特殊前缀的
   payload 可能 `headsMatch` 失败 → 事件被无声忽略 → 落入屏读。
3. **判定不携带来源。** `Result` 不区分"事件确认"与"屏读推断",误判发生时无法
   从证据上区分是哪一层错了,HQ/用户只能靠事后对时间线。

### 0.2 wake/nudge 通路(`internal/hqnudge` + `internal/hqwake` + `internal/hqpane`)

现状:`hq-wake-reliability`(已归档,25/25 任务完成)已交付磁盘队列
(一文件一 nudge,文件名编码 due/prio/key/attempt)、claim(`.txt`→`.sending`
原子 rename)、orphan 60s 回收、at-least-once + `#id` 幂等、8 行/800 字符/200 条
上限、3s fast tick、`wake-degraded` 升级(fail≥3 或 due 条目卡 >10min)。

弱点:**ack 的唯一证据是屏读**(`deliverPayload`:Enter 后 300ms 读 full capture,
`#id` 在 history 且不在 draft 即 delivered,`hqnudge.go:570-587`)。今天实锤的
失败模式:paste 成功、Enter 被吞 → `#id` 留在 draft 区 → `unacked`;更糟的是
我们自己的 paste 从此堵住 draft-guard(`boxEmpty` 永远非空),后续所有 drain
停摆,fail 计数不涨,只能靠 10 分钟 stale 判据兜底升级。屏读 ack 分不清
"没落屏"与"落了草稿没提交",于是 `requeueUnacked` 只能整批重试(≤3 次),
既慢又有重复贴入风险。

### 0.3 radar 采集与 digest(`internal/radar` + `internal/hook` + `internal/transcript`)

现状:感知已经是**双源**的,且结构化源已占真值高地——

- `waiting` 从不来自屏幕:hook 状态文件(`waiting/<pane>`)是权威
  (`agents.go:833-847`),屏幕只用于 stuck-dispatch 特例与 stale 校正。
- digest 的 goal/last 来自 per-agent JSONL transcript parser
  (`internal/transcript/{claude,codex}.go`),ask 来自实时抓屏菜单解析。
- working/idle 分类是屏幕启发式:盲文 spinner 字形、`✳` idle 字形、帧哈希、
  子树 CPU(`agents.go:263-314,539-577`)。
- per-agent 知识已散落成四张表:`hookAgents` 白名单(dispatchbridge)、
  `startupGates`/`bootBanners`(prompt)、`agentEventSemantics`(hook/classify)、
  `agentProfile`(radar/agents.json)。

弱点:这些表没有统一的能力模型;一行 digest 是 driver 级(hook+transcript 都在)
还是抓屏级(纯 capture 推断),消费者(HQ、手机、菜单栏)无从得知,只能同等对待。

### 0.4 spawn 就绪探测(`dispatchbridge.WaitAgentReady` + `internal/prompt`)

现状(`hq-send-delivery-reliability` 已归档交付):两阶段门——前台命令离开
shell 集,然后 `IsComposerReady`(prompt 字形在 + 无信任门 + 无 boot banner)
且两帧 byte-identical,300ms 轮询、20s 超时、超时拒贴附证据。

弱点:纯屏幕签名。banner/门是 per-agent 正则表,TUI 改版即漂移;"两帧相同"在
慢启动(MCP 连接抖动)下会长时间不满足;而 Claude 的 `SessionStart` hook 事件
(gtmux hook 已处理该事件,`hook.go` decide 分支)是一个现成的、确定性的
"会话已起来"正向信号,今天完全没有用于就绪判定。

### 0.5 exec 税

所有 tmux 调用都是独立 `exec.Command`(`internal/tmux`,无池化);本机实测每次
~145ms 安全软件税。校验循环 300ms 一次 capture、wake ack 一次 full capture、
draft-guard 两帧 color capture——热路径全在交税。结构化通道(读
events.jsonl / transcript / 状态文件)是纯文件读,零 exec。**注意:exec 批量化
本身是另一会话的在途工作(radar 批量化),本 change 只通过"少抓屏"间接减税,
不改 `internal/tmux`。**

---

## 1. 分层架构

```
              ┌────────────────────────────────────────────────────┐
   对外模型   │  digest 字段 · tasks · spawn/send 语义 · wake 类别   │  ← 统一不变
              └──────────────────────┬─────────────────────────────┘
                                     │
              ┌──────────────────────┴─────────────────────────────┐
   第 2 层    │  per-agent driver(可选能力,缺席即落回第 1 层)       │
              │  Claude Code: Receipt·State·Content·Ready·Headless │
              │  Codex:      Receipt·State·Content·Headless        │
              │  其它 agent:  (无能力声明 → 全部落回第 1 层)          │
              └──────────────────────┬─────────────────────────────┘
                                     │  仲裁规则:正向证据单调
              ┌──────────────────────┴─────────────────────────────┐
   第 1 层    │  tmux 通用底座(永久保留,永不移除,可整体强制回退)     │
              │  pane 生命周期 · capture-pane 抓屏 · paste+Enter 键入 │
              └────────────────────────────────────────────────────┘
```

三条不变式(写入 `agent-driver` spec,作为长期约束):

- **I1 — 第 1 层永久兜底。** 任何通道在 driver 能力缺席、出错、被配置关闭时,
  必须走与今天逐字节一致的抓屏/键入路径。driver 永远是"额外证据",不是"替代
  机制"。
- **I2 — 正向证据单调。** driver 级证据只用于**确认成功**(落地/就绪/完成),
  绝不单独用于宣告失败;屏读证据不得推翻 driver 已确认的成功。宣告任何失败前,
  必须对 driver 证据源做终局复查。(反向不成立:driver 无事件 ≠ 失败,只是
  降级到第 1 层判定。)
- **I3 — 对外模型不变。** driver 是实现细节;所有对外字段/语义/时序契约照旧,
  升级只允许做加法(`sense` 字段、`--oneshot` flag)。

### 1.1 driver 接口定义(`internal/driver`,新叶子包)

沿用仓库的注入式风格(参照 `dispatch.IO`):driver 是**一组可选函数**,nil 即
该能力缺席。注册表按 agent key(与 hook `--agent`、radar profile 同一命名域)
解析;所有函数只读 agent 已产出的事实(events.jsonl、transcript、状态文件、
exec JSON),**不与 agent 进程建立任何会话/连接**(non-goal:无 wrapper)。

```go
package driver

// Verdict 是 Receipt 的三值判定:确认落地 / 确认未提交(paste 在、submit 无)/ 无证据。
type Verdict int
const (
    NoEvidence Verdict = iota // driver 帮不上 —— 落回第 1 层
    Confirmed                 // 事件流确认已提交(终局,不可被屏读推翻)
    Unsubmitted               // 事件流确认 paste 到位但未提交(精确的"回车被吞")
)

type Driver struct {
    Name string

    // Receipt:投递回执。needle 是归一化 head(与 hook 落盘的 Summary 同一套
    // 归一化管线 —— 单轨化是 P1 的一部分);since 界定事件窗口。
    Receipt func(pane string, needle string, since int64) Verdict

    // State:状态真值。ok=false 即落回屏幕分类。
    State func(pane string) (st StateSnapshot, ok bool)

    // Content:goal/last/ask 的结构化来源。今天的 transcript.Load 即其实现。
    Content func(session SessionRef) (ContentSnapshot, bool)

    // Ready:spawn 就绪正向信号(如 SessionStart 事件)。ok=false 落回屏幕门。
    Ready func(pane string, since int64) (ready bool, ok bool)

    // Headless:一次性 worker 的启动命令与输出解析器(claude -p / codex exec)。
    Headless *HeadlessSpec
}

// For 返回 agent 的 driver;未注册或被配置关闭 → 零值 Driver(全能力 nil)。
func For(agentKey string) Driver
```

设计取舍:

- **struct-of-funcs 而非 interface**:与 `dispatch.IO` 同构,能力可单独为 nil、
  单独被配置关闭、单独在测试中注入 fake;避免 interface 断言散落。
- **注册表收拢四张散表**:`hookAgents`(→ Receipt/State 非 nil 即 hook-equipped)、
  `startupGates`/`bootBanners`(仍属第 1 层 prompt 包,但按 driver.Name 索引)、
  transcript parser 选择(→ Content)、`agentProfile` 保持在 radar(显示层,
  不迁移)。首期(P0)是纯收拢:行为零变化,`hookEquipped()` 改为
  `driver.For(key).Receipt != nil`,值不变。
- **配置开关**:`driver.enable`(总开关)与 `driver.<agent>.<capability>`
  (如 `driver.claude.receipt: off`)。关闭 = 对应函数视为 nil = I1 生效。

### 1.2 感知档位(digest `sense` 字段)

digest 每行新增可选字段 `sense`,标注该行的感知档位(加法,omitempty):

- `driver` — 状态来自 hook 状态机 **且** 内容来自 transcript(双通道在位);
- `partial` — 两者只有其一(如 hook 在但 transcript 尚无 / 反之);
- `screen` — 纯抓屏/进程树推断(第 1 层)。

判据完全来自既有事实(waiting 标记来源、`sessionRef` 是否解析出 transcript),
不新增采集。消费端(HQ playbook、手机、菜单栏)可据此加权信任度;本 change 只
交付字段与 CLI/文档,消费端 UI 不动(等档位数据积累后另行迭代)。

---

## 2. 各通道升级路径(与已归档 change 的边界)

### 2.1 投递回执 —— send/spawn 落地校验(P1,痛点最大、改动最小)

**定位:修 `agent-dispatch` 既有"Layered verification"要求的执行缺口 + 收紧
仲裁,不新建校验机制。**

1. **归一化单轨。** hook 落盘 Summary 与 Deliver 比对 needle 走同一条管线
   (`driver` 包暴露 `NormalizeNeedle`,两侧共用),消除 0.1-弱点 2。
2. **终局复查。** `Deliver` 在宣告 `failed` 之前,强制再读一次
   `Receipt(pane, needle, start)`;`Confirmed` → landed(哪怕屏读从未看见)。
   消除 0.1-弱点 1。
3. **正向单调仲裁。** 循环内一旦 `Confirmed` 即终局返回(现状已如此);屏读
   `landed` 判定保持两帧规则不变;新增:屏读永远不能把 `Confirmed` 改判。
4. **判定来源标注。** `Result` 增加 `JudgedBy: driver|screen`(`--json` 透出,
   附 evidence);误判再发生时可以一眼定位错误层。
5. **`Unsubmitted` 精化补 Enter。** 屏读 fallback 里"草稿仍持有 payload"的
   判定(`draftHasDelivery`)之外,`Receipt` 返回 `Unsubmitted`(有 paste 痕迹、
   无 submit 事件)时同样走既有的退避补 Enter 路径——补 Enter 机制、次数、退避
   全部沿用现状。

不动:paste/Enter 分步、片段守卫、re-send interlock、`PasteAndSubmit`
(API/`--no-verify` 路径)、超时与重试参数。

实现注记(P1 落地校准):①`NormalizeNeedle` 落在 `dispatch` 包(dispatch 依赖
transcript 叶子;driver 依赖 dispatch 的纯匹配函数,dispatch 永不依赖 driver——
Deliver 的事件证据经 `dispatch.IO.Events` 注入,保持无环与可注入测试)。
②events 回执对**全部 hook-equipped agent** 注册同一实现:证据是 gtmux 自己的
hook 写入的流记录,与 agent 无关——只注册 Claude/Codex 会让其余 hook agent 丢失
既有事件优先校验(违反零回归);司令拍板点(Codex 不延后)被此超集覆盖。
③事件流看不见草稿,`eventsReceipt` 只产 Confirmed/NoEvidence;`Unsubmitted`
的生产者是能看见"paste 在、submit 无"的证据源(P2 wake 的 draft 检查)。

### 2.2 wake ack —— `hq-wake-reliability` 的扩展(P2)

**定位:显式扩展已归档的 `hq-wake-reliability`,只换 ack 的证据来源;队列、
claim、orphan 回收、优先级、上限、fast tick、degraded 升级全部不动。**

HQ 是 hook-equipped Claude 会话,wake 行以 `#id` 结尾——`deliverPayload` 的
ack 升级为三层:

1. **driver 回执优先**:Enter 后查 `Receipt(hqPane, "#id", enterTs)`;
   `Confirmed`(HQ 会话出现含该 `#id` 的 `UserPromptSubmit`)→ delivered。
   这是确定性回执,不受"HQ 一回车就滚屏 200 行"影响。
2. **`Unsubmitted` → 只补 Enter**:driver 判定 paste 到位而未提交(或屏读见
   `#id` 在 draft 区)→ 新增一个精确状态:**不 requeue 整批、不重贴 payload**,
   由下一次 drain 对同一 claim 只补发 Enter(复用 dispatch 的"swallowed Enter
   只在草稿仍完整时补 Enter"纪律)。这直接消灭今天的实锤失败模式:回车被吞
   不再堵死通道等 10 分钟 stale 兜底,3s fast tick 的下一拍就补上。
3. **屏读兜底不变**:driver 无证据(如 HQ 换成非 hook agent)→ 现状的
   history/draft `#id` 屏读判定,`unacked` ≤3 次重试照旧。

`#id` 幂等语义不变(at-least-once、playbook 忽略重复 id);`wake-degraded`
判据不变(fail≥3 或 stale>10min),但因为 2 的存在,stale 兜底应当极少再触发。

### 2.3 spawn 就绪 —— driver 正向信号(P3)

**定位:给 `hq-send-delivery-reliability` 交付的屏幕就绪门加一个更快、更硬的
正向短路,门本身不动。**

- Claude driver 的 `Ready`:spawn 记录启动时刻,`SessionStart` 事件(该 pane、
  时刻之后)出现 → 会话确定已起来;此后只需**一帧** `IsComposerReady`(信任门/
  banner 若在,SessionStart 根本不会发)即判 READY——把"两帧 byte-identical"
  的等待短路掉,慢启动(MCP 抖动导致屏幕久不稳定)不再拖满 20s。
- 无 `Ready` 能力或事件未到:现状屏幕门逐字节不变(两帧稳定 + 超时拒贴)。
- I2 适用:`SessionStart` 缺席**不是**失败证据(旧版 agent hook 不发它),
  只是不短路。

### 2.4 radar 状态/digest 内容 —— 显式化 + 档位标注(P4)

radar 的 waiting 权威(hook 状态文件)与 digest 的 transcript 内容今天已是
driver 级,本期只做两件事:

1. 状态/内容来源改经 `driver.For()` 取用(实现仍是原函数;纯接线收拢)。
2. digest 行计算并透出 `sense`(§1.2);`docs/cli.md` + `api/contract.md` 同步。

**明确不做**:working/idle 屏幕分类(spinner/帧哈希/CPU)不迁移、不重写——它
是第 1 层的核心资产,对 hook-less agent 是唯一信号。

### 2.5 一次性 headless worker(P5,最后、最克制)

`gtmux spawn --oneshot <goal>`:

- 仅当 `driver.For(agent).Headless != nil`(Claude:`claude -p --output-format
  stream-json`;Codex:`codex exec --json`),否则**明确拒绝**并提示去掉
  `--oneshot`(不静默降级,避免用户以为拿到了结构化保证)。
- **仍寄宿 tmux pane**:命令跑在新 pane 里,stdout(JSON 流)对用户可见、radar
  一行照常、reap 照常——保住双通道的"可看"半边;"可接管"半边天然缺失
  (非交互进程),这是 `--oneshot` 的显式契约,文档写明。
- 状态真值来自流式 JSON(result/error 事件)与进程退出码:done/crash 不经屏幕
  分类;`sense: driver`。投递回执天然免除(goal 是 argv,不存在 paste 落地问题)
  ——这条路径把 §0 的所有屏幕启发式整体归零,是"结构化到底"的样板间。
- ledger/tasks/digest 语义照旧;`--worktree`/`--title` 等 flag 正交可组合。
- 环境纪律:launch 前清除会递归触发 hook 的环境(沿用 multiplexer-research ⭐B
  的调查结论)。

---

## 3. 风险与回退

| 风险 | 缓解 | 回退 |
|---|---|---|
| driver 正向证据本身出错(如事件流被无关 submit 污染)→ 假 landed | Receipt 仍要求 needle 匹配(归一化单轨后更严);wake ack 匹配全局唯一 `#id`;事件窗口以 `since` 时刻界定 | `driver.<agent>.receipt: off` → 逐字节回到屏读校验 |
| 归一化单轨改动影响既有 hook 消费者(events.jsonl 的 Summary 字段) | Summary 落盘格式不变,只统一 Deliver 侧的比对管线;fixture 测试钉住两侧等价 | P1 独立可回退 |
| `SessionStart` 语义漂移(agent 版本差异) | I2:事件只短路、缺席不改变行为;屏幕门保留 | `driver.claude.ready: off` |
| wake"只补 Enter"在 draft 已被用户手动改动时误提交 | 补 Enter 前沿用 dispatch 纪律:draft 仍完整持有 `#id` 批文本才补;否则按现状 requeue | `driver.claude.receipt: off`(wake 侧同门) |
| oneshot 流式 JSON 格式随 agent 版本变化 | 解析容错(未知事件忽略,退出码兜底);oneshot 是显式 opt-in,不影响默认路径 | 不用 `--oneshot` 即完全无此面 |
| events.jsonl 轮转/滞后导致 Receipt 短暂盲区 | 现有 HookGrace 语义保留;盲区内自动落第 1 层(I1) | 无需回退,天然降级 |
| 新包引入 import 环 | `internal/driver` 定位为叶子(只依赖 events/transcript/state 等既有叶子),遵守 `app → hq → {radar, dispatchbridge} → leaves` 无环规则,CI 的架构 conformance 检查覆盖 | — |

**与在途工作对齐**(本设计只读 main,不与以下分支抢文件):

- `hq-capture-loop`(在途 change):知识捕获闭环,交集仅在 spawn 的 advisory
  输出(KB echo);P5 的 `--oneshot` 不触碰 advisory 输出结构,后合者做一次
  机械 rebase 即可。
- radar 批量化 / restore 回归锁 / doctor 归一化(另一会话):本 change 不改
  `internal/tmux`、不改 restore/doctor;radar 侧只加 `sense` 计算(纯函数,
  与批量化正交)。若批量化先合,P4 直接受益(少一次 capture 更划算)。
- `hq-wake-reliability` 及其后续(`done-wake-keyed-on-awaited`、
  `hq-send-delivery-reliability`,均已归档):本 change 是它们的**证据层升级**,
  所有已交付机制被复用而非重建;spec delta 以 MODIFIED 叠加,不回滚任何既有要求。

**渐进性保证(每期独立可发布、可回退):** P0 零行为变化;P1–P4 各有独立
kill-switch,关闭即逐字节回到现状;P5 是纯新增面。任何一期可以单独跳过或搁置,
不阻塞其它期(P2 依赖 P1 的 Receipt 实现,P3–P5 只依赖 P0)。

---

## 4. 开放问题(已由司令拍板,2026-07-23)

1. **`sense` 三档还是两档?** → 按设计:三档(driver/partial/screen),消费端可折叠。
2. **`--oneshot` 命名** → 按设计:`--oneshot`,help 里写明与 `--headless` 的区别。
3. **Codex 的 Receipt/State** → **首期(P1)就开**:Claude 与 Codex 的 Receipt
   同批注册(表驱动);Codex 事件密度低只影响命中率,不影响正确性(NoEvidence
   自动落第 1 层,符合 I2)。
4. **wake"只补 Enter"与 dispatch 侧共用实现** → 按设计:共用。

## 5. kill-switch 语义澄清(实现校准)

- 开关关闭(`driver.enable: false` 或 `driver.<agent>.<capability>: false`)的
  含义是**该通道只走第 1 层(纯抓屏/键入)路径**——即 spec 所述 "restoring
  Layer 1 behavior"。对 Receipt 而言,关闭 = Deliver 无事件通道、纯屏读校验:
  这是比 main 现状(hook-first + 屏读)**更保守**的回退面,用于隔离事件通道
  本身的故障。
- P0 阶段注册表只收拢事实(hook-equipped 白名单),尚无任何 driver 能力函数,
  既有 hook-first 校验属**基线行为**、不受开关影响——因此 P0 是严格零行为
  变化;从 P1 起 hook-first 校验收拢为 Receipt 能力,开关才开始作用于它。
