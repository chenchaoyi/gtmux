# gtmux — 设计落地总交接（Claude Code Handover）

> 把本文件**整段**贴给仓库里的 Claude Code 作为任务起点，让它把 `docs/design/` 下的全部设计
> 落地到代码。设计权威：菜单栏看 `docs/design/DESIGN.md`，移动端看 `docs/design/MOBILE.md`；
> 可视化参照在 `docs/design/mockup/`。本文件是**索引 + 顺序 + 验收 + 约束**，细节以两份规范为准。

---

## 0. 你要做什么

gtmux 是 **coding agent 的指挥中心**，一个产品、三块屏，共用同一套**状态语言**：

1. **CLI**（已存在）—— `gtmux`，真相源：`agents --json` / `serve` / `focus`。
2. **menu-bar app**（macOS，本次落地）—— `NSStatusItem + NSPopover + SwiftUI` 自定义视图。
3. **mobile app**（iOS，本次落地）—— bare React Native，桌面的远程伴侣（只读 MVP）。

你的任务：把 2 和 3 按规范实现/迭代到位，并保持与 CLI 的契约一致。

---

## 1. 先读这些（按序）

**通用 / 真相源**
- `docs/design/DESIGN.md` §0–§3 —— 设计原则 + **状态语言（色+形+字形）**，三块屏共用。
- `internal/menubar/icon.go` —— 状态色权威值 `#EF4444 / #06B6D4 / #22C55E / #8E8E93`。
- `internal/app/agents.go` —— agent 探测、`agentJSON`、状态分类与排序。
- `internal/menubar/model.go` —— `agents --json` 契约 + 纯函数（title / summary / rows）。

**菜单栏**
- `docs/design/DESIGN.md`（全文，权威）。
- `docs/design/mockup/gtmux-menubar.dc.html`（可交互原型，浏览器打开需联网）+ `preview-*.png`。
- `cmd/gtmux-menubar/`（现状 cgo systray 入口）、`README.md` 的「menu-bar app」节。

**移动端**
- `docs/design/MOBILE.md`（权威设计补充：App 图标 / Agent 图标 / Radar·Detail 交互 / 推送 / 状态）。
- `mobileapp/SPEC.md`（工程蓝图）、`api/contract.md`（HTTP/SSE `v0` 契约）。
- `mobileapp/src/ui/theme.ts`·`StatusBadge.tsx`·`AgentRow.tsx`（token 与组件，权威）。
- `docs/design/mockup/gtmux-mobile.dc.html`（可交互原型，四屏 + 推送 + 图标 + 折叠 + 彩色终端）。
- `relay/`（push relay → APNs）。

---

## 2. 共用不变量（两块屏都必须遵守）

- **状态三重编码**：waiting `#EF4444` 红**方块·双竖线** / working `#06B6D4` 青**圆·加载环（静态不转）** /
  idle `#22C55E` 绿**圆·对勾** / running `#8E8E93` 灰**圆·小圆点**。**颜色只表达状态**。
- **层级**：waiting 响、idle 静；分区顺序 **needs-you → working → idle → running**，waiting 分区标题红。
- **agent 身份**用中性单字标（CC/Cx/G/Ai/oc/Cu/Cr/Am…），真机加载官方图标走 `Agent.icon`/profile 的
  `icon` 字段；**绝不在代码里绘制第三方商标 logo**。
- **i18n**：en/zh，跟随 `GTMUX_LANG` / 设备语言，设置可锁定三态（跟随系统/EN/中文），即时生效；
  CJK 不换行、用省略号；命中区 ≥ 44pt。
- **动效**：只允许 idle→waiting 一次脉冲；加载环静态；空闲零动画。
- **视觉克制**：无彩虹渐变、无彩色发光阴影；文案平实、**禁止营销腔**（尤其权限/首次运行卡）。
- **VoiceOver / a11y**：每行一个按钮，label=「主标识，agent，状态，task，时间」，hint=「跳转」；绝不只靠颜色。
- **品牌**：app 标记 = **pane 网格**（右上格青色高亮）；三色圆点降为辅助母题。命令/wordmark/文档一律小写 `gtmux`。

---

## 3. Surface A — 菜单栏 app

**形态**：`NSStatusItem + NSPopover + SwiftUI`（**不是** NSMenu）。是 `agents --json`（~1.5s 轮询 +
watch `~/.local/share/gtmux/`）的纯消费方，点击行 shell 出 `gtmux focus <target>`。

实现顺序：
1. **状态项**（最重要，DESIGN §2）：shape-shift 字形 + 三模式（dot / dot+count / hide-when-idle），
   浅/深/着色菜单栏、刘海右侧、宽度极小。优先 SF Symbols + tint，回退 @1x/@2x PNG。
2. **Popover**（DESIGN §3）：尺寸、分组、行模型（agent 头像 + 角标状态徽章、session 主/window 次、
   task 截断、相对时间、跳转记号）、hover=选中、键盘 ↑↓⏎⎋、滚动、footer。
3. **状态徽章字形**（DESIGN §1）。
4. **空状态 + 首次运行**（自动化权限卡，DESIGN §5，平实文案）。
5. **快速切换器**（全局热键，DESIGN §4）。
6. **偏好设置**（DESIGN §8，语言三态即时生效）。
7. **原生终端支持**（DESIGN §7，本期落地）：`agents --json` 扩展 `source/project/terminal/tab`，
   行与跳转按来源泛化。

---

## 4. Surface B — 移动端 app

**形态**：bare React Native，经 VPN/Tailscale 连 `gtmux serve`，**只读 MVP**（监控 + focus + 推送）。
四屏：配对 → 雷达 → 详情 → 设置。数据契约见 `api/contract.md`。

实现顺序：
1. **token 对齐**（`theme.ts`）+ **StatusBadge**（色+形+字形，与菜单栏同款）。
2. **Radar**（MOBILE §3）：分组列表（needs-you→working→idle→running）；**可发现折叠**（计数气泡 +
   Hide/Show 文字 + 圆形旋转箭头 + 按压高亮）；**分区分隔槽**（间隙 + 顶部粗线）；只看等输入 / 下拉刷新 /
   连接状态点（live/reconnecting/offline）；初始 `GET /api/agents` + `GET /api/events`(SSE) 重拉。
3. **行头像**（MOBILE §2）：**圆角方块（radius 9, overflow:hidden）**，`Agent.icon` 渲染 `<Image>`，
   缺省回退中性字标；右下角 16pt 状态徽章。
4. **Detail**（MOBILE §4）：`GET /api/pane` 用 **`tmux capture-pane -e -p`** 出 ANSI 颜色，RN 端 SGR
   解析 → 彩色 `<Text>`（色板对齐 §4）；窄屏适配（换行/滚动、A−/A+、cols×rows·live、回滚缓冲 + ↓ FAB）；
   Focus on Mac 在顶栏（`POST /api/focus`）。
5. **Composer（Phase 2，写入门控）**：agent 化上下文键（waiting=1/2/3、其它=继续/⏎/停止）+ 控制键排 +
   自由输入；语音输入（聆听态 + 波形 + 转写）；全部走 `POST /api/send`（send-keys），未授权置灰并标注。
6. **空状态 + 首次配对**（MOBILE §1 末 / §6）：`Connect your Mac` + 副标题；扫码（QR schema v1）或手动
   host:port + token；`health()` 自检 → 存 Keychain → 进雷达。
7. **设置**：语言三态、已配对 Mac（可移除）、推送开关 + 说明、版本。
8. **推送**（MOBILE §6）：`serve → relay → APNs`，原生 token 经 `POST /api/push/register`；`alert` 两类
   waiting/done；点推送深链直达该 pane 的 Detail；前台改应用内 banner。
9. **App 图标**（MOBILE §1）：pane 网格，iOS 18 四变体，全尺寸导出。
10. **iPad / 自适应**（MOBILE §5）：`width ≥ 768` 走 **split-view**（侧栏雷达 + 主区详情，点行即时切）；
    窄屏回退抽屉 / 单列。同一套 RN 组件按 `useWindowDimensions` 断点切换，导航层加 `regular/compact` 分流；
    支持硬件键盘（↑↓ / ⏎ / ⌘1–9 / ⌘F）与横竖旋转。

---

## 5. 验收标准

**菜单栏**
- 0 / 1 / ~5 / 15+、超长 task、CJK、native 终端、waiting↔calm 都符合 DESIGN §13。
- waiting 在浅/深/着色菜单栏靠形状+字形可辨（非仅色）。en/zh 即时生效不破行。
- 点击/⏎ 正确 `gtmux focus`：tmux 用 pane id；native 聚焦 `terminal` app 中标题匹配 `tab` 的标签页。

**移动端**
- 0 / 1 / ~5 / 15+、超长 task、CJK、离线/重连、idle→waiting 都符合 MOBILE §7。
- 状态徽章与菜单栏视觉一致；折叠可发现、分区分隔清晰。
- Detail 终端彩色且窄屏可读（换行/字号/回滚/跳底）。
- 推送离开 VPN 仍达；点推送深链到对应 pane。
- 真机 agent 头像优先显示官方图标，缺省回退字标；颜色不用于区分 agent。
- **iPad**：`width ≥ 768` 双栏并置、点侧栏行主区即时切；竖屏/窄屏回退抽屉或单列；横竖旋转不锁定；硬件键盘可用。

---

## 6. 已定（产品确认，直接做）

- **native 终端跳转目标 =「终端 app + 标签标题」**（`terminal` + `tab`）：AppleScript 选中标题匹配的 tab 并 activate。
- **本期落地** `agents --json` 的 `source/project/terminal/tab` 扩展（不再只留接口）。
- **App 命名**：全小写 `gtmux`（命令/wordmark/文档统一）。
- **移动端 MVP 只读**；终端输入（`POST /api/send`）属 Phase 2，UI 先就位、置灰门控。

---

## 7. 先问再做（需要时找人确认）

- `POST /api/send` 的**写权限门控**机制（一次性授权？serve 侧开关？）—— 落地输入前先在 `api/contract.md`
  定稿；本次先按「UI 就位 + 置灰 + Phase 2 标注」交付。
- 移动端官方 agent 图标的**来源**：app 内置常见工具图标集，还是从 `Agent.icon`（`.app` 路径）运行时解析。
- relay 的部署形态（自托管 vs 托管）与 APNs key 管理。

---

## 8. 持续约束（务必执行）

把 `docs/design/CLAUDE.snippet.md` 合并进仓库根 `CLAUDE.md`，让后续每次会话都遵守本套设计规范。
任何与 `DESIGN.md` / `MOBILE.md` 冲突的 UI 改动，**先提出、不要擅自偏离**；改了设计就同步更新这两份规范。
