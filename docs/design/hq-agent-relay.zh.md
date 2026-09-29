# Agent → HQ 转接（第一版）

`gtmux relay report|ask` 把 agent 的请求保存到 gtmux 状态目录。来源由当前存活的 `TMUX_PANE` 确定；tmux 给出 session，派工台账给出任务 ID。调用者可以指定请求 ID 用于重试：同一 ID、同一内容返回原记录；内容变化则拒绝。先保存请求，再排队唤醒。普通进展只入台账；阻塞疑问只带 ID 和来源唤醒 HQ。HQ 用 `gtmux relay list/show` 读取、认领，然后回复或结案。唤醒是提示，台账才是待办来源。

每条请求独立存一个 JSON 文件，状态为 `pending → claimed → replied|closed`。写入使用进程锁和原子替换，并同步文件与目录，避免并发重复认领或读到半条记录。认领有租期；HQ 轮换或离线后可重新认领。回复先入账，再按 ID 重试投递；投递前核对原 pane、session 和 pane 进程。回复文本明确标注 HQ 来源，现有发送审计也记录作者。agent 之间没有直接通信路径。

源 pane 若已消失、被复用或退回 shell，回复留在台账，不会投进别人的会话。`gtmux knowledge sync` 后，托管的本机知识块会为 Claude、Codex、opencode、Kimi 提供简短的全局入口说明，新会话启动时加载；其他类型由新的交互式 `gtmux spawn` 派发内容告知。已运行的会话需重新加载指令，或主动查 `gtmux relay --help`。HQ 托管章程说明读取、认领、回复和权限边界。`--for user` 的请求只能由用户在源 pane 回答，HQ 可在之后结案，不能代替用户授权、决定保留给用户的方案或批准不可逆操作。一次性任务没有可接收异步回复的交互输入框，第一版不走回复通道。
