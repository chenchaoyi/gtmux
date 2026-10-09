# Codex 接入

这里集中说明 Codex 的特殊处理。通用约定见[接入 agent](agent-onboarding.zh.md)，
命令见 [CLI](../cli.zh.md)。Codex 版本改变 hook、屏幕或会话日志格式时，
这份文档与英文版要一起更新。

| 事项 | 规则 | 实现 |
|---|---|---|
| 身份与历史 | 注册表声明 Codex 的进程、恢复命令、hook、解析器和图标。每轮对话保留来源 agent；HQ 从 Claude 切换到 Codex 后，旧对话不会改成 Codex。 | `internal/agents/registry.go`、`internal/transcript/transcript.go`、`mobileapp/src/ui/ChatView.tsx` |
| 启动 | 目录信任、hook 审核和 MCP 启动行都不是就绪输入框。原地换 agent 后残留的 Claude 屏幕不能让 Codex 被判就绪。 | `internal/prompt/prompt.go`、`internal/prompt/prompt_codex_test.go` |
| Hook | `gtmux install hooks --agent codex` 在 `$CODEX_HOME`（默认 `~/.codex`）的 `hooks.json` 追加条目，在 `config.toml` 启用 `features.hooks`，保留已有旧式 `notify`。新 hook 可能需信任一次；更改后应重启已运行的 Codex。 | `internal/app/codex_hooks.go`、`internal/hook/classify.go` |
| 归属 | 共享 app-server 可能发出没有 cwd、会话 ID 的 hook，却继承其他客户端的 `TMUX_PANE`。完成事件须有唯一绑定且写下 `task_complete` 的日志；审批事件须有唯一会话绑定。无法归属的 Stop 保留不带 pane 的生命周期记录；无法归属的审批只写抑制诊断，不写 Waiting 生命周期记录。雷达随后可检查实际 pane。 | `internal/hook/codexpane.go`、`internal/hook/hook.go`、`internal/radar/codexcompletion.go` |
| 派生会话首次绑定 | 尚未绑定的交互式 `gtmux spawn --agent codex` 会附带一次性随机标识。私有记录只保存完整投递内容的 SHA-256、目标 pane ID／位置／shell PID／Codex 客户端 PID／目录及原绑定，不保存任务原文。提交 hook 核对完整内容、实时目标和日志的终端来源后，才绑定新会话。hook 缺 ID 或日志尚未写好时，雷达可从 native 会话日志中的真实用户提交补查。标识十分钟后失效；补查限于每份匹配日志末尾 8 MiB。目标或归属已变、桌面版会话、agent／工具回显都不能据此认领 pane。写绑定失败保留记录供重试，成功后消费；对话和知识采矿不展示标识。已绑定会话、其他 agent 和一次性任务维持原有投递方式。 | `internal/app/spawn_binding.go`、`internal/resume/codexbinding.go`、`internal/hook/codexbinding.go`、`internal/radar/agents.go`、`internal/transcript/sessionbinding.go` |
| 空闲时的警告 | 用量或其他警告会刷新 Codex 已就绪的输入框，但不代表新回合开始。没有回合标记或可见工作时，雷达保持空闲；即使 Codex 还没跑过第一轮、同一 tmux 位置仍留有旧 agent 的恢复记录，也不会误报运行中。旧记录不用于 Codex 的对话、报错或完成时间。唯一工作目录可识别时，新版日志的 `session_meta.id` 可用于绑定会话。 | `internal/radar/agents.go`、`internal/radar/codexcompletion.go`、`internal/transcript/codex.go` |
| 任务投递 | `[Pasted Content N chars]` 中的 N 与任务长度相符时，只能证明粘贴到了输入框。提交回执还须匹配已绑定的会话 ID。若 Enter 被吞，只在已记录的折叠草稿仍在时补发 Enter，不再次粘贴。 | `internal/dispatch/deliver.go`、`internal/dispatchbridge/dispatchbridge.go` |
| 桌面通知 | `PermissionRequest` 早于自动审核；只有已归属 pane 的编号菜单持续显示，才算需人处理。无法归属的完成事件不发通用横幅。HQ 例行完成静默，真实输入仍可通知。抑制原因写入结构化诊断。 | `internal/hook/hook.go`、`openspec/specs/notifications/spec.md` |
| Ghostty | Codex TUI 可绕过菜单栏通知队列，经终端转义序列另发通知。**新启动的 Codex HQ 进程**默认带 `-c 'tui.notifications=["approval-requested","plan-mode-prompt"]'`，除非命令已显式指定；普通 Codex 不变。`gtmux hq --rotate` 会等当前回合结束后，在**同一进程**内发送 `/new`，不能更新启动参数。退出进程后运行 `gtmux hq` 才会采用新参数。 | `internal/hq/hqagent.go`、`internal/hq/rotate_pending.go` |
| 对话 | 当前读取器支持 `response_item` 中用户的 `input_text` 和 agent 的 `output_text`；公开回复的阶段是 `commentary` / `final_answer`。对话在回合进行中就按顺序展示过程消息和工具步骤，不展示 analysis。旧版 `event_msg.user_message` / `agent_message` / `task_complete` 仍可读，重复的结尾只展示一次。注入的指引与环境快照不当用户消息。同一桌面版会话可能续写到文件名含 `<session-id>_<instance-id>.jsonl` 的新记录；gtmux 核对其中的 `session_meta.id`，按记录顺序合并各轮对话，任一记录增长时更新聊天缓存标识。未知记录不挡住后续轮次。 | `internal/transcript/codex.go`、`internal/transcript/transcript.go`、`internal/transcript/testdata/codex-current.jsonl` |
| tmux 外的会话 | `source: native` 只表示没有 tmux pane，不等于在终端窗口。匹配会话日志中的 `session_meta.originator` 可区分 ChatGPT 桌面版（`codex_work_desktop` 或 `Codex Desktop`）和终端 Codex（`codex-tui`），通过新增的 `client` 字段传给界面。这些名称须精确匹配；Codex 0.159.0 的桌面会话记录中已观察到两种写法。两者的日志 `source` 都可能是 `vscode`，不能拿它判断客户端；未知来源保持不标注。若 native 记录还处于工作中且没有可用的 Stop hook，同一会话所有匹配记录中的最新 `task_complete` 或 `turn_aborted` 可将其改判为空闲；更新的 `task_started` 则仍算工作中。菜单栏、手机列表和手机长按面板均显示来源。ChatGPT 桌面版会话不能转入 tmux：原客户端进程无法由 native 记录关闭，恢复后会出现两个客户端。其他 tmux 外会话保留原有操作。 | `internal/transcript/codex.go`、`internal/radar/agents.go`、`internal/app/adopt.go`、`macapp/Sources/GtmuxBar/MenuView.swift`、`mobileapp/src/ui/rowSheetModel.ts` |
| 桌面会话跟进 | 已确认来源的桌面会话默认仅显示状态，单列于「桌面应用」。用户按会话开启后，HQ 才能观察；通知与后续知识采集独立授权，初始关闭。停止会清除这些权限和本轮观察区间，保留已有记录。未知来源及其他 agent 保持原行为；桌面工作目录不代表 HQ 身份。 | `internal/sessionpolicy`、`internal/hq/unread.go`、`internal/radar/digest.go`、`internal/mine/mine.go`、`internal/server/sessionfollow.go`；[设计](desktop-follow.zh.md) |
| 手机终端 | 终端按手机宽度折行，不提供原宽模式。非全屏时，识别出的 Codex 截断提示可移进显示完整已记录提示的条；全屏时保留原始捕获行。浏览器的聚焦终端视图用 JavaScript 识别器，运行同一份共享用例。 | `mobileapp/src/ui/codexPinned.ts`、`mobileapp/src/ui/PinnedPrompt.tsx`、`mobileapp/src/screens/DetailScreen.tsx`、`internal/server/web/app.js` |

固定提示的识别以已观察到的 Codex 0.160.0 截图为依据：提示占第 0 行，换行并成空格，
截到 pane 宽度减一格，以 `…` 结尾，回合结束后仍保留。识别需要已知的 tmux `cols`（v1.0.53 起的服务端提供）、
`› ` 前缀、行宽在 `cols - 2` 到 `cols` 之间、规整后的提示开头至少六个 UTF-16 码元，
以及下方另一个 `› ` 行。最近十条非空的**已记录**提示去掉空白和变体选择符后，
必须恰好只有一种不同的提示匹配：以可见开头起始，而且比它长；仅在开头后加一个字面的 `…` 不算匹配。
同一提示重复出现算一种，不同提示都匹配时保留原屏幕。
若 `…` 本是提示正文，下一行还接着这条提示，也不提取。正在发送的提示不参与匹配。
手机非全屏终端和浏览器聚焦终端上，有截断行但尚未找到对应提示时，每四秒最多发起一次带缓存条件的对话请求。

排查用 `gtmux agents --json` 看 pane 与角色，用 `gtmux events --json` 和
`gtmux logs --component hook --json` 看事件与抑制原因。桌面会话缺少客户端标注时，
检查匹配日志中的 `session_meta.originator`，并核对会话 ID。新名称须有实际记录佐证后
再显式支持，不能根据 `source`、目录或标题猜测。

同一个 worker 同时出现在 tmux 和 native 列表、对话一直为空时，先查实时位置的
resume 绑定。已有真实日志却没有绑定，不是历史同步慢。
`codex.binding.confirmed`、`codex.binding.deferred`、`codex.binding.unavailable`
分别记录绑定成功、等待核实／重试和准备失败。旧 worker 没有投递标识时，须用独立
投递记录核实后修复，不能按目录、标题或最近时间猜归属。标识过期或目标变化后，
保留未绑定状态，不借用其他会话。终端、菜单栏、手机、iPad 和 Web 共用雷达与
对话数据，无需改协议或界面；真机验收仍是独立步骤。

Ghostty 通知还需核对
当前 HQ 的 **Codex 进程参数**：升级 gtmux 不会改变旧进程的参数。较晚出现的真实审批
菜单可由雷达下次轮询发现。手机 VoiceOver 与小屏布局仍需真机验收。

知识采集也支持没有 `ordinal` 的日志：缺少序号时，纠正线索使用稳定的文件／字节位置作为来源标识，增量上下文保留真实会话 ID。不同纠正不能共用回退的 `o0` 标识。
