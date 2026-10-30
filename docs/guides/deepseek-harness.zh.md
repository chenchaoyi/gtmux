---
title: DeepSeek Harness 对话
description: 读取 Web 历史，管理已核验的 DeepSeek Harness 终端会话。
order: 18
---

gtmux 可以读取 DeepSeek Harness 的 Web 历史，并管理通过 dst 接入、已核验的终端会话。两种入口都显示为 DeepSeek Harness，使用官方鲸鱼图标。各 agent 的接入方式见[接入说明](agent-integrations.zh.md)。

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

可使用既有 pane 捕获、聚焦、对话与发送流程。`gtmux spawn --agent dsh` 直接启动声明客户端的聊天工作区，跳过首页；
新会话在首条历史文件之前也能通过原生实时身份核验。核验过的终端恢复记录限定相同数据目录和客户端。模型在 dst 中选择，暂不支持通用
`--model` 和一次性派发。正文通过目标明确的原生命令提交，私有一次性请求由真实接收
agent 校验后返回回执。保留草稿保护，切换会话、菜单、提问、Vim 普通模式或展开编辑器时
拒绝提交；窗口过窄导致令牌折行也可能拒绝。回执只证明入队，不代表执行或任务完成。
结果不明不自动重投；完成请求的重试记录保留 24 小时，未解决请求到有界上限后停止新发送。

终端关闭或绑定失效后，保存的对话可能出现在只读外部会话中。这里的“外部”表示当前没有
受管 pane，不代表最初从 Web 启动。保留日志能将精确数据目录、会话和原生消息序号关联到
成功投递时，历史仍会标记 HQ 或其他 agent 的发送者身份；不能仅凭文字相同判断。证据缺失、
过期或有歧义时不标记，读取历史归属也不会恢复输入权限。

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

## 终端重绘时的原生状态

已验证且接入原生生命周期事件的终端客户端，会单独保存一份原生事件状态，绑定当前
harness、数据目录、会话、pane 和客户端进程实例。开始、提问、恢复和结束 hook 决定状态；
屏幕/CPU 变化及通用 active/finished 缓存不能覆盖它。“完成时间”缓存也可能由雷达推断生成，
因此不能证明原生回合已结束。

同一个 DSH 会话的持久化历史若出现更晚的明确回合边界，可以补救漏掉的 hook。
历史的默认状态、不相关会话和更旧的边界不能覆盖原生事件。读取复用现有增量 DSH 索引，
不扫描全局事件日志。两种证据都没有时保留中性状态（`running`），不编造完成时间。
旧会话需要自然发生的 hook 或明确历史边界，不猜测补填，也不需要额外安装插件。
原生提问在界面重绘时保持等待，直到原生恢复/结束事件或更晚的明确回合边界出现。
这项能力由适配器声明，换社区客户端时 harness 身份仍为 `dsh`；其他接入保留各自文档说明的回退。
已跟踪但仍卡在输入框中的未投递目标，继续使用独立的投递等待判断。

## 来源与验证

各端图标来源为[官方 favicon](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/apps/web/public/favicon.svg)。
原生接口依据 [DSH 会话](https://github.com/deepseek-ai/deepseek-harness/tree/dsh-v0.2.0-rc.2/packages/core/session)和
[dst v0.16.1](https://github.com/ccch1mneyyy/dsh-TUI/tree/73ff99fbed56563bf154d719effd09f9f9725349)。
信任、保留和验证边界见[终端设计](../design/dsh-terminal-clients.zh.md)。作者构造数据测试、
真实组件加载、TUI 原生入队和模型执行是不同证据；本机版本组合实际通过哪些，以交付报告为准。


## 恢复与图形界面中的工具识别

存档中的 `node /…/bin/dst` 或直接 dst 命令，只有同时匹配此前已核验的 DSH 数据目录、客户端及会话恢复记录，且根对话仍存在，才能作为恢复依据。明确指定其他 backend、保存为空闲 shell、缺少记录或数据目录不符时，均不能据此恢复。外层命令识别不授权实时输入，当前进程、pane 和原生绑定仍单独验证；只读 Web 历史不会因此被自动拉进终端。

图形界面可直接发现 NVM 安装位置，不运行登录 shell；启动 dst 时为该子进程带上配套 Node 的目录。更新或新版本启动时可维护已有的 gtmux 自有桥接，不改 profile、不重启客户端。没有可选客户端或当前没有接入会话属于信息，不是持续故障。
