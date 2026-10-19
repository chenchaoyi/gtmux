---
title: DeepSeek Harness 对话
description: 读取 Web 历史，管理已核验的 DeepSeek Harness 终端会话。
order: 18
---

从 Web 或支持的社区终端客户端使用，身份都保持 DeepSeek Harness，沿用官方鲸鱼图标。
dst 是首个终端适配器，不是另一种 agent。各 agent 的实现统一见[接入说明](agent-integrations.zh.md)。

## Web 历史

Gtmux 从 `$DSH_HOME/sessions`（默认 `~/.dsh/sessions`）发现保存的根会话，在菜单栏、手机、
iPad、Web 的外部对话中只读显示。明确的 CLI 读取不需要插件、凭证、模型请求或会话写入租约：

```sh
gtmux transcript <session_id> --json
```

支持 DSH 0.2.0-rc.2 的 v4 JSONL 和带校验的连续 Zstandard 帧。未写完的尾部等下一轮；
未知代际、身份歧义或损坏源文件会拒绝。可读公开文本和工具步骤，排除思考、原始工具结果、
注入与替换、重放副本。发现本身不开启 HQ 跟进、通知或学习；需用既有
`gtmux follow <session_id> …` 为单个外部对话设置。Web 历史不能输入、adopt 或由 gtmux 重启。

## 原生桥接与终端安装

不用另装 gtmux npm 包。Gtmux 打包桥接脚本，Cordis 运行时由 DSH 自己提供。先单独安装
官方 DSH CLI 和社区 dst，完成 dst 首次 profile 初始化，再运行：

```sh
gtmux install hooks --agent dsh
# --agent auto 和常规安装也会探测 dsh/dst。
dst --backend dsh
gtmux doctor
```

安装器在 `$DSH_HOME/gtmux` 写出自有模块、组件清单与覆盖文件；已有 dst profile 会在
`profiles/dsh-tui/cordis.patch.yml` 增加自有标记段，保留其他文本与自定义标签，按内容生成
备份。卸载只删该段和自有文件。符号链接、歧义配置或归属不明的文件会拒绝处理；非空行内
根数组需先转为块序列。不会自动安装、升级第三方客户端，也不会重启现有进程。

若要接 **Web 的原生生命周期事件**，下次主动启动 Web 时使用安装器另外打印的
`dsh web --patch '…/gtmux/gtmux.cordis.yml'`。读取历史不依赖它。桥接使用 DSH 的
`session/event` 和 `user-questions/request`，不走 Codex 兼容 hook，不代答、不批准权限。
Doctor 分开报告安装、终端实时绑定和输入权限，不能证明运行中的 Web 已加载覆盖文件。

## 受管终端会话

终端适配器针对 dst 0.16.1 的 DSH 后端。实时桥接需与精确数据目录、根会话、进程祖先、
tmux 代际和前台选择一致，核心才呈现受管 DeepSeek Harness pane。后台挂载、dst 的其他
后端、过期或歧义记录不能授权输入。受管期间，同一对话不再重复出现在外部历史列表。
Mac、手机、iPad、Web 和 CLI 消费同一份核心数据及图标。

可使用既有 pane 捕获、聚焦、对话与发送流程。`gtmux spawn --agent dsh` 启动声明的客户端；
核验过的终端恢复记录限定相同数据目录和客户端。模型在 dst 中选择，暂不支持通用
`--model` 和一次性派发。正文通过目标明确的原生命令提交，私有一次性请求由真实接收
agent 校验后返回回执。保留草稿保护，切换会话、菜单、提问、Vim 普通模式或展开编辑器时
拒绝提交；窗口过窄导致令牌折行也可能拒绝。回执只证明入队，不代表执行或任务完成。
结果不明不自动重投；完成请求的重试记录保留 24 小时，未解决请求到有界上限后停止新发送。

## HQ 逐条审阅公开消息

核对器增量恢复已核验前台区间内提交的公开用户与助手消息，不自动纳管较早的 Web 或后台
活动。`ConversationMessage` 事件携带消息身份与简短公开摘要，完整公开正文按页读取：

```sh
gtmux transcript <session_id> --messages --after-seq -1 --limit 100 --json
# 判断完本页返回的消息，再明确声明审阅进度：
gtmux transcript <session_id> --messages --after-seq -1 --limit 100 --review-through <last_message_seq> --json
```

随后从 `reviewed_through` 继续，`has_more` 表示还有下一页。读取消息、消费 HQ 事件、收到
通知和输入回执都不会自动标记已审阅。审阅用于观察与跟进，不是事前批准关卡。源失败、替换
或截断会产生 `ConversationGap`，替换会重置审阅覆盖；应核对缺失历史，不能声称完整覆盖。
分页只含文本消息，工具及权限交互仍在原客户端。DSH 用量统计尚未实现。

## 来源与验证

各端图标来源为[官方 favicon](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/apps/web/public/favicon.svg)。
原生接口依据 [DSH 会话](https://github.com/deepseek-ai/deepseek-harness/tree/dsh-v0.2.0-rc.2/packages/core/session)和
[dst v0.16.1](https://github.com/ccch1mneyyy/dsh-TUI/tree/73ff99fbed56563bf154d719effd09f9f9725349)。
信任、保留和验证边界见[终端设计](../design/dsh-terminal-clients.zh.md)。作者构造数据测试、
真实组件加载、TUI 原生入队和模型执行是不同证据；本机版本组合实际通过哪些，以交付报告为准。
