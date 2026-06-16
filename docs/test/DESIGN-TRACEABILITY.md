# 设计跟随矩阵（Design Traceability）

> 把 `docs/design/DESIGN.md` 每个小节映射到「实现 / 自动化测试 / 人工验收 / 状态」。
> 这是「设计是否被完整 follow」的 review 凭证。**每次迭代更新本表**。
> 状态：✅ 完成 · 🟡 部分 · ⏳ 待办。

| DESIGN 小节 | 实现位置 | 自动化测试 (L1/L2/L3) | 人工验收 (L4) | 状态 |
| --- | --- | --- | --- | --- |
| §0 设计原则（安静/层级/三重编码/克制/动效最小） | 全局 | L3 架构不变量 | 整体观感、克制度 | 🟡 |
| §1 状态模型（色+形+字形） | `AgentStore.Status`,`Theme.Status`,`StatusBadge`,`StatusItemGlyph` | L2 `testStatusColorsMatchDesignHex`/`testStatusRankOrder`/`testEveryStatusHasColor`；L3 调色板 | 徽章三重编码一致性 | ✅ 逻辑 / 🟡 视觉 |
| §2 状态项（shape-shift + 3 模式 + 着色适配） | `StatusItemGlyph`,`AppDelegate.renderIcon` | L3 调色板 | shape-shift、浅/深/**着色**菜单栏、3 模式、刘海 | 🟡（已渲染验证运行；视觉待验收） |
| §3 Popover（尺寸/分组/行/交互/footer） | `MenuView`,`Components`,`Theme.Size` | L2 `sections*`/`testWaitingOnlyFilter`/`testFuzzySearch`/`testRelativeTime` | 布局/材质/键盘/滚动对照 mockup | 🟡 |
| §4 快速切换器（热键） | A: popover 搜索；**B: `CommandPalette.swift` 独立命令面板**（⌘⌥G 唤起，⌘1–9 直达）；`GlobalHotkey` | L2 `testFuzzySearch`/`testPaletteWrapNavigation` | 热键唤起面板、搜索、⏎/⌘1–9 跳转 | ✅（A+B 完成。**默认热键 ⌘⌥G —— 产品确认，覆盖 DESIGN §4 的 ⌥⇧G；⌘⌥G 开命令面板，点状态项开 popover**） |
| §5 空状态 & 首次运行 | `States.swift`（Empty/FirstRun） | — | 文案平实无营销腔；权限卡 | 🟡（视图就绪；首次运行**触发时机/权限探测未接线** ⏳） |
| §6 Agent 身份（中性单字标、不画 logo） | `agentMonogram`,`AgentAvatar` | L2 `testAgentMonogram`；L3「不自探测」 | 头像中性、不抢状态色 | 🟡（profile `icon` 官方图标字段 ⏳） |
| §7 tmux 与原生终端（数据泛化 + native 跳转） | `agentJSON`+`Agent`(source/project/terminal/tab/activity_at)；`focus --terminal/--tab`；`ghostty.FocusTerminalTab` | L1 `TestAgentJSONContractFields`/`TestGhosttyTabScript`；L2 `testDecodeNativeAgent`/jumpArgs | native 行渲染、native 跳转真机 | 🟡（schema/渲染/跳转 ✅；**native 探测 scanner ⏳ — 需 ps/cwd/终端 tab 标题，建议随 Terminal 驱动一起做**） |
| §8 偏好设置 | `Preferences.swift`,`AppSettings` | —（UI） | 语言三态即时、间隔、自启、显示模式、通知 | 🟡（**可录制热键 ⏳，当前静态显示 ⌥⇧G**） |
| §9 设计 Token（颜色/字体/间距） | `Theme.swift` | L2/L3 颜色 hex 一致 ✅ | 字体/间距/材质对照 | ✅ 颜色 / 🟡 其余 |
| §10 动效（仅 idle→waiting 脉冲；环不转） | `StatusItemGlyph`(环静态),`StatusBadge` | — | **idle→waiting 单次脉冲 ⏳ 未实现**；其余零动画 | 🟡 |
| §11 无障碍 & i18n | `L10n`（en/zh）；行=按钮 | L2 解码/分组（i18n 文案随 L10n） | **VoiceOver label/hint ⏳ 未显式设置**；CJK 不破行 ✅ | 🟡 |
| §12 Logo（pane 网格） | `GtmuxLogo` | — | 头部/空状态/首次运行一致 | ✅ |
| §13 状态与边界矩阵 | 全局 | L2 计数/分组覆盖一部分 | 0/1/~5/15+/超长/CJK/native/切换 | 🟡（人工矩阵为主） |
| §14 数据契约 | `agentJSON`,`Agent` 解码 | L1 契约 + L2 解码 ✅ | — | ✅ |
| §15 参照 | — | — | — | n/a |

## 本期已知缺口（待后续迭代）

1. **native 探测 scanner**（§7）：契约/渲染/跳转已落地，但「发现非 tmux 里的 agent」未实现（需进程扫描 + cwd +
   终端 tab 标题映射，且状态判定需读终端 tab 标题）。建议与 `Terminal` 驱动抽象一起做，并真机验证。
2. **idle→waiting 单次脉冲**（§10）：唯一允许的动效，尚未实现。
3. **可录制全局热键**（§8）：目前固定 ⌥⇧G 并静态展示。
4. **VoiceOver label/hint**（§11）：行已是按钮，但未显式设置无障碍标签。
5. **首次运行权限卡触发**（§5）：视图就绪，未接「首次点击跳转时检测自动化权限并弹卡」。
6. **agent 官方图标 `icon` 字段**（§6）：预留，未加载官方图标。
