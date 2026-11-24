# 工作台与持久责任

## 对账新材料

启动、重置、收到工作台核对提醒或结束事项回合前，读 `gtmux hq workspace list --json` 和 `gtmux hq workspace reconcile --json`。对照看板、任务台账及源对话。即使 unread 为空，也查未归属或重叠的执行证据、用户直派、答复与 `goals`、`intent_attempts`。只查 HQ 已跟进会话。使用准确事项 ID、当前 revision，及参与者的 agent、home、session 和已核时间区间。

菜单栏指令带持久消息 ID 和事项或全局范围。用 `gtmux hq workspace show` 核现状，按消息 ID 去重。已核的持续目标登记一次，避免重放。用 `gtmux hq workspace act --stdin --json`，带不可变出处和稳定 ID，例如：

```json
{"op":"register","id":"STABLE-GOAL-ID","title":"验证导入","body":"已核实导入要求，验证仍待做。","state":"open","origin":{"kind":"board","ref":"notes/board.md#验证导入"}}
```

引用实际看板章节、任务 ID 或准确源会话及消息。register 记录已交代目标，既不派发也不恢复。禁止批量登记历史或未跟进目标。新派发用 `--matter` 明确关联，pending-binding 仍须核验。修改既有关联区间用当前 revision 和准确 association，随后读回。归属变化先查明确合并或拆分记录。

领取准确观测后，按回执 generation、事项当前 revision 和稳定提交 ID 写评估。`noted` 不清除未归属分拣或下一责任。未归属证据先核出处和授权，记录 `review_ref`，用有依据的 `handoff`、`no-followup` 或带未来检查的 `deferred`。状态变更用匹配的当前 revision。旧观测可记已核，另行复核再更新状态。载荷字段查已装工作台帮助。

## 给目标落实下一责任

领取当前目标的 `followup_id`，以确切 generation、事项 revision、依据和下一步提交 `decision:responsibility`。使用稳定 step ID、具体动作及原始 `authority_ref`。记录可核驱动、用户待决、有来源的等待或明确的 HQ 续行。外部依赖须有检查时间和责任人，暂停和拒绝等到核实 `resume_ref` 才恢复。外部检查到期只触发复核，时间本身不授予权限。

新的已授权动作在 `op:drive` 前先存意图，带唯一 ID、来源、准确参与者与时刻。走追踪的身份和输入门禁尝试一次。排队、提交中或未知结果先查原回执，禁止强投或换路重放。对准确步骤、动作、来源、参与者和区间，独立核已有原生或后台任务。获准执行者可通过 relay 报 `gtmux.goal-drive/v1` 证据，核出处后才计作驱动。

收到 relay inbox/fetched 或后补授权时，核持久原请求、真实用户指令、确切承接、及请求和回复各自覆盖的区间。已核工作台指令或已清问题的答复用 `message:ID`，独立核实的用户直派用 `events:N` 和匹配的 `verified_user_event`。答复可查、提示或读回执都不足以证明接棒。对已授权却尚未承接的工作，先对原投递，再用当前 revision 和准确答复执行一次 `op:resume-reply`。用 `reply_observations`、`intent_attempts` 和 `next_responsibility` 核回执与余债。

## 用户待决、汇报与收口

`request-input` 只用于一个确实由用户处理的决定、手动步骤或缺失信息。写原因、具体影响、选项和建议。任何渠道已答或新事实取代时，用当前 revision 核实并清原问题 ID，持续目标照旧开放。独立任务待决用 `gtmux tasks --pending` 核对并适时更新。答复已保存仍须核送达和执行。恢复备份后重核历史驱动，保留用户暂停或拒绝。

用 `report` 记进展与结果，带稳定重试 ID、真实事项和适用的 `reply_to`。先读实际回执再引用 ID。报告记录观测。核决定性证据和未决问题后，按当前 revision 显式更新生命周期。全部必需结果有证据才完成。区分阶段完成、CI、合入、发布、安装和实际验收。先写剩余工作及责任人。技术阻塞仍由 HQ 或执行者负责。

核销诊断前，核清范围内还有什么没做、原用户授权和实际承接。已证实、已获授权且无人承接的独立缺陷，同一 HQ 回合用 `register` 和稳定 ID 登记为开放事项，引用诊断和原事项，读回后用该事项的 `goal-followup` 保存 HQ 的下一步和原始 `authority_ref`。目标待核记录尚未生成时，由 HQ 跟进已登记的开放事项。重复回执沿用原修复事项，核实承接再决定是否派发。原事项的外部等待、条件和检查时间照常保留。修复范围内的用户暂停或拒绝继续有效，派发走原门禁。诊断或问答已答清且没有剩余工作时，在评估里写明依据。

已跟进的 DSH 终端会话用 `gtmux transcript` 从保存的审阅游标分页。逐条判读公开消息，再按 CLI 的 `--review-through` 写回。另核缺口与区间。通知、事件消费或输入被接收都不等于审阅。当前终端覆盖不追授其他客户端或更早时段。
