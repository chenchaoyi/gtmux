# Tasks — agent-drivers(每期独立可发布、可回退;第 1 层路径始终可用)

依赖关系:P0 → P1 → P2;P3/P4/P5 仅依赖 P0,可乱序。任何一期可单独搁置,
不阻塞其它期。每期收尾都要求 `make check` 绿 + 对应 kill-switch 验证
(关闭开关后行为与 main 逐字节一致)。

## P0 — driver 注册表(纯收拢,零行为变化)

- [x] 0.1 新建 `internal/driver` 叶子包:`Driver` struct(能力字段随各期落地,
      P0 声明 `Receipt`)、`Verdict` 三值、`For(agentKey)` 注册表、
      配置开关解析(`driver.enable`、`driver.<agent>.<capability>`)
- [x] 0.2 收拢现有事实为首批注册项:`hookAgents` 白名单(claude/codex/gemini/
      cursor/cursor-agent/opencode/copilot/kiro)→ 注册表 `HookEquipped`;
      行为不变(transcript parser 的 Content 收拢移至 P4,见 4.3)
- [x] 0.3 `dispatchbridge.hookEquipped()` 改经 `driver.For()`,值逐一相等
      (新 fixture 测试 + 既有 `TestHookEquipped` 双重钉住)
- [x] 0.4 import 无环验证:`internal/driver` 仅依赖 `usercfg` 叶子;
      `make check`(gofmt+vet+staticcheck+race)+ `CGO_ENABLED=0` 构建通过
- [x] 0.5 验证 P0 零行为变化:注册表只收拢 hook-equipped 事实(基线行为,
      不受开关影响);开关解析就位但在 P0 无作用面(design §5)

## P1 — 投递回执仲裁(send/spawn 落地校验;修误判 NOT delivered)

- [x] 1.1 归一化单轨:`dispatch.NormalizeNeedle`(落在 dispatch 而非 driver,
      保持 dispatch 不依赖 driver 的无环方向)统一 hook 落盘 Summary 与
      Deliver 比对两侧——hook 侧改为字面调用同一函数;fixture 断言两侧对同一
      payload 产出相等指纹(events.jsonl 落盘格式不变)
- [x] 1.2 实现 `Receipt`(读 events.jsonl,按 pane+since+needle 判定
      Confirmed/NoEvidence)——**Claude 与 Codex 首期同批注册**(司令拍板
      2026-07-23;Codex 事件密度低只降命中率,NoEvidence 落第 1 层)。实现为
      全部 hook-equipped agent 共用同一 events 回执(同一 gtmux hook、同一
      证据流,agent 无关)——只注册两家会让其余 6 家丢失既有事件优先校验,
      违反零回归;`HookEquipped` 字段按计划退役,消费方迁移到 `Receipt != nil`
- [x] 1.3 `dispatch.Deliver`:宣告 failed 前强制终局 Receipt 复查
      (`submitConfirmed`,经注入的 `io.Events` 读同一事件证据);Confirmed
      即 landed;屏读不得推翻 Confirmed(单调仲裁测试:事件迟到于最后一轮
      轮询之后、deadline 时刻才可见 → 终局复查捞回)
- [x] 1.4 `Result` 增加 `JudgedBy: driver|screen`,`send --json`(本期新增
      flag,仅限验证路径)/`spawn --json` 透出 `judged_by`
- [x] 1.5 swallowed-Enter 精化:dispatch 侧 Unsubmitted 的证据源即既有 draft
      判定(`draftHasDelivery` → 退避补 Enter,机制零改动);事件流看不见
      草稿,`eventsReceipt` 不产 Unsubmitted——该 verdict 的生产者是 P2 的
      wake draft 检查(`Verdict` 类型已就位)
- [x] 1.6 回归测试:复刻"UserPromptSubmit 已在流中而屏读判 NOT delivered"的
      实锤时间线(可剥前缀 payload 双轨失配 + 事件迟到)为 fake-IO 测试用例
      (`TestDeliver_StrippablePrefix_EventStillMatches`、
      `TestDeliver_TimeoutRecheck_LateEventNotLost`)
- [x] 1.7 kill-switch 验证:`driver.<agent>.receipt: off` 下 `hookEquipped()`
      为 false → Deliver 走纯第 1 层屏读路径(design §5;
      `TestSwitchForcesLayerOne`);docs(`docs/cli.md` spawn/send 一节的
      judged-by + 开关说明)

## P2 — wake ack 升级(hq-wake-reliability 的扩展;修回车被吞搁浅)

- [x] 2.1 `hqnudge.deliverPayload` ack 三层化:Receipt(`#id` 的
      UserPromptSubmit——hook 侧对 wake 批次落盘其 `#id` 为 Summary,
      `hqwake.BatchID` 提取)优先 → 屏读见 draft 持 `#id` = 精确 unsubmitted
      → 只补 Enter → 屏读 history 判定兜底;队列/claim/orphan/优先级/上限
      机制零改动
- [x] 2.2 "只补 Enter"实现:unsubmitted 批次的 claim 停靠为 `.stuck` +
      `enter-repair` 记录,下一次 drain 只发 Enter(draft 仍完整持有该批
      payload 头 + `#id` 尾才补,否则交还 requeue 路径);不重贴 payload、
      不重算 `#id`;补 Enter 有界(3 次)后按现状 unacked 交还
- [x] 2.3 回归测试:复刻今日实锤(paste 落地、Enter 被吞、draft-guard 堵死、
      10min stale 才升级)为 fake 时间线,断言升级后下一拍 drain 即补上
      (`TestDrain_SwallowedEnter_RepairedOnNextTick`;另有用户改动草稿
      永不代提交 / 用户代按 Enter 即确认 / 修复额度耗尽交还 三态)
- [x] 2.4 `wake-degraded` 判据不变的回归(fail≥3 / stale>10min 仍有效——既有
      Degraded 测试全数保留通过;停靠不计 fail,弃修才计一次);at-least-once
      与 `#id` 幂等语义不变(playbook 不需要改动)
- [x] 2.5 kill-switch 验证(receipt off → `driver.For` 剥除 → NoEvidence →
      纯屏读 ack;`TestDeliver_ReceiptNoEvidence_ScreenStillJudges` +
      driver 侧 `TestSwitchForcesLayerOne`)+ spec 场景对齐(hq-wake-protocol
      delta 已在 P0 落盘,实现与其逐场景吻合)

## P3 — spawn 就绪正向信号(SessionStart 短路)

- [x] 3.1 driver `Ready`(`eventsReady`):`WaitAgentReady` 以进门时刻为启动
      时刻(-1s 余量),SessionStart 事件(该 pane、时刻后)→ ready 短路
      (一帧 IsComposerReady 确认,免两帧稳定;gate/banner 检查对该帧仍生效)。
      与 Receipt 同理对全部 hook-equipped agent 注册——hook 把各家
      session_start/on_session_start/agentSpawn 等统一归一为 `SessionStart`
      流记录,证据 agent 无关;不发该事件的 agent 只是永不短路(I2)
- [x] 3.2 缺席不降级:`sessionUp` 为 nil / 恒 false 时屏幕门(两帧 + 超时
      拒贴)逐字节不变(`TestReadyGate_NoEvent_TwoFrameUnchanged`;旧
      `TestReadyGate` 全数保留)
- [x] 3.3 慢启动回归测试:每帧都在变(MCP 抖动)的 ready 帧序列,无事件永不
      settle、有事件首个 ready 帧即过(`TestReadyGate_SessionStartShortCircuits`);
      非 ready 帧不付事件扫描成本(`TestReadyGate_NoPollWhileNotComposerReady`)
- [x] 3.4 kill-switch 验证(`driver.<agent>.ready: off` 单独剥除、不碰
      receipt;`driver.enable: false` 全剥;`TestSwitchForcesLayerOne`)

## P4 — digest 感知档位标注(sense 字段)

- [x] 4.1 radar/digest 计算 `sense`(driver/partial/screen;判据取自既有事实:
      hook 落盘的 session 记录(sessionRef 解析成功 = hook 在)+ transcript
      实际可读;零新增采集;`senseOf` 纯函数 + 表测试)
- [x] 4.2 契约加法:`digest --json` / `GET /api/digest` 透出 omitempty `sense`
      (追加在结构体尾,既有字段与顺序不变;`TestDigestRow_SenseIsAdditive`)
- [x] 4.3 内容来源接线经 `driver.For()`:新增 `Content` 能力(claude/codex 的
      transcript.Load 纯收拢,`driver.<agent>.content` 开关独立剥除)。
      状态来源(waiting 标记)按 pane 键控、在 radar 内核先于 agent 识别被
      消费,强行经 agent 键控的注册表属人为绕行——保持直连,sense 判据如实
      反映之(校准记录于 design.md)
- [x] 4.4 文档:`docs/cli.md` digest 一节 + `api/contract.md`(示例含 `sense`)
      + CLAUDE.md 契约行 同 PR 更新(docs-conformance 检查通过)

## P5 — 一次性 headless worker(spawn --oneshot)

- [x] 5.1 `HeadlessSpec`:Claude(`claude -p --output-format stream-json
      --verbose`)与 Codex(`codex exec --json`,type 平铺/msg 嵌套两种协议
      形态都认)的启动命令构造 + 流式 JSON 解析器(容错:未知/不可解析行
      忽略,退出码兜底;解析器表测试)
- [x] 5.2 `gtmux spawn --oneshot`:仅 Headless 能力在位才接受,否则明确拒绝
      (en+zh,先于任何 tmux 依赖;kill-switch `driver.<agent>.headless` 同门);
      经隐藏 runner `gtmux oneshot-run`(check-design HIDDEN 白名单)在 tmux
      pane 内跑,JSON 流对用户可见;`--pane` 复用要求空 shell;
      `--worktree`/`--title`/`--headless` 正交
- [x] 5.3 状态真值接线:done/crash 来自 JSON 流与退出码,不经屏幕分类——
      runner 落 Stop/StopFailure 事件 + finished/active 标记(镜像 hook
      decide 语义,run 自带 hook 已记录时去重兜底不重复);session_id 落
      resume 记录 → digest 行经既有 sense 判据读出 `driver`;radar 行经
      进程子树识别照常;ledger/tasks/reap 语义照旧(投递即 argv,
      `judged_by: driver`)
- [x] 5.4 环境纪律:runner exec 前清除 `CLAUDECODE`/`CLAUDE_CODE_*`/`CMUX_*`
      (multiplexer-research ⭐B),代理等其余环境透传;测试断言
- [x] 5.5 CLI 面文档:CLAUDE.md 命令列表不变(仍是 spawn;`oneshot-run` 进
      HIDDEN)、spawnUsage(en+zh)、`docs/cli.md` spawn 一节 `--oneshot` 与
      `--headless` 的区别说明(watch-only 契约写明);check-design.sh 通过

## 收尾(每期各自包含;此处为全 change 完成态)

- [ ] 6.1 `npx @fission-ai/openspec validate --specs --strict` 通过
- [ ] 6.2 实现全部合并后:sync-specs + archive 本 change(同 PR 或紧随 PR),
      tasks 勾选保持真实
