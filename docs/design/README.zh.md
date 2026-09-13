# gtmux 设计文档

一个产品、五种形态（终端含远程 attach · 菜单栏 · 手机 · iPad · Web）的设计权威，共用一套状态语言
（色 + 形 + 字形）。这里每份文档都是一对：`<name>.md` 英文、`<name>.zh.md` 中文，同一个 PR 里一起改
（带日期的记录类文档只有一种语言，`scripts/check-design.sh` 里列着）。

## 从哪开始

| 文件 | 用途 |
| --- | --- |
| `SURFACES.md` | 五种形态：每个提案都要过一遍的清单，以及靠结构防偏移的四条。 |
| `DESIGN.md` | 菜单栏权威规范；§0–§3 是各形态共用的状态语言。 |
| `MOBILE.md` | 手机与 iPad 的权威规范（App 图标 / Agent 图标 / 交互 / 推送 / 状态）；§5 是 iPad，同一个 app 的 regular 壳。 |
| `WEB.md` | Web 浏览器镜像权威规范（工作台 / 只读红线 / 对话模式 / 头像 / 键盘）。 |
| `knowledge-layers.md` | 三层知识（出厂章程 / 你的守则 / 本机台账）：谁写、何时进谁的脑子、怎么从一层升到另一层。 |
| `knowledge-engineering-research.md` | 知识引擎背后的调研：九种做法的对照，借了什么、没借什么。 |
| `agent-onboarding.md` | 接入或迭代一个 coding agent：支持分层、注册表是身份唯一来源、逐步流程、踩坑清单。 |
| `HANDOFF.md` | 一轮设计的落地顺序与验收。 |
| `SECURITY.md` | 安全姿态与边界。 |
| `remote-access-tunnel.md`、`remote-attach-research.md`、`server-mode-research.md`、`multiplexer-research.md`、`multi-agent-multi-terminal.md`、`mosh-predictive-echo-research.md` | 决策背后的调研，留着让下一轮读记录而不是重推一遍。 |
| `ITERATIONS-2026-06.md`、`REVIEW-mobile-01.md`、`AUDIT-2026-09-07.md`、`HANDOFF-mobile-2026-06.md`、`DECISIONS-FOR-CCY.md`、`RESEARCH-prior-art-2026-06.md` | 某一轮、某一次评审的带日期记录。历史，不再更新。 |
| `mockup/gtmux-menubar.dc.html`、`mockup/gtmux-mobile.dc.html`、`mockup/gtmux-web.dc.html` | 可交互原型（浏览器打开，运行时联网加载）。 |
| `mockup/preview-*.png` | 菜单栏静态参照。 |

改任何 UI 前，先读对应形态的权威规范并遵循；要偏离就先提出、再写回文档，不留下没说的偏差。
