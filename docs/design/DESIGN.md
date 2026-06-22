# gtmux 菜单栏 app — 设计要求（Design Requirements）

> 本文件是 gtmux 菜单栏 app 的**权威设计规范**。实现以本文为准；可视化参照见
> `docs/design/mockup/`（可交互原型）与 `docs/design/mockup/preview-*.png`（静态截图）。
> 任何 UI 改动都应遵守这里定义的 token、状态语言与交互规则。

目标实现：**原生 macOS — NSStatusItem + NSPopover + SwiftUI**（自定义 popover 视图，
不是系统 NSMenu）。数据来自 `gtmux agents --json`（契约见 `internal/menubar/model.go`）。

---

## 0. 设计原则

1. **一瞥工具（glance tool）**：一天看几十次。安静、快、可扫读，胜过装饰。
2. **层级即一切**：「需要你」要响，「空闲」要静到几乎不打扰。
3. **绝不只靠颜色**：每个状态 = 颜色 + 形状 + 字形 三重编码（色盲 / 余光 / 着色菜单栏都可读）。
4. **原生、克制、可信**：这是开发者工具，不是消费级 app。无彩虹渐变、无发光阴影、无营销腔文案。
5. **动效最小**：至多在 idle→waiting 时给一次提示；其余零动画。

---

## 1. 状态模型（核心）

每个 agent 有四种状态（与 CLI 一致，见 `internal/app/agents.go`）：

| status | 含义 | 颜色（权威，来自 `internal/menubar/icon.go`） | 形状 | 字形（白色，置于徽章内） |
| --- | --- | --- | --- | --- |
| `waiting` | 被你卡住、阻塞在你的输入上（最紧急，排最前） | `#EF4444` 红 | **方形**（圆角 ~3.5px） | **双竖线 ⏸**（暂停） |
| `working` | 忙碌、运行中（别打扰） | `#06B6D4` 青 | 圆形 | **加载环**（开口圆环，**静态不旋转**） |
| `idle` | 这一轮跑完、轮到你（不紧急） | `#22C55E` 绿 | 圆形 | **对勾 ✓**（完成） |
| `running` | 在 prompt 待命、存活 | `#8E8E93` 灰 | 圆形 | **小圆点** |

- 「最紧急状态」优先级：`waiting > working > idle/running`。
- 这套字形语言**全表面统一**：菜单栏状态项、popover 行徽章、图例、偏好设置里都用同一套。
- 颜色这一信道**只**留给状态，不要用来区分 agent 类型（见 §6）。

---

## 2. 菜单栏状态项（NSStatusItem）— 最重要的表面

一个 16pt 字形，要在余光、色盲、浅/深/着色菜单栏上都能秒读。**WAITING 必须靠形状+字形，不只靠颜色。**

### 字形（推荐方案：Shape-shift / 随状态换形）

状态项的字形随「最紧急状态」切换形状，而不仅是换色：

- **calm（0 / 全空闲可选隐藏）**：灰色**空心环**。
- **idle**：绿色 **✓**（或 sparkle）。
- **working**：青色**加载环** + 计数。
- **waiting**：红色**方块 + 双竖线** + 计数。

形状本身就是信号 → 余光 / 色盲都能读。

### 三种显示模式（偏好可选，对应 §10「状态栏显示」）

1. `dot` — 仅圆点（颜色 = 最紧急状态）。
2. `dot + count`（默认）— 圆点/字形 + 最紧急可操作状态的数量（`waiting` 否则 `working`，见
   `BadgeText`）。
3. `hide-when-idle` — 没有人等你时**完全隐藏**状态项，绝不打扰。

### 渲染与适配

- 彩色字形是**非模板图像**（non-template），在系统给菜单栏着色时仍保留红/青/绿
  （现状见 `icon.go` 的 `IconFor`/`dotPNG`，非模板）。
- **计数数字是模板图像**（template），自动随菜单栏黑/白。
- **着色壁纸**：当红方块在红/橙壁纸上对比不足时，自动加 **0.5pt 白描边**（或翻白填充）。
- 交付优先 **SF Symbols + tint token**，失败回退 **@1x/@2x PNG**（22×22 抗锯齿，参照 `dotPNG`）。
- 笔记本**刘海**：状态项在刘海右侧，宽度预算要小。

### tint / symbol token

| status | SF Symbol（建议） | tint |
| --- | --- | --- |
| waiting | `pause.fill` / 自绘双竖线方块 | `#EF4444` |
| working | `circle.dotted` / 加载环 | `#06B6D4` |
| idle | `checkmark` / `sparkle` | `#22C55E` |
| calm | `circle`（空心） | `#8E8E93` |
| count badge | template 文本 | auto B/W |

---

## 3. Popover（NSPopover + SwiftUI 自定义视图）

点击状态项**或**全局热键唤起。

### 尺寸（pt）

| 项 | 值 |
| --- | --- |
| popover 宽 | **320** |
| 圆角 popover / row / chip | 13 / 8 / 5 |
| 行高（两行式，默认）/ 紧凑式 | 46 / 28 |
| agent 头像 | 30 |
| 列表 max-height | 360（>7 行出现滚动） |
| 内边距 / 行内 gap | 7 / 11 |

材质：vibrancy（毛玻璃）。深浅色都要支持。

### 结构（自上而下）

1. **Header**：gtmux 标记（pane 网格，见 §15）+ 「仅等待」过滤开关 + 搜索按钮；下一行是
   摘要 `5 agents · 1 waiting · 2 working · 2 idle`（本地化，见 `Summary`）。搜索模式下摘要
   行换成搜索输入框。
2. **分组列表**：分区顺序固定 **需要你（waiting）→ 运行中（working）→ 空闲（idle）→ 待命（running）**。
   每个分区一个小标题（全大写、字重 700、+0.5 字距）+ 计数；**waiting 分区标题用红色**，其余中性。
3. **Footer**：四个动作按钮 **Overview · Live watch · Restore · New session**（图标+文字）；分隔线；
   左侧齿轮 **偏好设置**，右侧版本行 `gtmux 0.1.0 · designed by ccy`。

### 行（Row）模型

行首 = **agent 头像**（识别「哪个工具」），右下角叠一个**状态徽章**（§1 的色+形+字形）。

```
[ agent 头像 30pt + 右下角状态徽章 15pt ]  session(主) · window(次)  [latest?]
                                            task（次要、灰、省略号截断）        time   ›/⏎
```

- **第一行**：`session`（加粗主体，13/590）+ `window`（次要、灰、11.5）+ 可选 `latest` 标记。
- **第二行**：`task`（12、灰、`nowrap` + 省略号；空时显示 `—`）。
- **右栏**：相对时间（mono，tabular-nums）+ 跳转记号（悬停/选中时 `›`→`⏎`）。
- **waiting 行额外加重**：淡红底 `rgba(239,68,68,0.08)` + 红色分区标题 + 红方块徽章（必要时脉冲一次）。
  其余状态保持安静（中性头像、低饱和徽章）。
- `latest` = 最近结束的那个 agent，用绿色文字 pill 标「latest / 最近完成」（**不要**再加 ✓，避免与
  idle 徽章的 ✓ 重复）。

### 交互状态

- **hover = 选中**：鼠标移到某行即把键盘选中高亮移到该行（单一高亮，命令面板式）。
- 选中行：`rgba(255,255,255,0.12)`（深）/ `rgba(0,0,0,0.07)`（浅），右侧记号变 `⏎`。
- **键盘**：`↑/↓` 移动、`⏎` 跳转（`gtmux focus <target>`）、`⎋` 退出搜索/关闭。
- 选行点击或 `⏎` → 调 `gtmux focus`（tmux 用 pane id，native 见 §7）。
- 超过 max-height 滚动；分区标题可考虑吸顶。
- 「仅等待」开关：过滤到只剩 waiting。

---

## 4. 快速切换器（全局热键，默认 ⌥⇧G）

两种形态，按需选其一或都做：

- **A · popover 搜索模式**：在 popover 内输入即模糊过滤同一列表（复用所有状态与行样式）。原地、零跳转。
- **B · 独立命令面板**（推荐做热键入口）：屏幕居中的 Raycast 式面板，大命中区、键盘优先，底部快捷键条
  （`↑↓ 选择 · ⏎ 跳转 · ⌘1–9 直达`）。

模糊匹配字段：`session / project / window / task / agent / pane`。

---

## 5. 空状态 & 首次运行

### 空状态

不报错、不留白尴尬。展示一行可复制的启动命令；文案说明**任意 coding agent**（不限 Claude）：

> 没有运行中的 agent
> 在 tmux pane 里启动任意 coding agent（Claude Code · Codex · Gemini · aider…）
> `tmux new -s work \; claude`

### 首次运行（自动化权限）

`focus` 通过 AppleScript 控制终端切到对应 tab/pane，需要一次性的 macOS「自动化」权限。用一张友好的
解释卡，而不是直接糊系统弹窗。

**文案要求：平实、就事论事，禁止营销腔**（不要「一眼看清谁在等你」「点一行直达」这类句子）。范例：

> **跳转需要「自动化」权限**
> 点击某个 agent 时，gtmux 用 AppleScript 把它所在的终端标签页和 tmux pane 切到最前。这需要一次
> 「自动化」授权，只切换窗口、不读取终端内容。
>
> 1. 点「允许并继续」，会弹出 macOS 系统对话框
> 2. 在「"gtmux" 想要控制 "Ghostty"」中点「好」
> 3. 随时可在 系统设置 › 隐私与安全性 › 自动化 撤销
>
> *不授权也能用：`agents`、`overview` 照常工作，只是不能点击跳转。*

---

## 6. Agent 身份（要区分，但别上色）

- 行首头像默认用**中性单字标**（`C` / `Cx` / `G` / `A` / `oc` / `Cr` / `Cu` / `Am`），**单色中性**，
  不抢状态色。
- **不要给 agent 上品牌色**（紫/橙等会和状态色打架、稀释 waiting 的红）。
- **真实 logo 是各家商标，不在设计里直接绘制**。系统预留接口：在 agent profile
  （`~/.config/gtmux/agents.json`，见 `internal/app/agents.go` 的 `agentProfile`）增加一个
  `icon` 字段，app 运行时按各家品牌规范加载**官方图标**（仓库 hook 已有缓存 Claude 图标的先例）。
- 可在偏好里开关「显示 agent 名」：开启时第二行前缀暗色 agent 名（`Codex · task…`）。

---

## 7. tmux 与原生终端（数据模型泛化）

gtmux 不只支持 tmux 里的 agent，也要支持**直接跑在原生终端**（无 tmux）里的 agent。

| 来源 | 主标识 | 次标识 | 跳转目标 |
| --- | --- | --- | --- |
| `tmux` | `session` | `window` | `gtmux focus <pane_id>`（`%N`） |
| `native` | `project`（cwd basename） | `terminal`（Ghostty / Terminal / iTerm2…） | 聚焦 `terminal` app 中标题为 `tab` 的标签页（AppleScript） |

- 原生终端 agent **没有 tmux session/window/pane**；行首主标识改用 `project`，次标识用 `terminal`，
  并加一个小 `native` 标记。
- `agents --json` 契约**本期落地**新增字段：`source: "tmux" | "native"`、`project`、`terminal`、
  `tab`（native 的标签标题）（tmux 项可省略 project/terminal/tab，native 项可省略 session/window/pane/loc）。
- **jump 的 native 目标 =「终端 app + 标签标题」**（`terminal` + `tab`）：用 AppleScript 在该终端里选中
  标题匹配 `tab` 的标签页并 `activate`。（已定）

---

## 8. 偏好设置（Preferences window）

标准 macOS 设置窗口栅格（标签右对齐、控件左对齐）。字段：

- **语言 Language**：`跟随系统`（默认）/ `English` / `中文`。读取 `GTMUX_LANG`，锁定后写入偏好并覆盖。
  切换**即时生效**，状态项、popover、通知全部跟随。
- **刷新间隔**：滑块，默认 1.5s。
- **开机自启**：开关。
- **状态栏显示**：`点+数字` / `仅圆点` / `空闲时隐藏`（对应 §2 三种模式）。
- **全局热键**：可录制（默认 ⌥⇧G）。
- **通知**：开关（agent 开始等你 / 完成时提醒）。
- **远程访问**：开关。开即 always-on 隧道（`gtmux tunnel --service`，常驻、重启可达）；
  开启前必须弹**确认**（长期公网敞口，token 把关）。开启后 popover footer 显示
  `🌐 远程开启` 指示 —— **敞口绝不静默**，点它回偏好可关。详见
  `docs/design/remote-access-tunnel.md`。

---

## 9. 设计 Token

### 颜色

```
# 状态色（权威，icon.go）
waiting  #EF4444   working  #06B6D4   idle  #22C55E   none/running  #8E8E93

# 深色 popover（vibrancy）
bg      rgba(28,28,31,0.60) + blur(28px) saturate(180%)
fg/2/3  rgba(255,255,255,0.95) / rgba(235,235,245,0.62) / rgba(235,235,245,0.34)
divider rgba(255,255,255,0.09)    row-selected rgba(255,255,255,0.12)

# 浅色 popover
bg      rgba(252,252,253,0.72) + blur
fg/2/3  #1d1d1f / rgba(60,60,67,0.62) / rgba(60,60,67,0.34)
divider rgba(0,0,0,0.08)          row-selected rgba(0,0,0,0.07)
```

> 背景/壁纸用**中性、低彩度**色，禁止彩虹渐变；卡片禁止彩色发光阴影。

### 字体

- UI：系统字体 `-apple-system` / **SF Pro Text/Display**；中文 **PingFang SC**。
- 代码 / pane id / loc / 命令：**SF Mono**（`ui-monospace`）。
- 字号/字重：命令面板标题 18/680 · session 13/590 · task 12/400 · 分区标题 11/700 +0.5 大写 · mono 11。

### 间距 / 圆角

8pt 栅格。见 §3 尺寸表。

---

## 10. 动效

- **唯一允许的动效**：idle→waiting 跃迁时给状态项/徽章**一次**脉冲提示。
- working 的加载环**静态**（靠形状表达「进行中」），不旋转。
- 空闲态**零动画**。hover/选中/打开可有极克制的微过渡，整体保持安静。

---

## 11. 无障碍（VoiceOver）& i18n

- 每个行 = 一个按钮。label = 「session，agent，状态，task，时间」；hint = 「跳转到该 pane / 终端」。
- **绝不**让颜色单独承载含义（已用形状+字形冗余）。
- 命中区 ≥ 44pt（菜单项可略小，但可点区域要够）。
- **i18n**：`GTMUX_LANG` en/zh；偏好语言三态。中文（CJK）更宽 → 行固定高、弹性中列、`nowrap` + 省略号，
  **绝不换行或溢出**。计数用数字，宽度稳定。

---

## 12. Logo / 品牌标记

- **App 标记 = pane 网格（方案 C）**：2×2 网格，一格高亮 **`#06B6D4` 青**，其余中性；底为深色方角图标。
  用于 popover 头部、空状态、首次运行、快速切换器、Dock/关于。
- 三色圆点（红/青/绿）降为**辅助母题**（图例、强调），不再做主 logo。
- **状态项 ≠ logo**：菜单栏里显示的是**状态字形**（§2），不是 app logo。

---

## 13. 状态与边界矩阵

| 场景 | 状态项 | popover |
| --- | --- | --- |
| 0 agent | 灰空心环 / 可隐藏 | 空状态卡 + 启动命令；不报错 |
| 1 waiting | 红方块 + 计数 + 脉冲一次 | 单行直接落在「需要你」，已选中，⏎ 即跳 |
| ~5 mixed | 取最紧急（红优先） | 三分区；waiting 高亮、idle 安静；标 latest |
| 15+ | 仅显示待办计数，不爆栏宽 | 360pt 后滚动；可「仅等待」收窄 |
| 超长 task | 不受影响 | 单行省略号截断，tooltip 给全文；session 永不被挤断 |
| CJK 中文 | 计数为数字，宽度稳定 | 固定行高 + 弹性中列 + 省略号，不换行/溢出 |
| native 终端 | 同上 | 主标识用 project、次用 terminal、带 `native` 标记 |
| idle→waiting | 绿圆 → 红方块 + 单次脉冲 | 唯一动效时刻 |

---

## 14. 数据契约

消费 `gtmux agents --json`（稳定 shape，见 `internal/menubar/model.go` 的 `Agent` 与
`internal/app/agents.go` 的 `agentJSON`）。现有字段：

```
pane_id, session, window, pane, loc, agent, status, task, latest, activity
```

为支持原生终端（§7）**本期扩展**：`source`、`project`、`terminal`、`tab`。app 轮询 ~1.5s，并 watch
`~/.local/share/gtmux/`（hook 写 `waiting/<pane>`、`last-finished`）以便即时更新。

---

## 15. 参照（borrowed from）

Tailscale（状态项克制、连接态一眼可读）· OrbStack（两行列表、温和材质）·
Stats/exelban（栏内密度、可配「显示什么」）· Raycast（命令面板、模糊搜索、底部快捷键条）·
Dato/itsycal（轻量原生贴合）· CCMenu（状态+列表+跳转成熟范式）。
