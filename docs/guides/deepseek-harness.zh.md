---
title: DeepSeek Harness Web 对话
description: 查看 DSH Web 对话，并接入专用原生生命周期事件。
order: 5
---

[English](deepseek-harness.md) · **中文**

Gtmux 从 `$DSH_HOME/sessions`（默认 `~/.dsh/sessions`）发现已保存的用户对话。
在菜单栏、手机、iPad 或 Web 的**外部会话**中打开，查看用户提示、助手公开文本和工具步骤。
终端可先运行 `gtmux agents --json`，再使用对应行的 `session_id` 和 `home_id`：
`gtmux transcript SESSION_ID --home-id HOME_ID --json`。继续交互请回到 DSH Web。
读取历史不会开启 HQ 跟进、通知或知识留存；访客无权读取或更改这些外部会话。

适配器支持 DSH **0.2.0-rc.2** 当前使用的 **v4** JSONL，包括多个带校验和的
Zstandard 帧。选择已保存的最高格式代际；遇到未知代际、损坏记录、重复身份或同一代际的
两种编码会拒绝读取，不迁移或偷偷回退旧格式。尚未写完的末帧留待下次轮询。
列表展示最近变更的 128 个根会话，指定准确 ID 仍能读取更早的会话。子 agent 和没有
用户消息的会话不进入列表。视图只保留有上限的近期窗口，并报告省略的轮数。

这里展示公开追加历史，包括模型压缩前的用户对话，不是 DSH 重建的模型上下文。
注入指令、压缩检查点、隐藏推理、模型重放数据和原始工具结果不会展示。
无需读取凭证文件、获取写锁或访问 DSH HTTP 接口。保存的活动不能证明某个 Web
页面现在仍连接着，控制端显示为未确认。Gtmux 不恢复、接管或向 DSH 发送输入。
本轮不支持 TUI 控制和用量、上下文占比。

## 原生生命周期插件

读取已保存对话不需要插件。接入后续生命周期事件使用 gtmux 自带的专用 **Cordis 插件**，
无需另装 npm 包：

```sh
gtmux install hooks --agent dsh
```

自动识别已有 agent 目录或可执行文件并安装 hooks，可用 `gtmux install hooks --agent auto`；
不带 `--agent` 仍保持默认 Claude。自动模式会保留配置中禁用 hook 的选择。
`gtmux doctor` 也能识别 DSH，同时检查模块和准确的启动覆盖内容；
`gtmux doctor --fix` 修复同一组自有文件。文件已装不等于运行中的 Web 已加载：
doctor 会给出下次启动命令，并明确标注运行中的加载状态尚未核实。

命令会输出 `dsh web --patch '…/gtmux/gtmux.cordis.yml'`。下次主动启动 Web 时
加上这个覆盖文件即可。安装器不重启、不改动当前服务；已有对话的读取也不依赖插件。
安装器仅管理 `$DSH_HOME/gtmux/gtmux.mjs` 和 `gtmux.cordis.yml`，保留用户配置
和其他插件。`gtmux uninstall hooks --agent dsh` 仅删除这两个自有文件；删除后
后续启动不再传入该覆盖文件。

插件直接使用 DSH 的 `session/event` 和 `user-questions/request` 原生接口，
不经过 Codex 或 Claude 兼容 hook。它观察根会话的用户提示、开始、完成、中断和失败，
把每次提问原样交给 DSH 原有处理器，等请求结束后清除等待状态，不回答问题或批准工具。
`approval/asked` 可能被自动策略或其他插件处理，因此不据此声称“等待你批准”。
目前没有核实到明确代表人工审批卡片的原生 Host 事件，审批仍在 DSH Web 处理。
子进程使用管道输入、有上限的队列和超时，不会抢走终端输入，也不会因 CLI 不可用阻塞 DSH。

每段会话分别开启 **HQ 跟进**，HQ 才能观察后续活动；**会话通知**另行开启完成和
提问通知，**知识留存**另行允许从后续活动整理经验。关闭 HQ 会同时关闭后两项。
重新开启知识留存会建立新的时间边界，不追溯挖掘旧对话。授权同时绑定 agent、规范化
数据目录和会话 ID，复制到另一个目录的同名 ID 不继承授权。设置带版本校验，避免
悄悄覆盖另一设备的修改。生命周期事件流需要原生插件；仅轮询已保存历史只读取元数据，
不会把旧事件重新投递给 HQ。

鲸鱼标记来自 [DeepSeek Harness 官方 favicon](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/apps/web/public/favicon.svg)，
来源及校验值见 [图标溯源](../../assets/agent-icons/SOURCES.md)。
原生接口依据：[会话类型](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/core/session/src/types.ts)、
[会话观察器](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/session/session-telemetry/src/coordinator.ts)、
[审批服务](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/interaction/user-approval/src/index.ts)。

首次发现会遍历选中的文件建立元数据，之后未变化的轮询复用索引，追加读取只解码新的完整帧。

模型表层的替换副本不会当作新提示、生命周期事件或学习输入，原先追加的公开对话仍可见。
