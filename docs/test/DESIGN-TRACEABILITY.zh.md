# 设计跟随矩阵

[English](DESIGN-TRACEABILITY.md) · [当前测试分层](TEST-PLAN.zh.md)

这里保留历史设计要求与未验边界；旧技术栈分层已改为 L1/L2 行为测试、G 规则、M 人工验收。具体用例见 [用例目录](CASES.zh.md)，不得把下表中的入口当作本次实际通过。

> 把 `docs/design/DESIGN.md` 每个小节映射到「实现 / 自动化测试 / 人工验收 / 状态」。
> 这是「设计是否被完整 follow」的 review 凭证。**每次迭代更新本表**。
> 状态：✅ 完成 · 🟡 部分 · ⏳ 待办。

本表记录实现与测试入口，不代替带版本、设备和结果的验收记录。DESIGN 后续新增及重号章节的映射尚未全部补齐，见表后清单；不能据此认定设计已全量跟随。

CI 覆盖入口见 [PR 检查](../ci.zh.md)：规范与文档每 PR 检查，平台按影响范围选择；
Web 的 `tests/web/` 验证实际渲染与窄屏控件，iOS 无签名构建验证 App/扩展编译。
两者均不更新本表中的真机视觉或无障碍验收状态。

| DESIGN 小节 | 实现位置 | 自动化测试 (L1/L2/G) | 人工验收 (M) | 状态 |
| --- | --- | --- | --- | --- |
| §0 设计原则（安静/层级/三重编码/克制/动效最小） | 全局 | G 架构不变量 | 整体观感、克制度 | 🟡 |
| §1 状态模型（色+形+字形） | `AgentStore.Status`,`Theme.Status`,`StatusBadge`,`StatusItemGlyph` | L1/L2 `testStatusColorsMatchDesignHex`/`testStatusRankOrder`/`testEveryStatusHasColor`；G 调色板 | 徽章三重编码一致性 | ✅ 逻辑 / 🟡 视觉 |
| §2 状态项（品牌网格与非颜色状态符号） | `StatusItemGlyph` 保留 pane 网格，下格用暂停/环/对勾镂空并着色；`AppDelegate.renderIcon` 控制计数与 3 模式 | G 调色板 | 网格、浅/深/**着色**菜单栏、3 模式、刘海 | 🟡 实现可查；当前设备视觉仍待验收；`StatusItemGlyphTests` 已提供真实渲染的非 RGB 几何检查，仍不代表壁纸验收 |
| §3 Popover（尺寸/分组/行/交互/footer） | `MenuView`,`Components`,`Theme.Size` | L1/L2 `sections*`/`testFuzzySearch`/`testRelativeTime` | 布局/材质/键盘/滚动对照 mockup | 🟡 |
| §4 快速切换器（热键） | A: popover 搜索；B: `CommandPalette.swift` 独立命令面板（⌘⌥G 唤起，⌘1–9 直达）；`GlobalHotkey` | L1/L2 `testFuzzySearch`/`testPaletteWrapNavigation` | 热键唤起、搜索、⏎/⌘1–9 跳转、视觉对照 mockup §4 B | ✅ 逻辑；视觉仍需真机验收 |
| §5 空状态 & 首次运行 | `States.swift`（Empty/FirstRun） | — | 文案平实无营销腔；权限卡 | 🟡（视图就绪；首次运行**触发时机/权限探测未接线** ⏳） |
| §6 Agent 身份（图标优先、无图标回退字标） | `AgentIcons`,`AgentAvatar`,`agentMonogram`；Go `IconFor`/`BuiltinIconPath` 提供 profile 或内置图标路径 | L1 `TestBuiltinIconHintIsAnOpenablePath`；L1/L2 `testDecodeIconField`/`testAgentIconsNilWhenUnavailable`/`testAgentMonogram` | `.app`/图片路径、内置图标与缺失回退；不抢状态色 | ✅ 加载路径已实现 / 🟡 视觉待验收 |
| §7 tmux 与原生终端（数据泛化 + native 跳转） | `internal/native/native.go` 由 hook 感知；`internal/radar/agents.go` 输出 `source:native`；原生跳转仍未开放 | L1 native/radar 契约；L1/L2 原生行解码 | native 行出现、结束后消失；原生跳转暂不验收 | 🟡 感知已实现；无 pane 的原生会话仅可查看，不能跳转/发送 |
| §8 偏好设置 | `Preferences.swift`,`AppSettings` | —（UI） | 语言三态即时、间隔、自启、显示模式、通知 | 🟡 可录制热键未实现；当前固定显示 ⌘⌥G |
| §9 设计 Token（颜色/字体/间距） | `Theme.swift` | L1/L2/G 颜色 hex 一致 ✅ | 字体/间距/材质对照 | ✅ 颜色 / 🟡 其余 |
| §10 动效（行内 working 环 2 秒旋转，界面关闭或 Reduce Motion 时静止） | `StatusBadge`/`MenuVisibility`；`StatusItemGlyph` 网格不旋转，服务器模式呼吸点另见 §17 | L1/L2 `SurfaceTeardownTests.swift` 的 `MenuVisibilityTests` 检查可见性 flag；未覆盖旋转视图的替换或 CPU | 开关界面与 Reduce Motion；**idle→waiting 单次脉冲 ⏳ 未实现** | 🟡 旋转分支已实现；动态验收另记 |
| §11 无障碍 & i18n | `L10n`（en/zh）；agent 行由 `onTapGesture` 触发跳转，并非整行 Button | L1/L2 解码/分组（i18n 文案随 L10n） | agent 行未显式设置 VoiceOver label/hint；部分 HQ 控件已有 label。VoiceOver 与 CJK 需实际验收 | 🟡 |
| §12 Logo（pane 网格） | `GtmuxLogo` | — | 头部/空状态/首次运行一致 | ✅ |
| §13 状态与边界矩阵 | 全局 | L1/L2 计数/分组覆盖一部分 | 0/1/~5/15+/超长/CJK/native/切换 | 🟡（人工矩阵为主） |
| 数据契约（非现行 DESIGN §14） | `agentJSON`,`Agent` 解码 | L1 契约 + L1/L2 解码 | — | ✅ 已有逻辑测试；历史两个 §14 已改为独立标题 |
| §15 参照 | — | — | — | n/a |

### 尚待补齐的章节映射

DESIGN 已移除重号，现行共用规则使用稳定 ID；下列条目尚未在本表完成逐项实现、自动测试与人工验收的对应，不表示功能未实现：

- §9 下两段带日期的行为规则：A row you can't jump to has to say so first（2026-08-16）；A slow action must speak for itself, and needs a concurrency floor（2026-08-17）。
- ROLE-HQ、PREF-SHARING。
- §14 Footer v3、历史 mockup 对应的 Popover action layout。
- §16 Pane browser、ICON-SIZE 图标尺寸。
- §17 Server mode、§18 Screenshot to an agent。

### HQ 导航与可操作性

| 设计要求 | 实现 / 回归入口 | 验收边界 |
| --- | --- | --- |
| 旧记录读取期间切换事项不等待轮询、不显示旧事项 | `HQWorkspaceStore.loadEarlier`；`HQWorkspaceTests.testSwitchingDuringEarlierReadImmediatelyLoadsTheNewMatter` | 合成客户端的并发回归；实际窗口与真实 HQ 另验 |
| HQ 复制按钮只显示图标，保留完整复制与失败反馈 | `HQChatBubble`；`HQChatPresentationTests.testCopyPreservesFullMarkdownAndFeedbackResetsAndReportsFailure` | 剪贴板回归使用注入写入器；实体桌面剪贴板另验 |
| 资源入口、页签、卡片选择和子操作可独立识别 | `HQHeader`、`HQScreen`、`KnowledgeSheet`；`HQHeader.test.tsx`、`HQScreen.poll.test.tsx` | 角色与状态检查不代表 VoiceOver/TalkBack 播报已验收 |
| 搜索结果第一次点击不被软件键盘吞掉 | `KnowledgeSheet` 单列与分栏滚动容器；`e2e/__tests__/knowledge-search.test.ts` | 需带软件键盘运行 iPhone/iPad UI 测试；不是仅组件渲染断言 |
| iPad 主区保留对话阅读宽度 | `ui/layout.hqInspectorFits`、`SplitShell`、`HQView.showInspector`；`layout.test.ts`、`HQScreen.poll.test.tsx` | 转屏、侧栏切换和草稿保留另做运行验收 |
| HQ 键盘不被浮动头部遮挡 | 复用 `useKeyboardChrome`，暂收次要输入行；`HQScreen.poll.test.tsx`、`hq-keyboard.test.ts` | 软件键盘、手机横屏和大字号运行验收；Android 与实体硬件键盘另验 |

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

## 设计一致性修复（2026-11-17）

评审设计 `designs/design-consistency-20261117.json` 先于新增测试提交。现行规则使用
TEXT-READ / TOUCH-44 / MOTION-LOAD / ROLE-HQ / A11Y-CONTEXT / UPLOAD-PHASE /
STATE-LABEL / FILE-SCOPE；组件×平台×状态×主题矩阵见 SURFACES。
DesignConsistency / SettingsAppearance 执行真实 App 组件；Web 测试执行真实 XHR
阶段与角色分组；Mac 测试渲染 NSImage 并比较非 RGB 几何。这些覆盖不冒充原生布局、
材质或设备实际朗读验收。商店素材与文案单列冻结候选及 Apple 审核状态，源码合入不等于素材已公开展示。
