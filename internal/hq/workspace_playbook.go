package hq

const hqWorkspaceInstructionsEN = `
## HQ workspace
Menu-bar instructions carry [gtmux workspace message ID; matter MATTER (title)] or global HQ conversation. The ID is a durable instruction identity: never execute the same ID twice. Read current membership/state with gtmux hq workspace list --json and show MATTER --json. A matter is a goal, not a single pane. Associate workers explicitly through gtmux spawn --matter MATTER --cwd PROJECT --goal-file FILE --json. Never infer membership from title, cwd or pane number. A pending-binding spawn receipt is not a confirmed association.
Publish concise public progress, decisions and results through gtmux hq workspace act --stdin --json. Send ONE JSON object, e.g. {"op":"report","id":"UNIQUE-REPORT-ID","matter":"MATTER","reply_to":"MESSAGE-ID","kind":"progress","body":"User-facing progress"}. Reuse the same ID when retrying a write. Omit matter for global conversation, reply_to for unsolicited progress. Report is accepted only from the current HQ pane. Set state:"needs-input" for a real user decision; state:"completed" only after verifying the goal. Result, idle, Stop and delivery acknowledgement do not imply completion. Check explicit merge/split records before replying after membership changes. Keep raw operational/tool details in original records, not public reports. Follow the user's authority limits: this workspace is an interaction channel, not extra authorization.
`

const hqWorkspaceInstructionsZH = `
## HQ 工作台
菜单栏指令带有 [gtmux workspace message ID; matter MATTER (title)] 或 global HQ conversation。ID 是持久指令标识，同一 ID 不重复执行。用 gtmux hq workspace list --json 和 show MATTER --json 核对当前归属与状态。事项是目标，不是单个 pane。用 gtmux spawn --matter MATTER --cwd PROJECT --goal-file FILE --json 明确关联执行会话，不能靠标题、目录或窗格编号推断。pending-binding 回执不代表关联已确认。
通过 gtmux hq workspace act --stdin --json 写简明的进展、决定和结果，stdin 传一个 JSON 对象，例如 {"op":"report","id":"UNIQUE-REPORT-ID","matter":"MATTER","reply_to":"MESSAGE-ID","kind":"progress","body":"面向用户的进展"}。写入重试沿用原 ID。全局对话省略 matter，主动汇报省略 reply_to。report 只接受当前 HQ pane 的调用。确需用户决定才设 state:"needs-input"，验证目标达成后才设 state:"completed"；result、空闲、Stop 和送达确认都不等于完成。归属变化后先读明确的合并、拆分记录再回复。工具细节留在原始记录，不将原始唤醒信号堆入公开汇报。始终遵守用户既有权限范围，工作台是交互入口，不是新增授权。
`
