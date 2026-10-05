# Agent → HQ 转接（第一版）

`gtmux relay report|ask` 把 agent 的请求保存到 gtmux 状态目录。来源由当前存活的 `TMUX_PANE` 确定；tmux 给出 session 名称和 pane PID，派工台账给出任务 ID。调用者可以指定请求 ID 用于重试：同一 ID、同一内容和来源返回原记录；内容或来源变化则拒绝。先保存请求，再排队唤醒。进展默认不阻塞，加 `--blocking` 也会唤醒 HQ；疑问默认阻塞，除非加 `--nonblocking`；`--for user` 总是阻塞请求。唤醒带 ID、来源、请求种类和读取命令。HQ 用 `gtmux relay list/show` 读取、认领，然后回复或结案。唤醒是提示，台账才是待办来源。

每条请求独立存一个 JSON 文件，状态为 `pending → claimed → replied|closed`。写入使用进程锁和原子替换，并同步文件与目录，避免并发重复认领或读到半条记录。认领五分钟后可被重新认领，但不会因此拒绝稍后的回复或结案；记录保存认领时间，不保存认领者身份。回复先入账，再投递；失败后由 HQ 显式执行 `gtmux relay retry <id>` 重试，relay 没有自动重试循环。`delivered` 字段表示派送层确认送达，**或从屏幕上看到 agent 把消息排在当前回合之后**，不表示 agent 已读或已处理。回复文本明确标注 HQ 来源，现有发送审计也把作者记为 HQ。relay 不提供 agent 间直达路径；`gtmux send` 是另一个命令。

投递前会核对保存的 pane ID、tmux session 名称和 pane PID，并拒绝当前命令是已知 shell 或 `gtmux` 的 pane；不满足这些检查时，回复留在台账。这些检查识别的是 pane 生命周期。pane PID 通常属于其 shell，因此同一 pane 中更换 agent 进程或新开对话都无法识别。命令检查也不是 agent 白名单，只排除已知 shell 和 `gtmux`。`gtmux knowledge sync` 成功后，托管的本机知识块会为 Claude、Codex、opencode、Kimi 提供简短的全局入口说明，新会话启动时加载；其他类型由新的交互式 `gtmux spawn` 派发内容告知。已运行的会话需重新加载指令，或主动查 `gtmux relay --help`。HQ 托管章程说明读取、认领、回复和权限边界。`--for user` 的请求只能由用户在源 pane 回答，HQ 可在之后结案，也可把其他问题转给用户，但不能代替用户授权、决定保留给用户的方案或批准不可逆操作。一次性任务没有可接收异步回复的交互输入框，第一版不走回复通道。
