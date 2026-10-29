# Agent hook 生命周期契约

[English](agent-hooks.md) · 核查日期 2026-10-11。这里只记录已有的八个 hook 集成，不新增 agent 支持。[Agent 接入流程](agent-onboarding.md)仍是实现检查清单。

所有内置 agent 的历史来源和输入边界见[各 Agent 接入机制](../guides/agent-integrations.zh.md)；DSH 使用单独的[原生事件桥接](../guides/deepseek-harness.zh.md)。

hook 只观察 agent，不应批准工具、修改提示、阻止回合或重启会话。**正常完成、用户中断、终止错误是三种结果。** 中断和错误会清除工作、等待状态，把结束原因留给独立通知轮询器，不发送完成通知，也不更新最近完成的 pane。真正的新回合开始后才清除该原因。单个工具失败不等于父回合终止。

| Agent／官方契约 | 状态相关绑定与本轮修正 | 边界 |
|---|---|---|
| [Claude Code](https://code.claude.com/docs/en/hooks) | 保留提交、授权、会话、压缩、Stop／StopFailure；补 Elicitation／ElicitationResult 的 MCP 提问，以及解除等待的 PostToolUseFailure。 | 用户中断不会触发 Stop；未见公开顶层 Interrupt。工具失败、子 agent 完成不能结束父回合；仍需屏幕／日志兜底。 |
| [Codex](https://learn.chatgpt.com/docs/hooks) | 补 Interrupt；保留提交、授权、工具、会话、压缩。同步 hook 最多 3 秒，符合 Interrupt 上限；PostToolUse 解除等待。 | Interrupt 只覆盖正在运行的主线程回合，不包括空闲 Esc、子 agent。修改后需在 `/hooks` 检查并信任。共享 app-server 必须核实绑定会话，不能只信继承的 TMUX_PANE。 |
| [Gemini CLI](https://geminicli.com/docs/hooks/reference/) | BeforeAgent／AfterAgent、BeforeTool／AfterTool、会话、Notification、PreCompress；只有 notification_type=ToolPermission 才是授权等待。AfterTool 恢复工作；移除旧配置中错误的 PreToolUse／PostToolUse 注册。 | 普通工具开始不代表等待授权；未公开独立的中断或终止 API 错误 hook，不虚构事件。 |
| [Cursor](https://cursor.com/docs/hooks) | 改用 stop，根据 completed／aborted／error 区分正常完成、中断、失败。保留提交、会话、压缩、工具成功／失败；移除 afterAgentResponse、旧 beforeShellExecution／beforeMCPExecution。 | 一条回复不等于回合结束。授权决策 hook 在普通工具执行时也会触发，且要求响应格式；观察型桥接不能代替用户作权限决策。真实等待依靠屏幕判断；未知 stop 状态忽略。 |
| [GitHub Copilot CLI](https://docs.github.com/en/copilot/reference/hooks-reference) | 保留合法 PascalCase 别名；补工具成功／失败、PreCompact、ErrorOccurred。只有明确 recoverable=false 才结束回合，缺失或 true 只作遥测。 | PermissionRequest 早于规则、自动批准及用户提示，不能证明等用户。未公开专门用户中断事件；sessionEnd 是会话结束，不是普通回合完成。 |
| [Kiro CLI V3](https://kiro.dev/docs/cli/v3/hooks-migration/) | 全局 ~/.kiro/hooks/gtmux.json 使用 version=v1、trigger／action／timeout／enabled；注册 SessionStart、UserPromptSubmit、PreToolUse、PostToolUse、Stop，并清理 gtmux 旧 profile 中的重复绑定。 | CLI 2.x 保留 ~/.kiro/agents/gtmux.json，补 agentSpawn。已有 V3 全局／数组配置优先；否则限时查询版本，主版本 3+ 使用 V3。不能确定引擎时采用兼容 profile，通过 --agent gtmux 加载，包括 2.x 构建用 --v3 启动的场景。工具开始不等于等授权；未公开 Interrupt／SessionEnd／StopFailure。 |
| [OpenCode（`opencode`）](https://opencode.ai/docs/plugins/) | 使用当前全局 plugins/ 目录，只迁移自有旧 plugin/gtmux.js。根会话 busy 开始工作；MessageAbortedError 中断，其他 session.error 失败；只给活跃且正常结束的回合发一次 Stop。授权／问题答复恢复工作。 | 插件收到全局事件，需通过 SDK 的 session.parentID 排除子会话和未知归属。错误后的 idle 不能算成功；缺失事件不能猜。[SDK](https://opencode.ai/docs/sdk/)和[会话 schema](https://github.com/anomalyco/opencode/blob/dev/packages/schema/src/v1/session.ts)及[提问 schema](https://github.com/anomalyco/opencode/blob/dev/packages/schema/src/v1/question.ts)定义归属与中断错误。 |
| [Kimi Code](https://www.kimi.com/en/help/kimi-code/cli-hooks) | 补 TurnStarted、Interrupt；保留 StopFailure、授权请求／结果、工具、压缩、会话。PostToolUseFailure 仅作遥测。 | TurnStarted 包括任务／系统触发；UserPromptQueued 不代表已开始。TOML 只接受 event／matcher／command／timeout 四项，以托管块标记归属。只有 PermissionResult 解除授权等待，不拿无关工具结果替代。 |

## 归属与轮询

已知 turn_id 不匹配的迟到回调不能结束新回合。中断后的工具、授权、压缩回调不能重新置为工作中；只有真正的新提交／回合开始可以。tmux 外的 native 会话也有这层保护。没有回合标识的 agent 仍以会话级保护为准，不宣称可以解决所有事件乱序。

Codex 的 tmux 兜底只读取已绑定日志，最新 task_complete 或 turn_aborted 必须晚于当前工作及等待标记。更新的 task_started、会话不匹配、目录变化、未来时间或没有绑定都会阻止补判。中断路径保留 interrupted，不记正常完成。native Codex 同样读取自己的日志；无需改接口或 UI 数据结构。

### Codex API 错误缺少结束 hook

Codex 0.163.0 可能在 API 错误结束一轮时不触发 Stop、Interrupt 或旧 notify（[上游报告](https://github.com/openai/codex/issues/54884)）；其[日志持久化规则](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/rollout/src/policy.rs)也不保存 Error、StreamError，因此结束边界不能单独证明成功。不要注册尚未支持的 StopFailure hook。

tmux 雷达为已观察到的“Selected model is at capacity. Please try a different model.”增加严格的显示兜底：必须是可输入框前最后一个原生 `■` 错误块，且没有工作、排队输入提示或选择菜单。真实开始 hook 的前两秒优先，给上一帧留出重绘时间。终端、菜单栏、手机、iPad、Web 共同读取现有 idle + errored/error_text/end_reason=error，不增加状态。此判断不伪造 hook、不修改回合归属标记、不持久化屏幕错误；后续工作画面会清除此推断。其他错误文案、已滚走的错误、原生桌面会话，以及被精确复刻成终端错误块的输出，不在本检测保证内。回归使用合成画面，不向用户活跃会话制造错误。

### Codex 真实终端进程归属恢复

HQ 重用 pane 时，如果同目录还有其他会话，hook 归属可能因歧义而未更新，恢复记录便会保留旧 ID。恢复同时要求同一 Codex home 内明确的 CLI 创建记录、hook 记下的准确进程出生时间，以及刚验证的真实前台终端：稳定父链、pane 实例、前台进程组和字符设备 stdin。复用现有终端检查器，不单凭程序名、目录、创建来源或继承的 TMUX_PANE 认领；即使在前台 PTY 启动，也排除已知 app-server、mcp-server、exec 模式；参数仅即时检查，不保存或展示，扁平命令文本有歧义时保守延后。共享／无界面后端和管道不能认领终端。

任务投递 nonce 证明保持优先。新的主会话 SessionStart／UserPromptSubmit hook 可凭自身前台进程绑定；恢复已有观测则要求该客户端只有一个符合条件的会话。写入前再次检查进程、pane、原绑定及观测记录；新身份必须能解析为真实对话；空的启动记录延后，命令桩不能认领客户端，已知辅助提示词也被排除。原生记录和历史保留，由核心现有连接逻辑仅去掉准确绑定会话的外部重复项，终端、菜单栏、手机、iPad、Web 共用结果。输入、同意和跟进规则照旧。

出生时间通过 hook 的 UTC 读取器比较，避免本机时区差异。缺少来源／出生时间／stdin 证据、多个会话争用、记录变化、pane 不可读或已改变时保守放弃。这不证明其他前端已释放 Codex 写锁，也不是 Desktop 接管能力。回归使用合成进程和记录，不重启用户会话或发送诊断指令。

## Doctor 与配置升级

先跑 `gtmux doctor`，再用 `gtmux doctor --fix` 执行建议修复。对存在的 agent 检查真实命令／事件参数、matcher、禁用条目、配置格式和插件模板，而非只看事件名称。Claude、Codex、Kimi 使用专用安装器；其余需检查的 agent 从注册表派生。

修复只替换 gtmux 的条目，写入前备份，保留第三方 hook，以及 Kimi 托管块之外的原始字节，并核实修复后配置。不明确的托管块边界、无效 JSON 会拒绝修改。全局禁用是用户选择，doctor 只报告，不悄悄开启；也检查 Gemini 的 [hooksConfig 开关](https://geminicli.com/docs/reference/configuration/)。

配置齐全不等于已加载：用户方便时重启相应 agent；Codex 在 `/hooks` 检查、信任；再看 hook 通路。doctor 不重启运行中的会话，也不绕过信任。项目／系统配置或组织策略可能覆盖用户目录配置，已有通路检查可帮助区分已配置与真实收到事件。

## 验证范围

回归覆盖 tmux／native 中断、迟到回调、新回合保护、不同载荷的结束原因、Codex 已绑定日志兜底、通知轮询抑制、错误／禁用绑定、第三方配置保留、Kiro V2→V3 和插件迁移。用 Node 运行实际生成的 OpenCode JS 模块及文档规定的 SDK／shell 契约，覆盖子会话与错误→idle 去重。

本轮是契约核查与回归验证，不等于八款上游应用全部真机端到端实测。旧版上游可能不发新事件；没有中断 hook 的路径继续依赖现有屏幕／日志识别。后续改集成时复核官方契约，并同步本页及英文版、安装器、doctor 和 OpenSpec。

### 共享 Codex 客户端归属

[终端会话归属](codex-terminal-identity.zh.md) 为支持的共享后台增加客户端根请求成功回执。hook 将捕获的后台 PID／启动时间与精确当前根会话核对；根工作目录可以不同于外层 shell。携带 `agent_id` 的 Codex hook 不改变父资料和生命周期。旧独立进程恢复及任务投递证明优先级保留。升级期间，活动桥接可恢复旧 hook 写回相同 thread 时遗漏的新增归属字段，不能抢回其他会话。
