# Agent hook 生命周期契约

[English](agent-hooks.md) · 核查日期 2026-10-11。这里只记录已有的八个 hook 集成，不新增 agent 支持。[Agent 接入流程](agent-onboarding.md)仍是实现检查清单。

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

## Doctor 与配置升级

先跑 `gtmux doctor`，再用 `gtmux doctor --fix` 执行建议修复。对存在的 agent 检查真实命令／事件参数、matcher、禁用条目、配置格式和插件模板，而非只看事件名称。Claude、Codex、Kimi 使用专用安装器；其余需检查的 agent 从注册表派生。

修复只替换 gtmux 的条目，写入前备份，保留第三方 hook，以及 Kimi 托管块之外的原始字节，并核实修复后配置。不明确的托管块边界、无效 JSON 会拒绝修改。全局禁用是用户选择，doctor 只报告，不悄悄开启；也检查 Gemini 的 [hooksConfig 开关](https://geminicli.com/docs/reference/configuration/)。

配置齐全不等于已加载：用户方便时重启相应 agent；Codex 在 `/hooks` 检查、信任；再看 hook 通路。doctor 不重启运行中的会话，也不绕过信任。项目／系统配置或组织策略可能覆盖用户目录配置，已有通路检查可帮助区分已配置与真实收到事件。

## 验证范围

回归覆盖 tmux／native 中断、迟到回调、新回合保护、不同载荷的结束原因、Codex 已绑定日志兜底、通知轮询抑制、错误／禁用绑定、第三方配置保留、Kiro V2→V3 和插件迁移。用 Node 运行实际生成的 OpenCode JS 模块及文档规定的 SDK／shell 契约，覆盖子会话与错误→idle 去重。

本轮是契约核查与回归验证，不等于八款上游应用全部真机端到端实测。旧版上游可能不发新事件；没有中断 hook 的路径继续依赖现有屏幕／日志识别。后续改集成时复核官方契约，并同步本页及英文版、安装器、doctor 和 OpenSpec。
