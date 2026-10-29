# Codex 终端会话归属

[English](codex-terminal-identity.md) · **中文**

后台进程与终端客户端是两种身份。Codex 0.156 增加可选的自动后台，0.157 对符合条件的 TUI 默认启用。原生 hook 仍通过 `session_id` 标识对话；子 agent 可同时携带父会话与 `agent_id`。因此共享后台 PID 和继承的 `TMUX_PANE` 不能证明调用来自哪个终端。官方出处：[0.156 说明](https://github.com/openai/codex/releases/tag/rust-v0.156.0)、[0.157 说明](https://github.com/openai/codex/releases/tag/rust-v0.157.0)、[hook 字段](https://learn.chatgpt.com/docs/hooks#common-input-fields)。

## 归属证据

`internal/codexbridge` 经私有 Unix socket 转发一个终端客户端的本机 WebSocket 连接。通过内核核实同用户对端、真实前台 Codex PID 与启动时间、TTY、pane 实例、后台 PID 与启动时间及规范化 Codex home。只有该连接自己成功的根会话 `thread/start`、`thread/resume`、`thread/fork` 响应才更新归属。失败请求、全局通知、已加载列表、历史、临时标题助手和子 agent 视图均不能抢占 pane。请求与连接序号阻止旧响应覆盖新归属。`/new` 在首条消息之前即可建立新根会话。

私有端点遵循 Codex 的受保护本机后台布局，不覆盖默认 socket、权限或访问设置。协议帧原样转发，上限 64 MiB；只有身份请求进入有界的待处理表，桥接层不记录或保存协议正文。0.165 启动时实际有约 24 MB 的目录响应，因此观察身份时的小解析上限不能用作转发上限；这些传输数据不等于进入模型上下文。

私有元数据每秒保活，五秒失效；输入前另核前台进程、TTY、后台和根会话。退出只删除本客户端的活动回执与 socket，保留可恢复历史。只有精确客户端与后台证据才可按记录代际清退未变化的最小 SessionStart；真实原生回合和后续独立活动保留。不会为清理一个客户端而停止共享后台。

## 启动与兼容

`gtmux codex -- …` 按实际能力接入交互启动，不按版本号硬切。标准派发、HQ、恢复共用同一适配。hook 安装和 doctor 提供可移除的 Bash/zsh 配置段，对新开的 tmux 交互终端生效；保留现有别名和函数，卸载只移除本配置段。不会重启现有会话或补造归属凭据。

旧独立客户端继续使用前台进程与 hook 验证。显式 `--no-daemon`、remote、profile/config 覆盖、非交互、未知参数和自定义包装命令保留原生参数、环境及退出行为，不生成未经核实的桥接归属。符合条件的首次交互启动可仅调用官方 `codex app-server daemon start`；读取列表和历史不启动、不恢复客户端。子进程启动前失败可回退到原命令一次，启动之后不会自动再拉起一个客户端。

实现依据固定为官方 `rust-v0.165.0` 的 `360319d8dee62d87e2d59fc0c21805d79519cfed`：`codex-rs/tui/src/daemon_startup.rs`、`codex-rs/tui/src/app_server_connection.rs`、`codex-rs/app-server-protocol/src/protocol/v2.rs`、`codex-rs/app-server-transport/src/uds/daemon_directory.rs`。能力不满足时安全回退原生，不声称自动支持未来协议。

## 输入、HQ 与回信

经验证的桥接客户端用 `thread/queue/add` 精确指定 home/thread 和后台实例，继续遵循草稿、等待提示、目标复核、HQ 输入租约与 hold 保护。文字和已授权上传的图片保留输入类型。复用持久输入协议，在变更前独占领取发送尝试。客户端消息 ID 与队列 ID 对应才算接收；队列接收不等于执行。写入不确定时保留状态，不回退粘贴，也不自动重放；相同已完成 ID 返回已有回执。用户明确确认的不确定重试使用新的传输代际。

agent 工具命令优先使用 `CODEX_THREAD_ID`，兼容旧 `CODEX_SESSION_ID`，结合 home 与当前归属核实来源。继承的 pane、cwd 或子 agent 的父身份不授予 HQ 权限。报告保留精确 thread/home/进程，未确认 pane 时可只关联对话。HQ 授权、事件消费、回声抑制和 CLI 审计共用此解析。回信走已核实的旧独立进程路径或精确原生 thread 路径；来源变化或不可用时仍可从台账读取，不盲目重试不确定投递。

所有平台消费同一 Go 核心证据。菜单栏待发送操作使用一致的 caption 字体与“重试／取消发送”。浏览器、手机和 iPad 沿用既有输入授权，并接收排队、拒绝、不确定结果；不从 logo、originator、标题或目录猜测归属。

## 验证边界

合成用例覆盖临时助手、乱序根请求、重连序号、PID 复用、多个 home、外来 hook、过期、旧启动方式、shell 覆盖、队列回执、持久去重、不确定写入、草稿／hold 和 worker/HQ 身份。真实 0.165 共享客户端与显式 `--no-daemon` 对照验证当前两种架构。历史二进制、所有包装命令、缓存子会话切换 UI、真机投递和 VoiceOver 仍分别验收；根协议用例不等于设备验收。通用 Desktop 接管仍不可用，不归档历史、不改变远程访问策略。

托管 Codex 的 HQ 唤醒批次同样走受保护的精确输入。不确定批次单独保留原条目供核对，并纳入降级提示；不会重新组批成新消息 ID 后重放。其他 agent 沿用原有终端唤醒协议。

启动保留显式 `-C`／`--cd`，仅在未提供时补齐原终端工作目录。根响应的实际工作目录与外层 shell 目录分开保存，避免恢复和 `/new` 后误归属；hook 还核对捕获的后台启动时间。携带 `agent_id` 的 Codex 子 agent hook 不覆盖父资料，也不完成或唤醒父回合。

浏览器轨迹仅在同一条记录仍存在时保留选中文字。替换 session 后，已消失记录的详情立即更新；窄屏同样处理，避免新列表下仍显示旧会话详情。
