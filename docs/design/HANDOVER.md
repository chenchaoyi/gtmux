# Claude Code 交接 Prompt — gtmux 菜单栏 app

> 把下面整段贴给仓库里的 Claude Code 作为任务起点。设计权威是
> `docs/design/DESIGN.md`；可视化参照在 `docs/design/mockup/`。

---

## 贴给 Claude Code 的 Prompt

你将为 **gtmux** 实现/迭代 **macOS 菜单栏 app**。先读这些，再动手：

1. `docs/design/DESIGN.md` —— **权威设计规范**，所有 UI 决策以它为准（token、状态语言、尺寸、交互、文案口吻）。
2. `docs/design/mockup/gtmux-menubar.dc.html` —— 可交互高保真原型（浏览器打开，需联网加载运行时）。
   看不了就看 `docs/design/mockup/preview-*.png` 静态截图。
3. 现有代码：
   - `internal/menubar/model.go` —— 菜单栏消费的 `agents --json` 契约 + 纯函数（title / summary / rows / 颜色）。
   - `internal/menubar/icon.go` —— 状态色权威值（`#EF4444 / #06B6D4 / #22C55E / #8E8E93`）+ dot 渲染。
   - `internal/app/agents.go` —— agent 探测、`agentJSON` 输出、状态分类与排序。
   - `cmd/gtmux-menubar/` —— 现状的 cgo systray 入口。
   - `README.md` 的「menu-bar app」一节。

**目标形态**：`NSStatusItem + NSPopover + SwiftUI` 自定义 popover 视图（**不是** NSMenu）。它是
`gtmux agents --json`（~1.5s 轮询 + watch `~/.local/share/gtmux/`）的**纯消费方**，点击行 shell 出
`gtmux focus <target>`。

### 实现顺序（建议）

1. **状态项**（最重要）：按 DESIGN §2 实现 shape-shift 字形 + 三种模式（dot / dot+count / hide-when-idle），
   支持浅/深/着色菜单栏，刘海右侧、宽度极小。优先 SF Symbols + tint，回退 PNG。
2. **Popover**：DESIGN §3 的尺寸、分组（needs-you→working→idle→running）、行模型（agent 头像 + 角标状态徽章、
   session 主 / window 次、task 截断、相对时间、跳转记号）、hover=选中、键盘 ↑↓⏎⎋、滚动、footer。
3. **状态徽章字形**：waiting=双竖线方块 / working=静态加载环 / idle=✓ / running=点，色+形+字形三重编码。
4. **空状态 + 首次运行**（自动化权限解释卡，文案用 DESIGN §5 的平实口吻，禁止营销腔）。
5. **快速切换器**（全局热键，DESIGN §4）。
6. **偏好设置**（DESIGN §8，含语言三态 跟随系统/EN/中文，即时生效）。
7. **原生终端支持**（DESIGN §7，**本期落地**）：扩展 `agents --json` 的 `source/project/terminal/tab` 字段，
   行与跳转按来源泛化；native 跳转 = 聚焦 `terminal` app 中标题为 `tab` 的标签页。

### 必须遵守的约束

- 颜色只表达**状态**；agent 类型用中性单字标，官方图标走 profile 的 `icon` 字段（DESIGN §6）。不要在代码里
  绘制任何第三方商标 logo。
- 状态色用 `icon.go` 的权威值，别另造。
- 双语（en/zh，跟随 `GTMUX_LANG`）；CJK 行不换行、用省略号；命中区够大。
- 动效：只允许 idle→waiting 一次脉冲；加载环静态不旋转；空闲零动画（DESIGN §10）。
- 视觉克制：无彩虹渐变、无彩色发光阴影、文案不营销（DESIGN §0/§5）。
- VoiceOver：每行一个按钮，label/hint 见 DESIGN §11；绝不只靠颜色。

### 验收标准

- 0 / 1 / ~5 / 15+ agent，以及超长 task、CJK、native 终端、waiting↔calm 切换都符合 DESIGN §13 矩阵。
- waiting 在浅/深/着色菜单栏上靠形状+字形可辨（非仅颜色）。
- en/zh 切换即时生效、行不破。
- 点击行/⏎ 正确 `gtmux focus`，tmux 用 pane id、native 聚焦 `terminal` app 中标题匹配 `tab` 的标签页。

### 已定（产品确认）

- **native 终端跳转目标 =「终端 app + 标签标题」**（`terminal` + `tab`）：用 AppleScript 选中标题匹配的 tab 并 activate。
- **本期落地** `agents --json` 的 `source/project/terminal/tab` 扩展（不再只留接口）。

### 持续约束

把 `docs/design/CLAUDE.snippet.md` 的内容合并进仓库根的 `CLAUDE.md`，让后续每次会话都遵守本设计规范。
任何与 `docs/design/DESIGN.md` 冲突的 UI 改动，先提出、不要擅自偏离。
