package hq

const hqDSHMessagesEN = `
DSH terminal message review: verified DeepSeek Harness panes may use a community
client such as dst; this does not change their harness identity. ConversationMessage
records are discovery evidence, not a declaration that you reviewed their content.
Read public text with gtmux transcript <session_id> --messages --after-seq <cursor>
--limit 100 --json. Start at -1, or the saved reviewed_through value, and continue
while has_more. Only after judging the returned messages, repeat the same page read
with --review-through <last reviewed message seq> to declare review. Keep your
judgment in the board; event consumption, a notification and an input acceptance
receipt never mean review or task completion. ConversationGap invalidates coverage:
reconcile public history and disclose missing material. Current foreground terminal
coverage does not automatically include older Web or background-client intervals.
DSH terminal spawn uses the model selected in dst; generic --model and --oneshot
are refused. If a task requires another model, select it in the client first.
Native questions remain with the user. Review is observational, never an automatic
permission approval or a slow pre-execution gate.
`
const hqDSHMessagesZH = `
DSH 终端逐条审阅：受管 DeepSeek Harness pane 可以使用 dst 等社区客户端，身份仍是
DeepSeek Harness。ConversationMessage 是发现消息的线索，不代表你已审阅正文。
用 gtmux transcript <session_id> --messages --after-seq <cursor> --limit 100 --json
读取公开文本。从 -1 或已保存的 reviewed_through 开始，has_more 为真时继续翻页。
判断完返回的消息后，用相同分页参数加 --review-through <最后审阅的消息序号> 明确记录
审阅进度，具体判断写进归档板。消费事件、收到通知、原生输入被接收，都不等于已经审阅或
任务完成。ConversationGap 表示覆盖有缺口，需核对公开历史并说明缺失材料。前台终端
覆盖不会自动追授较早的 Web 对话或后台客户端区间。DSH 终端派发使用 dst 中已选模型，
拒绝通用 --model 和 --oneshot；任务需要其他模型时，先在客户端选好。用户问题仍由用户回答；审阅是观察和
跟进，不会自动批准权限，也不会把模型执行卡在一个缓慢的事前审批上。
`
