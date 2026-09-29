# Codex 接入

这里集中说明 Codex 的特殊处理。通用约定见[接入 agent](agent-onboarding.zh.md)，
命令见 [CLI](../cli.zh.md)。Codex 版本改变 hook、屏幕或会话日志格式时，
这份文档与英文版要一起更新。

| 事项 | 规则 | 实现 |
|---|---|---|
| 身份与历史 | 注册表声明 Codex 的进程、恢复命令、hook、解析器和图标。每轮对话保留来源 agent；HQ 从 Claude 切换到 Codex 后，旧对话不会改成 Codex。 | `internal/agents/registry.go`、`internal/transcript/transcript.go`、`mobileapp/src/ui/ChatView.tsx` |
| 启动 | 目录信任、hook 审核和 MCP 启动行都不是就绪输入框。原地换 agent 后残留的 Claude 屏幕不能让 Codex 被判就绪。 | `internal/prompt/prompt.go`、`internal/prompt/prompt_codex_test.go` |
| Hook | `gtmux install hooks --agent codex` 追加 `~/.codex/hooks.json` 条目并启用 `features.hooks`，保留已有旧式 `notify`。新 hook 可能需信任一次；更改后应重启已运行的 Codex。 | `internal/app/codex_hooks.go`、`internal/hook/classify.go` |
| 归属 | 共享 app-server 可能发出没有 cwd、会话 ID 的 hook，却继承其他客户端的 `TMUX_PANE`。完成事件须有唯一绑定且写下 `task_complete` 的日志；审批事件须有唯一会话绑定。归属不了就不指派 pane，让雷达检查实际 pane。 | `internal/hook/codexpane.go`、`internal/radar/codexcompletion.go` |
| 空闲时的警告 | 用量或其他警告会刷新 Codex 已就绪的输入框，但不代表新回合开始。没有回合标记或可见工作时，雷达保持空闲；即使 Codex 还没跑过第一轮、同一 tmux 位置仍留有旧 agent 的恢复记录，也不会误报运行中。旧记录不用于 Codex 的对话、报错或完成时间。唯一工作目录可识别时，新版日志的 `session_meta.id` 可用于绑定会话。 | `internal/radar/agents.go`、`internal/radar/codexcompletion.go`、`internal/transcript/codex.go` |
| 任务投递 | `[Pasted Content N chars]` 中的 N 与任务长度相符时，只能证明粘贴到了输入框。提交回执还须匹配已绑定的会话 ID。若 Enter 被吞，只在已记录的折叠草稿仍在时补发 Enter，不再次粘贴。 | `internal/dispatch/deliver.go`、`internal/dispatchbridge/dispatchbridge.go` |
| 桌面通知 | `PermissionRequest` 早于自动审核；只有已归属 pane 的编号菜单持续显示，才算需人处理。无法归属的完成事件不发通用横幅。HQ 例行完成静默，真实输入仍可通知。抑制原因写入结构化诊断。 | `internal/hook/hook.go`、`openspec/specs/notifications/spec.md` |
| Ghostty | Codex TUI 可绕过菜单栏通知队列，经终端转义序列另发通知。**新启动的 Codex HQ 进程**默认带 `-c 'tui.notifications=["approval-requested","plan-mode-prompt"]'`，除非命令已显式指定；普通 Codex 不变。`gtmux hq --rotate` 会等当前回合结束后，在**同一进程**内发送 `/new`，不能更新启动参数。退出进程后运行 `gtmux hq` 才会采用新参数。 | `internal/hq/hqagent.go`、`internal/hq/rotate_pending.go` |
| 对话 | 新版日志把用户的 `input_text` 和 agent 的 `output_text` 都写在 `response_item` 中；公开回复的阶段是 `commentary` / `final_answer`。对话在回合进行中就按顺序展示过程消息和工具步骤，不展示 analysis。旧版 `event_msg.user_message` / `agent_message` / `task_complete` 仍可读，重复的结尾只展示一次。注入的指引与环境快照不当用户消息。同一桌面版会话可能续写到文件名含 `<session-id>_<instance-id>.jsonl` 的新记录；gtmux 核对其中的 `session_meta.id`，按记录顺序合并各轮对话，任一记录增长时更新聊天缓存标识。未知记录不挡住后续轮次。 | `internal/transcript/codex.go`、`internal/transcript/transcript.go`、`internal/transcript/testdata/codex-current.jsonl` |
| tmux 外的会话 | `source: native` 只表示没有 tmux pane，不等于在终端窗口。匹配会话日志中的 `session_meta.originator` 可区分 ChatGPT 桌面版（`codex_work_desktop`）和终端 Codex（`codex-tui`），通过新增的 `client` 字段传给界面。两者的日志 `source` 都可能是 `vscode`，不能拿它判断客户端；未知来源保持不标注。若 native 记录还处于工作中且没有可用的 Stop hook，同一会话所有匹配记录中的最新 `task_complete` 或 `turn_aborted` 可将其改判为空闲；更新的 `task_started` 则仍算工作中。菜单栏、手机列表和手机长按面板均显示来源。ChatGPT 桌面版会话不能转入 tmux：原客户端进程无法由 native 记录关闭，恢复后会出现两个客户端。其他 tmux 外会话保留原有操作。 | `internal/transcript/codex.go`、`internal/radar/agents.go`、`internal/app/adopt.go`、`macapp/Sources/GtmuxBar/MenuView.swift`、`mobileapp/src/ui/rowSheetModel.ts` |
| 手机终端 | 默认按手机宽度折行；“原宽／折行”切换到源 tmux 列宽并允许左右滑动，保留 Codex 宽幅 TUI。旧服务端按捕获行的字符格估算列数。这不改变对话历史。 | `mobileapp/src/ui/NativeTerm.tsx`、`mobileapp/src/ui/term.ts` |

排查用 `gtmux agents --json` 看 pane 与角色，用 `gtmux events --json` 和
`gtmux logs --component hook --json` 看事件与抑制原因。Ghostty 通知还需核对
当前 HQ 的 **Codex 进程参数**：升级 gtmux 不会改变旧进程的参数。较晚出现的真实审批
菜单可由雷达下次轮询发现。手机 VoiceOver 与小屏布局仍需真机验收。
