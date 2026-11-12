# 使用与审阅知识

[English](knowledge-review.md)

知识台账继续是唯一权威。先在计划或风险动作前读取少量适用知识，再由人或 HQ 审核修订。此版本提供 CLI 和 Markdown 工作台；Dreaming 复用现有模型 CLI，没有增加后台服务、调度器或自动同步插件。

## 动作前读取

```sh
gtmux knowledge consult --query '发布新版本，沿用访问限制' --action release --repo /path/to/project --budget 1200
```

结果为 JSON：完整卡片、版本、选择理由、待核验项及估算 token。预算范围 1–2000，最多 6 张；不会截掉例外来凑预算。`missing_required` 非空表示相关保护规则因过期、待复核或预算不足未能提供，先处理缺口。查询本身不写 hit 或使用记录。

`--tool name=version` 可重复；有版本约束的卡片必须精确匹配。`--role hq` 选择 HQ 内容，不授予写权限。旧条目没有范围时默认不推送；HQ 调查可加 `--include-legacy`，结果明确提示范围未知。原 `search` 仍可供人工广泛查找。`--purpose audit|evaluation` 标明审计或评测目的；HQ 的新版种子指令已接入编写 goal 前的 consult；现有 spawn 自动提示未被替换，worker 调用者仍须主动读取。源码修改不会更新活动 HQ 指令，须经正常版本升级与 seed 生效。

## 在 Obsidian 中查看与编辑

```sh
gtmux knowledge review export /path/to/project/kb-vault
gtmux knowledge review diff /path/to/project/kb-vault > /path/to/project/proposal.json
gtmux knowledge review check /path/to/project/proposal.json
```

首次导出选择空目录，然后在 Obsidian 中“打开文件夹为仓库”。一条一文件；正文与标题可编辑，三个生成属性不可编辑。保持 ID 可重命名文件；删除文件只报告缺失，不退休知识。个人笔记放 `个人批注/`；无 ID 新笔记列为草稿，走既有 capture/add 准入。

`diff` 是待审提案，不是已写回。每批最多应用 25 项；较大批次需人工拆分，保留各项 `base_revision` 并清空提案 `id` 以重算摘要。HQ 在既有授权入口执行 `gtmux knowledge review apply <proposal.json>`。不得通过切目录冒充 HQ。修改理由/证据后同样清空 `id` 再 check。

冲突保留两边内容，不覆盖已检测到的人工修改。笔记与版本基线按完整文件原子发布，只读文件拒绝覆盖；刷新导出前先结束编辑，外部编辑器不参与台账锁。旧 wiki 名称或占位链接可能仍需在 Obsidian 中手工核对，导出不会凭空补全知识。应用保持旧 ID、hit、来源和假设状态；正文改变会使旧动作卡转为待核验，并清除不再对应的翻译。已晋升规则重新待落地，需通过原 land/sync 流程核对载体；不会自动写所有指令文件。LOCAL 的精确历史生成段可以更新，人工改写或重复标记会拒绝覆盖。

敏感标记和 accounts 默认不导出。后来改成敏感的条目若已导出，刷新会明确报告旧副本需要人工清除；软件不能替你删除 Obsidian 历史、备份或云副本。不要启用未获授权的同步。工作台基线和属性是冲突检测资料，不是签名或身份凭据。

## Dreaming：存档、模型选择与续跑

HQ 在既有 distill 唤醒时判断是否需要审阅。先看 `dream list`，优先续跑未完成的任务；有待审提案积压时延后新包。没有另一个 daemon，也不在后台自行升级模型。启动包固定知识 ID、完整版本、正文、原动作卡、范围和证据；默认 20 条，执行包最多 25 条。

由现有 HQ 写者执行以下流程，文件路径均属于项目。不要通过切换目录冒充 HQ。

1. 准备 `run.json`：

```json
{"selection":{"mode":"pilot","limit":20},"max_calls":40,"max_seconds":7200}
```

2. `gtmux knowledge dream start /path/to/project/run.json` 返回 `id`。保存这个运行 ID，续跑用它，不重新创建任务。`dream plan <request.json>` 仍可只读预览；请求是 `selection` 内那层对象。

3. HQ 选择模型，写 `choice.json`。下面的模型名称是占位符，须替换为当前 CLI 可用的准确名称：

```json
{"agent":"codex","model":"<HQ选择的模型>","effort":"high","reason":"这一包包含边界冲突，需要较强推理","timeout_seconds":600}
```

`agent` 支持 `codex`、`claude`。HQ 按包的难度、失败原因和费用选模型；同一运行后续可换模型，每次选择和理由都留在历史中。简单修辞无需最高档，缺来源则暂缓，不能靠换强模型补造证据。

```sh
gtmux knowledge dream run <run-id> /path/to/project/choice.json
gtmux knowledge dream status <run-id>
gtmux knowledge dream history <run-id> > /path/to/project/dream-history.json
gtmux knowledge dream proposal <run-id> > /path/to/project/proposal.json
gtmux knowledge review check /path/to/project/proposal.json
```

有修改才 check/apply；零项表示无修改提案。`review_ready` 表示本包逐项判定结束，可能包含暂缓项，**不表示知识已修好或提案已应用**。HQ 审核后沿用 `review apply`；Dreaming 本身不改 KB、不加 hit、不清候选队列。

### 中断时如何继续

每条知识是一个检查点。开始前存档输入版本、模型/推理档/理由和超时；结束后原子保存结论、待审修改、可见输出摘要和用量回执。已完成条目再次 run 会跳过。当前条目的模型内部推理不能从一半恢复，至多重做这一条。

| 情况 | 行为 |
|---|---|
| CLI 正常失败、取消或返回无效 JSON | 留下失败记录并暂停，HQ 判断后续跑 |
| 控制进程崩溃，只有开始记录 | 下次 run 先记“中断、用量未知”并停止；HQ 核实旧进程结束后再 run |
| 输入被修订、退休或改为敏感 | 不调用模型，记暂缓，后续新包读取合法新版本 |
| 同时两次 run | 只有一个取得全局执行锁；另一个退出 |
| 某条连续失败 | 最多 3 次调用；保留历史，允许带理由暂缓 |

暂缓请求 `reason.json`：`{"item_id":"topic/id","reason":"需要补充当前来源，另行核对"}`，执行 `gtmux knowledge dream defer <run-id> <reason.json>`。这不清零已用次数/时间，也不把该版本标成已审。暂缓后可继续其余条目。旧任务处于未确认调用时，其他任务也不能绕过它启动模型。

用户或既有授权范围明确追加额度后，HQ 可执行 `gtmux knowledge dream extend <run-id> <allowance.json>`；文件例如 `{"max_calls":6,"max_seconds":1800,"reason":"用户追加至完成本次验收"}`。数值是新的总上限，只能增加，仍不超过每包 60 次/4 小时。扩额理由与上限追加进原日志；重复相同上限不重复记账。原计划、已用次数/时间、未知用量、单条三次尝试与共享周额度都保留，扩额不自动运行模型。未确认的中断须先恢复；没有待处理项时不再扩额。


### 哪些额度能硬停

| 约束 | CLI 执行规则 |
|---|---|
| 单包调用数 | 默认 40，可设 1–60；包括失败/中断 |
| 单包执行时间 | 默认 2 小时，可设 1 秒–4 小时 |
| 单次调用 | 必填 1–1800 秒，自动缩到包/周剩余额度 |
| 单条重试 | 最多 3 次；失败不自动重试 |
| 所有任务共用周限额 | UTC 周一重置：200 次、12 小时；换包或模型不清零 |
| 输入/输出 token | 保存 CLI 终结回执，达到阈值后阻止下一次调用 |

输入/生成量参考阈值：pilot 累计 50万/20万；bootstrap 累计 600万/180万；routine 每周 100万/40万。pilot/bootstrap 在本机完整历史中跨包累计，跨周不重置；routine 计本周所有模式的用量。只读 plan 的 `allow_burst` 仍用于估算，CLI 执行明确不接受该覆盖。

**CLI 不能保证在途 token 或账单硬上限。** 有终结 usage 就记录 provider 报告值；无回执保持 unknown，不能算 0。生成量以 CLI 报告为准，不凭可见文本估算内部推理。`plan` 的 `usage_known:false/can_start:false` 仍表示 token 预算未经核实；CLI 路线按上表的调用/时间约束执行，不宣称精确 token 限额。用户选择复用账号，因此这里不增加 API 凭据或计费系统。

控制器超时会终止自己的进程组；不能保证远端已立刻停止计费。进程被强杀后可能仍有未确认计算，因此中断须由 HQ 检查，不能承诺远端 exactly-once。模型 CLI 不支持隔离参数或协议时直接失败，不悄悄放宽工具权限。

失败历史保留固定分类 `failure_code` 和数字 `exit_code`，用来区分配置、认证、网络、限额、取消、协议和进程失败。分类是诊断线索，不是根因证明；未知仍记为未知。stderr 最多在内存暂存末尾 8 KiB，只用于分类，原文不落盘。失败调用和未知用量继续计入原运行，修复后续跑不能重置额度。

### 历史保存与恢复边界

托管日志位于 gtmux 状态目录的 `knowledge-dream/actions.jsonl`：0600、逐条序号和哈希链、临时文件 fsync 后原子替换，再同步目录。日志是唯一运行状态；`list/status/history/proposal` 只读。只保存有界最终答复和结构化结论，不归档模型隐藏推理、工具流或 stderr。哈希链检测意外损坏，不是防篡改签名。

文件到 16 MiB 或发现不完整/损坏时停下并保全，不自动修剪。当前版本没有日志压缩/分卷导入命令；不能删除或截断日志来“腾空间”，那会丢进度和额度。恢复须停执行器、备份原件、核对完整链；回滚二进制时保留整个日志。若旧二进制没有这些命令，暂停 Dreaming，升级回来续跑。

CLI 在一次性空目录工作，关闭工具/插件/会话持久化，使用现有账号，不读取原生会话。包排除 sensitive/accounts，调用前后复核版本；模型引用只允许来自包或该知识版本本身。包内证据是待审材料，不是授权指令；模型不能自行确认事实或扩大范围。

## 记录使用与回滚

`gtmux knowledge usage record <receipt.json>` 单列显式自报，`usage summary` 汇总最近 30 天；每条包含 version=1、task_key 与 plan_id 的 SHA-256、purpose、stage、observation=`self_report`、卡片 ID/版本/内容哈希。stage 为 retrieved/shown/adopted/outcome；outcome 另带 succeeded/failed/unknown。重复回执幂等，evaluation 单列，audit 不计入。8 MiB 上限到达后报错，需明确归档；不自动删除。

缺失记录不等于未使用，自报成功不证明知识导致改善，因果使用率保持未知。此版本尚无可信接收端展示回执和 A/B 结果。

台账历史不删除。普通误修可用当前基准版本提交恢复旧正文的新提案；不要覆盖整个台账去抹掉并发修改。退休恢复走已有 add/历史核对路径。升级前备份台账及人工工作台；旧程序可读 schema 3 基础字段，但不能理解新卡片/使用语义，降级期间暂停这些新写入。
## 落地对应的审阅版本

自动执行 `gtmux knowledge land <id>` 时，版本校验、写入载体、记录完成共用现有台账锁。
同 ID review 会重新打开待落地状态；延迟的旧 carry 无法结清或覆盖新修订。
公开的机器/仓库 sync（包括 doctor 修复）从读台账到发布也持有同一把锁，不能在新 carry
完成后写回旧快照。外部编辑器不参与这把锁。

若知识由你手工写入，先读 `gtmux knowledge show <id> --json`，确认这份正文确实到达目标，
再执行 `gtmux knowledge land <id> --ref '<目标位置>' --revision '<此次读取的 revision>'`。
缺版本或版本过期会被拒绝。重新读取后应核对实际目标再确认，不能仅替换成新 token 重试。
Mac 和手机确认框保留打开时读到的版本。旧客户端缺少版本时，需要刷新或升级才能手工记回执；
自动落地仍可使用。

LOCAL 更新保留无关人工规则，以完整私有文件（0600）原子替换。只读文件、链接目标和手改过的
生成段会被拒绝。载体写入失败时，晋升仍待落地；若完整正文已发布而后续步骤失败，重试不会
重复插入。即使正文相同，重试也会重新原子发布并确认文件/目录同步；同步再次失败仍保持
待落地。先读回执，再决定重试或修复渲染视图。旧落地历史保持可读，无需迁移。
