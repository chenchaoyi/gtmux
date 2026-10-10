# 设计跟随矩阵（Design Traceability）

> 把 `docs/design/DESIGN.md` 每个小节映射到「实现 / 自动化测试 / 人工验收 / 状态」。
> 这是「设计是否被完整 follow」的 review 凭证。**每次迭代更新本表**。
> 状态：✅ 完成 · 🟡 部分 · ⏳ 待办。

本表记录实现与测试入口，不代替带版本、设备和结果的验收记录。DESIGN 后续新增及重号章节的映射尚未全部补齐，见表后清单；不能据此认定设计已全量跟随。

| DESIGN 小节 | 实现位置 | 自动化测试 (L1/L2/L3) | 人工验收 (L4) | 状态 |
| --- | --- | --- | --- | --- |
| §0 设计原则（安静/层级/三重编码/克制/动效最小） | 全局 | L3 架构不变量 | 整体观感、克制度 | 🟡 |
| §1 状态模型（色+形+字形） | `AgentStore.Status`,`Theme.Status`,`StatusBadge`,`StatusItemGlyph` | L2 `testStatusColorsMatchDesignHex`/`testStatusRankOrder`/`testEveryStatusHasColor`；L3 调色板 | 徽章三重编码一致性 | ✅ 逻辑 / 🟡 视觉 |
| §2 状态项（当前 pane 网格与非颜色编码要求有偏离） | `StatusItemGlyph` 始终画 pane 网格、整体按状态着色；`AppDelegate.renderIcon` 控制计数与 3 模式 | L3 调色板 | 网格、浅/深/**着色**菜单栏、3 模式、刘海 | 🟡 实现可查；当前设备视觉仍待验收；仅靠状态色的网格未满足 §2 的非颜色编码要求 |
| §3 Popover（尺寸/分组/行/交互/footer） | `MenuView`,`Components`,`Theme.Size` | L2 `sections*`/`testFuzzySearch`/`testRelativeTime` | 布局/材质/键盘/滚动对照 mockup | 🟡 |
| §4 快速切换器（热键） | A: popover 搜索；B: `CommandPalette.swift` 独立命令面板（⌘⌥G 唤起，⌘1–9 直达）；`GlobalHotkey` | L2 `testFuzzySearch`/`testPaletteWrapNavigation` | 热键唤起、搜索、⏎/⌘1–9 跳转、视觉对照 mockup §4 B | ✅ 逻辑；视觉仍需真机验收 |
| §5 空状态 & 首次运行 | `States.swift`（Empty/FirstRun） | — | 文案平实无营销腔；权限卡 | 🟡（视图就绪；首次运行**触发时机/权限探测未接线** ⏳） |
| §6 Agent 身份（图标优先、无图标回退字标） | `AgentIcons`,`AgentAvatar`,`agentMonogram`；Go `IconFor`/`BuiltinIconPath` 提供 profile 或内置图标路径 | L1 `TestBuiltinIconHintIsAnOpenablePath`；L2 `testDecodeIconField`/`testAgentIconsNilWhenUnavailable`/`testAgentMonogram` | `.app`/图片路径、内置图标与缺失回退；不抢状态色 | ✅ 加载路径已实现 / 🟡 视觉待验收 |
| §7 tmux 与原生终端（数据泛化 + native 跳转） | `internal/native/native.go` 由 hook 感知；`internal/radar/agents.go` 输出 `source:native`；原生跳转仍未开放 | L1 native/radar 契约；L2 原生行解码 | native 行出现、结束后消失；原生跳转暂不验收 | 🟡 感知已实现；无 pane 的原生会话仅可查看，不能跳转/发送 |
| §8 偏好设置 | `Preferences.swift`,`AppSettings` | —（UI） | 语言三态即时、间隔、自启、显示模式、通知 | 🟡 可录制热键未实现；当前固定显示 ⌘⌥G |
| §9 设计 Token（颜色/字体/间距） | `Theme.swift` | L2/L3 颜色 hex 一致 ✅ | 字体/间距/材质对照 | ✅ 颜色 / 🟡 其余 |
| §10 动效（行内 working 环 2 秒旋转，界面关闭或 Reduce Motion 时静止） | `StatusBadge`/`MenuVisibility`；`StatusItemGlyph` 网格不旋转，服务器模式呼吸点另见 §17 | L2 `SurfaceTeardownTests.swift` 的 `MenuVisibilityTests` 检查可见性 flag；未覆盖旋转视图的替换或 CPU | 开关界面与 Reduce Motion；**idle→waiting 单次脉冲 ⏳ 未实现** | 🟡 旋转分支已实现；动态验收另记 |
| §11 无障碍 & i18n | `L10n`（en/zh）；agent 行由 `onTapGesture` 触发跳转，并非整行 Button | L2 解码/分组（i18n 文案随 L10n） | agent 行未显式设置 VoiceOver label/hint；部分 HQ 控件已有 label。VoiceOver 与 CJK 需实际验收 | 🟡 |
| §12 Logo（pane 网格） | `GtmuxLogo` | — | 头部/空状态/首次运行一致 | ✅ |
| §13 状态与边界矩阵 | 全局 | L2 计数/分组覆盖一部分 | 0/1/~5/15+/超长/CJK/native/切换 | 🟡（人工矩阵为主） |
| 数据契约（非现行 DESIGN §14） | `agentJSON`,`Agent` 解码 | L1 契约 + L2 解码 | — | ✅ 已有逻辑测试；两个 §14 分别描述 Footer v3 和 Popover action re-layout |
| §15 参照 | — | — | — | n/a |

### 尚待补齐的章节映射

按标题区分 DESIGN 的重复编号；下列条目尚未在本表完成逐项实现、自动测试与人工验收的对应，不表示功能未实现：

- §9 下两段带日期的行为规则：A row you can't jump to has to say so first（2026-08-16）；A slow action must speak for itself, and needs a concurrency floor（2026-08-17）。
- §12 HQ、§13 Preferences: Anywhere + Sharing。
- 两个 §14：Footer v3、Popover action re-layout。
- 两个 §16：Pane browser、Icon sizes。
- §17 Server mode、§18 Screenshot to an agent。

## 范围决定 & 本期已知缺口

0. **范围**：tmux pane 可查看、跳转和输入；非 tmux agent 通过 hook 做感知，显示为 native 行，
   目前没有对应 pane，不能跳转或发送。手机、iPad、Web 都消费 Go 核心的数据，不自行猜状态。
1. **idle→waiting 单次脉冲**（§10）：尚未实现；行内 working 环旋转已实现，不能再以「其余零动画」验收。
3. **可录制全局热键**（§8）：目前固定 ⌘⌥G 并静态展示。
4. **VoiceOver label/hint**（§11）：agent 行使用点击手势，未显式设置整行无障碍标签；不将部分 HQ 控件的 label 当作全列表已覆盖。
5. **首次运行权限卡触发**（§5）：视图就绪，未接「首次点击跳转时检测自动化权限并弹卡」。
6. **agent 图标**（§6）：`icon` 解码、`.app`/图片加载、内置图标路径及字标回退已实现；各来源的视觉仍需验收。

### 手机底部提示与真机验收（2026-10-10）

`RunningRow`、`SendFailedBar`、`DetailScreen.busyNote` 对应 MOBILE「底部状态提示」；`footerSpacing.test.tsx` 锁定外部间距、内部留白、行高和关闭/导航行为。`UsageSheet`、`KnowledgeSheet` 的关闭与搜索标签由对应组件测试覆盖。真机工具与兼容性记录见 `mobileapp/e2e/REAL-DEVICE{,.zh}.md`；本次源代码修复尚未安装，不计为真机视觉通过。
