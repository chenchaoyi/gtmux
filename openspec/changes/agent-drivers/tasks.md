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

- [ ] 1.1 归一化单轨:`driver.NormalizeNeedle` 统一 hook 落盘 Summary 与
      Deliver 比对两侧;fixture 断言两侧对同一 payload 产出相等指纹
      (events.jsonl 落盘格式不变)
- [ ] 1.2 实现 `Receipt`(读 events.jsonl,按 pane+since+needle 三值判定
      Confirmed/Unsubmitted/NoEvidence)——**Claude 与 Codex 首期同批注册**
      (司令拍板 2026-07-23;Codex 事件密度低只降命中率,NoEvidence 落第 1 层)
- [ ] 1.3 `dispatch.Deliver`:宣告 failed 前强制终局 Receipt 复查;Confirmed
      即 landed;屏读不得推翻 Confirmed(单调仲裁测试:事件迟到于最后一轮
      轮询、先屏读误判后事件到达 等场景)
- [ ] 1.4 `Result` 增加 `JudgedBy: driver|screen`,`send --json`/`spawn --json`
      透出;evidence 附带判定层
- [ ] 1.5 swallowed-Enter 精化:Receipt 返回 Unsubmitted 时走既有退避补 Enter
      路径(沿用"草稿仍完整才补"纪律;不新增重试机制)
- [ ] 1.6 回归测试:复刻"UserPromptSubmit 已在流中而屏读判 NOT delivered"的
      实锤时间线为 fake-IO 测试用例
- [ ] 1.7 kill-switch 验证:`driver.<agent>.receipt: off` 下 Deliver 走纯
      第 1 层屏读路径(design §5);docs(`docs/cli.md` send/spawn 一节的
      judged-by 说明)

## P2 — wake ack 升级(hq-wake-reliability 的扩展;修回车被吞搁浅)

- [ ] 2.1 `hqnudge.deliverPayload` ack 三层化:Receipt(`#id` 的
      UserPromptSubmit)优先 → Unsubmitted/屏读见 draft 持 `#id` → 只补 Enter
      → 屏读 history 判定兜底;队列/claim/orphan/优先级/上限机制零改动
- [ ] 2.2 "只补 Enter"实现:同一 claim 下一次 drain 只发 Enter(draft 仍完整
      持有该批文本才补,否则按现状 requeue);不重贴 payload、不重算 `#id`
- [ ] 2.3 回归测试:复刻今日实锤(paste 落地、Enter 被吞、draft-guard 堵死、
      10min stale 才升级)为 fake 时间线,断言升级后 3s fast tick 下一拍补上
- [ ] 2.4 `wake-degraded` 判据不变的回归(fail≥3 / stale>10min 仍有效);
      at-least-once 与 `#id` 幂等语义不变(playbook 不需要改动)
- [ ] 2.5 kill-switch 验证 + spec 场景对齐(hq-wake-protocol delta)

## P3 — spawn 就绪正向信号(SessionStart 短路)

- [ ] 3.1 Claude driver `Ready`:spawn 记录启动时刻,SessionStart 事件
      (该 pane、时刻后)→ ready 短路(一帧 IsComposerReady 确认,免两帧稳定)
- [ ] 3.2 缺席不降级:无事件时屏幕门(两帧 + 超时拒贴)逐字节不变;I2 测试
      (SessionStart 缺席绝不导致 failed)
- [ ] 3.3 慢启动回归测试:MCP 抖动导致屏幕久不稳定的时间线,断言短路后不再
      拖满 20s
- [ ] 3.4 kill-switch 验证(`driver.claude.ready: off`)

## P4 — digest 感知档位标注(sense 字段)

- [ ] 4.1 radar/digest 计算 `sense`(driver/partial/screen;判据取自既有事实:
      waiting 标记来源 + transcript 可达性;零新增采集)
- [ ] 4.2 契约加法:`digest --json` / `GET /api/digest` 透出 omitempty `sense`;
      既有字段与顺序不变(契约回归测试)
- [ ] 4.3 状态/内容来源接线经 `driver.For()`(实现不动,纯收拢)
- [ ] 4.4 文档:`docs/cli.md` digest 一节 + `api/contract.md` + CLAUDE.md 契约行
      同 PR 更新(docs-conformance 检查通过)

## P5 — 一次性 headless worker(spawn --oneshot)

- [ ] 5.1 `HeadlessSpec`:Claude(`claude -p --output-format stream-json`)与
      Codex(`codex exec --json`)的启动命令构造 + 流式 JSON 解析器
      (容错:未知事件忽略,退出码兜底)
- [ ] 5.2 `gtmux spawn --oneshot`:仅 Headless 能力在位才接受,否则明确拒绝
      (en+zh 提示);命令仍跑在 tmux pane 内;`--worktree`/`--title` 正交
- [ ] 5.3 状态真值接线:done/crash 来自 JSON 流与退出码,不经屏幕分类;
      digest 行 `sense: driver`;ledger/tasks/reap 语义照旧
- [ ] 5.4 环境纪律:launch 前清除会递归触发 hook 的环境变量(CLAUDE_CODE_* 等,
      沿用 multiplexer-research ⭐B 结论);测试断言
- [ ] 5.5 CLI 面文档:CLAUDE.md 命令列表不变(仍是 spawn)、`gtmux --help`
      (en+zh)、`docs/cli.md` spawn 一节 `--oneshot` 与 `--headless` 的区别
      说明;check-design.sh 通过

## 收尾(每期各自包含;此处为全 change 完成态)

- [ ] 6.1 `npx @fission-ai/openspec validate --specs --strict` 通过
- [ ] 6.2 实现全部合并后:sync-specs + archive 本 change(同 PR 或紧随 PR),
      tasks 勾选保持真实
