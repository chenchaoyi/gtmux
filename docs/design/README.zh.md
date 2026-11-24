# gtmux 设计文档

终端、菜单栏、手机、iPad 和网页共用状态颜色、形状和符号。更新现行设计时在同一个 PR 中改中英文两版。
单语历史记录的例外列在 `scripts/check-design.sh`。

## 从哪开始

| 文件 | 用途 |
| --- | --- |
| `SURFACES.md` | 五种形态：每个提案都要过一遍的清单，以及靠结构防偏移的四条。 |
| `DESIGN.md` | 菜单栏权威规范；§0–§3 是各形态共用的状态语言。 |
| `MOBILE.md` | 手机与 iPad 的权威规范（App 图标 / Agent 图标 / 交互 / 推送 / 状态）；§5 是 iPad，同一个 app 的 regular 壳。 |
| `WEB.md` | Web 浏览器镜像权威规范（工作台 / 只读红线 / 对话模式 / 头像 / 键盘）。 |
| [知识分层](knowledge-layers.zh.md) | 托管章程、用户规则和本机台账分别由谁维护，何时加载，如何把经验提供给其他受众。 |
| [数据目录](state-storage-lifecycle.zh.md) | 托管目录、离线分组、故障续行和回退。 |
| `hq-move-between-macs.md` | 已实现的新 Mac 迁移：预览、暂存并核对选中的知识和个人要求；不迁入旧态势板和生成的指令。 |
| `knowledge-engineering-research.md` | 知识引擎背后的调研：九种做法的对照，借了什么、没借什么。 |
| `terminal-keyboard-research.md` | 终端通用按键：移动终端借鉴、功能键/修饰键、按键面板、兼容和验收边界。包含实现和验证边界。 |
| [libghostty 调研](libghostty-research.zh.md) | 2026-11-10 原生/Web PoC、固定版本、渲染缺陷与配对测量；本轮为何不替换，以及何时值得重新评估。 |
| [各 Agent 接入机制](../guides/agent-integrations.zh.md) | 按每个内置 agent 说明发现、事件、历史、输入、安装与边界，链接具体实现。 |
| `agent-onboarding.md` | 接入或迭代一个 coding agent：支持分层、注册表是身份唯一来源、逐步流程、踩坑清单。 |
| `HANDOFF.md` | 原始设计交接的实施顺序和验收记录。新改动按当前界面的规范执行。 |
| `SECURITY.md` | 安全姿态与边界。 |
| `remote-access-tunnel.md`、`remote-attach-research.md`、`server-mode-research.md`、`multiplexer-research.md`、`multi-agent-multi-terminal.md`、`mosh-predictive-echo-research.md` | 决策背后的调研，留着让下一轮读记录而不是重推一遍。 |
| `ITERATIONS-2026-06.md`、`REVIEW-mobile-01.md`、`AUDIT-2026-09-07.md`、`HANDOFF-mobile-2026-06.md`、`DECISIONS-FOR-CCY.md`、`RESEARCH-prior-art-2026-06.md` | 某一轮、某一次评审的带日期记录。历史，不再更新。 |
| `mockup/gtmux-menubar.dc.html`、`mockup/gtmux-mobile.dc.html`、`mockup/gtmux-web.dc.html` | 可交互原型（浏览器打开，运行时联网加载）。 |
| `mockup/preview-*.png` | 菜单栏静态参照。 |

修改界面前先读对应设计和验收要求。共用行为变化时，一并更新受影响的界面及其说明。

[DeepSeek Harness 终端客户端](dsh-terminal-clients.zh.md)：原生客户端适配、目标明确的输入和显式消息审阅。
