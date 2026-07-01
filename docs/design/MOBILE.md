# gtmux 移动端 — 设计补充（App 图标 · Agent 图标 · 视觉规范）

> 本文件是移动端的**设计层补充**，与已有的工程蓝图配合使用：
> - `mobileapp/SPEC.md` —— 构建蓝图（栈、屏幕、依赖）。
> - `api/contract.md` —— HTTP/SSE `v0` 契约。
> - `mobileapp/src/ui/theme.ts` · `StatusBadge.tsx` —— token 与状态徽章（权威）。
> - `docs/design/DESIGN.md` §0–§3 —— 状态语言（三块屏共用）。
>
> 可视参照：`docs/design/mockup/gtmux-mobile.dc.html`（可交互，四屏 + 推送 + 图标）。

移动端是 gtmux 的**第三块屏**：桌面的远程伴侣。手机跑不了 tmux，所以它是
`gtmux serve` 的纯消费方，经 VPN/Tailscale 连接，**只读 MVP**（监控 + focus + 推送）。
状态语言与菜单栏完全一致：**颜色 + 形状 + 字形**，颜色只编码状态、绝不编码 agent 身份。

---

## 1. App 图标（gtmux mobile）

品牌母题 = **pane 网格**：2×2 网格，右上一格点亮**青色 `#06B6D4`** = 「被聚焦 / 在等你的
那个 pane」。深底、克制、小尺寸可辨。**App 图标不显示状态计数**（那是菜单栏状态项的职责）。

### 画法

- 画布满出血方形；圆角交给 iOS 系统 squircle 蒙版（设计稿用 ~22.5% 圆角预览）。
- 背景：`linear-gradient(160deg, #262B36 0%, #0E1016 100%)`，顶部 1.5px 内高光
  `inset 0 1.5px 0 rgba(255,255,255,0.08)`。
- 网格：居中、约占图标宽 58%；3 格中性 `rgba(255,255,255,0.22)` + 右上 1 格青
  `#06B6D4`（带轻微外发光 `0 4px 14px rgba(6,182,212,0.5)`）；底排单元横跨两列。
- 网格布局（与品牌 logo 一致）：

  ```
  ┌──────┬──────┐
  │ 中性 │ 青色 │   ← 右上点亮
  ├──────┴──────┤
  │    中性     │   ← 底排跨两列
  └─────────────┘
  ```

### iOS 18 变体（必交付）

| 变体 | 背景 | 网格 |
|---|---|---|
| Default | `#262B36→#0E1016` 渐变 | 中性白 22% + 青 |
| Dark | `#000000` | 中性白 16% + 青 |
| Tinted | `#1A1A1D` | 单色：中性白 30% + 亮白 85%（系统再着色） |
| Light | `#EEF0F3→#DADDE2` 渐变 | 中性黑 16% + 青 |

### 导出

全套 iOS 尺寸 20–1024（@2x/@3x）：notification 40、settings 29、spotlight 20、home 60、
App Store 1024。建议从矢量（网格是纯矩形 + 圆角）按尺寸重绘，避免小尺寸网格糊成一团。

---

## 2. Agent 图标（行首头像）

雷达每行行首是 **agent 头像**，用来区分「哪个工具」（Claude Code / Codex / Gemini …）。

### 规则

1. **真机显示各工具官方图标**，运行时从 `Agent.icon` 加载（`gtmux serve` 的 `agentJSON`
   已含该字段：`.app` 路径或图片）。
2. **官方 logo 是第三方商标 —— 不在仓库里重绘、不内置第三方 logo**（DESIGN §6）。
   iOS 端把 `Agent.icon` 解析为可加载源；解析不到时回退中性字标。
3. **回退中性字标**（IP 安全，区分用，不是 logo）：

   | agent | mark | agent | mark |
   |---|---|---|---|
   | Claude Code | `CC` | Cursor | `Cu` |
   | Codex | `Cx` | Crush | `Cr` |
   | Gemini | `G` | Amp | `Am` |
   | aider | `Ai` | Cline | `Cl` |
   | opencode | `oc` | 其它 | 名称前 2 字符 |

4. **颜色仍只属于状态徽章**：头像容器保持中性（`surface` 底），不给 agent 上色。

### 头像容器（app-icon 风格）

- 尺寸 **34pt**，**圆角方块 radius 9**（不是圆形——signal「这里放 app 图标」），
  `overflow:hidden` 让方形官方图标自然贴合；右下角叠 16pt 状态徽章。
- `AgentRow.tsx`：

  ```tsx
  {agent.icon
    ? <Image source={ {uri: resolveIcon(agent.icon)} } style={appIcon} />
    : <Text style={mono}>{agentMark(agent.agent)}</Text>}
  ```

> 注：这把已建的 `AgentRow` 头像从圆形改为圆角方块，并接入 `Agent.icon`。其余行结构
> （primary 加粗 · secondary 灰 · task · time · ›）不变。

---

## 3. Radar 交互（列表）

### 可折叠分区（必须可发现）

每个状态分区头是**可点折叠条**——发现性是硬要求，别只放一个小箭头：

- 左：分区名（waiting 红、其余中性）+ **计数气泡**（`surface` 底、描边、圆角 9）。
- 中：一条 `0.5px` 分隔线把头部拉成整条。
- 右：**显式文字 `Hide / Show`（收起 / 展开）** + **圆形里的箭头**（展开 ▼ 朝下 / 折叠 ▶ 朝右，`rotate(-90deg)`）。
- 整条有按压高亮（`style-hover` → `rowSel`）。
- 折叠后计数气泡仍在，便于收起也知道数量。状态可持久化（下次打开保持）。

### 分区之间的分隔

相邻分区之间插一道**分隔槽**（除第一个分区外）：`9px` 间隙铺页面底色 + `3px` 顶部粗线
（dark `rgba(255,255,255,0.16)` / light `rgba(0,0,0,0.16)`）。让「需要你 / 运行中 / 空闲」一眼
切成独立组块，而不是一条连续长列。

### 其它

- 「只看等输入」过滤；下拉刷新；初始 `GET /api/agents` + SSE 驱动重拉。
- 顶栏右侧：连接状态点（live / reconnecting / offline）+ 齿轮进设置。
- waiting 行：淡红底 + 红方块·双竖线徽章 + 一次脉冲。

---

## 4. Detail 交互（pane 视图 + 输入）

> **实现现状（2026-06，权威）：** pane 视图是原生 RN `<Text>` 渲染器
> `src/ui/NativeTerm.tsx`（capability `mobile-pane-renderer`），**不是** webview/
> xterm.js。早期的 xterm-in-webview 经过约 10 个 PR 的 WebGL/canvas/DOM 在真机上的反复
> 折腾后被放弃；下面的「换行/滚动切换、↓ 跳到底部 FAB」属于旧 webview 方案,**已被原生渲染器
> 取代**（原生 ScrollView 跟随到底 + 长按选区,见下）。色板/字体仍走 `theme.ts` + `terminal-theme`。

### 终端渲染（原生 NativeTerm）

- 数据：每 ~1.5s `GET /api/pane`，**`capture-pane -e -p -S -2000`**（可见屏 + 至多 2000 行回滚，
  带 ANSI SGR）。**不右裁尾部空行**,使光标偏移能锚到真实底部(见光标）。
- **彩色输出**：共享 ANSI/SGR 解析器 `src/ui/ansi.ts` 把转义映射到彩色原生 `<Text>` span
  （fg+bg、bold/dim、256/truecolor）。capture-pane 已把光标移动/清屏/备用屏解析成平面彩色网格,
  故无需终端模拟器。色板对齐 `theme.ts` / `GET /api/theme`。
- **长按选区 + Copy（透明叠层）**：彩色的深层嵌套 `<Text selectable>` 在真机上能选能复制但**画不出
  可见高亮**;因此选区由一层独立的**扁平单色 `<Text selectable>`**(字形透明)叠在彩色层之上,iOS 的
  半透明高亮层于是把背后的彩色文字染上底色 —— 无跳动、无模式切换。
- **触摸期冻结**：手指按住时冻结快照(文本 + 光标),streaming 的 pane 刷新不会抹掉进行中的选区/滚动;
  松手后稍延迟(~3.5s 或滚动到底)再 thaw,把期间缓冲的最新快照应用上。
- **跟随到底**：默认跟随 pane 底部;用户上滑看历史则停止跟随;再滑回底部时即便 pane 仍在 streaming
  也能抵达真实 live tail 并恢复跟随(`stick` 标志 + 到底阈值)。
- **光标**：按 `GET /api/pane` 的 `cursor{x,up,visible}`(底锚:`up` = 距末行上推行数)画一个反显格。
- **字形归一**：U+23FA「⏺」在 iOS 会被当成红色 emoji,映射为 U+25CF「●」(纯文本字形,随 SGR 上色)。
- 等宽字体(Menlo → PingFang 兜底 CJK 2 格宽);仅渲染末 500 行;离线时显示最后一帧。
- 纯逻辑(`cursorSpans` / `normalizeGlyphs` 抽到 `src/ui/term.ts` 单测;光标算术见 `paneCursor`)。

### 对话 / chat 模式（NativeTerm 的姊妹视图）

Detail 顶部「终端 / 对话」可切。对话视图(`src/ui/ChatView.tsx`,capability `mobile-chat-view`)
渲染**解析后的会话历史**(`GET /api/transcript`,capability `chat-transcript`),比裸终端更易一眼读懂
agent 做了什么:

- **多段气泡**：一个回合的回复按 `segments` 时序拆成多个 speech bubble —— 每段是一条 assistant 文本
  气泡 + 其后运行的工具步骤(text → tools → text → …)。**每条文本气泡都挂 agent 头像**(一个回合常被
  工具调用切成很多段,只在首段挂头像会让后续段落像孤儿气泡 —— 与 web 对话模式一致);中间穿插的工具
  步骤折叠为可点开的「N 步」组,夹在气泡之间,使中间过程一目了然。
- **带日期的时间标签**：取回合 prompt 的时间戳,带日期(今天/昨天 + HH:MM、否则日历日期、跨年带年份),
  随设备语言 en/zh;相邻相同标签去重(`fmtTurnTime`,抽到 `src/ui/time.ts` 单测)。
- **长按选区 + Copy**：prompt 气泡与渲染后的回复块均可长按选取复制,与终端视图一致。
- 数据形状(`Turn` / `Segment` / `Step`)见 `api/contract.md` 的 `/api/transcript` 与 `chat-transcript` spec。

### 顶栏

- 返回 ‹ + **agent 官方头像**（圆角方块 ~30pt，右下叠状态徽章，与雷达行一致 §2；不要只放一个
  裸状态徽章）+ primary/secondary；**Focus on Mac 移到顶栏轻按钮**（不占输入区），
  = `POST /api/focus`。

### Composer（输入 · 已上线，仅 bearer token 门控）

> **现状：** 终端输入**已上线、默认开启**,仅由配对的 bearer token 把关(无单独授权门 —— token
> 即密码,泄露即可在 Mac 上执行命令)。早期文档把它写成「Phase 2 · 写入需一次性授权」,**已过时**。
> 发送后 app 立即刷新 pane(不等下一轮轮询),并对刚发的 prompt 做乐观回显。

输入主次分明、**突出 agent 相关管理输入**，自由输入作为扩展。**2026-06-30 重设计:去掉罗列感**——
旧版静息行塞了十余个 pill(方向键盘×2、粘贴、继续/停止、独立 ⏎、Tab、平铺的每条快捷短语…),
已大幅精简:

- **静息键行(结构化、精简)**:`⌨` ｜(仅 waiting 时)`1·Yes 2·Always 3·No` ｜ `Ctrl-C` `Esc`
  ｜ `快捷短语 ▾` `历史`。
  - **去掉**:方向键盘(MoveKey + 浮动键盘 ✛)、`粘贴`(并入「+ 附件」动作表,仍支持剪贴板图片
    →标注→上传)、`继续/停止`对(继续=噪音;停止=Ctrl-C 重复)、独立 `⏎`(与发送重复)、`Tab`。
  - **快捷短语不再平铺**:收成一个 `快捷短语 ▾` pill → 原生 action sheet 选/管理。
- **打字时(键盘弹起)**:输入行 = `+`(附件:相册/拍照/文件/粘贴)＋ 自由输入框 ＋ `⤢`(全屏撰写)
  ＋ `↑`(发送)。键行(含 `▾` 收起 + 上述精简键)贴在键盘上方。
- 全部走 `POST /api/send`（send-keys）；发送后立即 `bumpPane` 刷新。

**样式与布局要求（对标 Moshi，2026-06-29 迭代）：**
- **键统一为「填充药丸」**：高 40pt（含行内边距 ≥44pt 触达）、圆角 11、`surface` 底 + 发丝边、
  字号 14 / 图标 17。不要用过小（~32pt）的描边小键，也不要稀疏铺开。
- **底部留安全区**：静息（键盘收起）时 composer `paddingBottom = max(8, safeArea.bottom)`，
  让边缘键不落进 home-indicator 手势区；键盘弹起时折叠该内边距（键盘已覆盖）。
- **键盘弹起**：键行（含 `▾` 收起键 + 控制键）紧贴在输入框上方，打字时仍可够到特殊键；
  用一个**空的 `InputAccessoryView` 抑制 iOS 自带的 上一项/下一项/完成 稀疏工具条**（不要让它出现）。
- **静息态**：仅一行键（`⌨` 唤起输入框/键盘，`▾` 收起），保持终端满高、避免误触弹键盘。

### 语音输入

- 麦克风键唤起全屏聆听态：脉冲麦克风 + 波形 + 实时转写 + 取消 / 发送。
- 转写结果走与 composer 同一条 `POST /api/send`（受同样写权限门控）。

---

## 5. iPad / 平板与自适应布局

iPad **不是「放大的手机」**——大画布用 **split-view（侧栏雷达 + 主区详情）**，一屏掌控全局。

### 布局

- **侧栏（左，宽 300–320pt，常驻）**：gtmux 标题 + 连接点 + 汇总；分组雷达列表（需要你→运行中→空闲），
  沿用 §3 的可折叠分区 + 分区分隔；**选中行高亮**（`rowSel` 底 + 左侧 `2.5px` 青色 accent 条）。
- **主区（右，占剩余）**：选中 pane 的头（状态徽章 + primary/secondary + `cols×rows·live` + Focus on Mac）；
  彩色终端（同 §4）；底部 composer（agent 化快捷键 + 输入 + 语音）。
- 点侧栏任意行 → 主区即时切换（`pickAgent`，不跳页）。

### 自适应（size class 断点）

- **横屏 / 宽（regular）**：双栏并置（如上）。
- **竖屏 / 分屏变窄**：侧栏退为可滑出抽屉（汉堡包按钮唤出），详情占满。
- **进一步压缩（Slide Over / 窄）**：回退手机式单列堆叠（雷达→详情两步）。
- 同一套 RN 组件按 `useWindowDimensions` 断点切换（建议阈值 ≈ `width ≥ 768` 走双栏），**不另起一套**；
  导航层加 `regular / compact` 分流。

### iPad extras

- 硬件键盘：`↑↓` 选 agent、`⏎` focus、`⌘1–9` 直达、`⌘F` 搜索。
- Stage Manager / 多任务：窗口可变宽，按断点自适应。
- 字号 A−/A+ 与回滚缓冲沿用；终端在大屏可显更多列。
- 需适配横/竖旋转，不锁定方向。

---

## 6. 视觉规范回顾（与菜单栏一致）

- **状态**：waiting `#EF4444` 红方块·双竖线 / working `#06B6D4` 青圆·加载环（静态不转） /
  idle `#22C55E` 绿圆·对勾 / running `#8E8E93` 灰圆·小圆点。
- **分区顺序**：needs-you → working → idle → running；waiting 分区标题红、其余中性。
- **深/浅色**（theme.ts）：dark `bg #0D0D0F · surface #1C1C1F`；light `bg #F2F2F7 · surface #FFF`。
- **i18n**：en/zh 跟随系统，设置可锁定；CJK 省略号截断、绝不换行。
- **动效**：仅 idle→waiting 一次脉冲；其余安静。无渐变滥用、无发光阴影、文案不营销。

### 设置（Settings · 对标 Moshi，2026-06-29）

- **分组卡片**：按 `连接 / 终端 / 推送 / 通用 / 关于` 分组；每组一张圆角卡，组上方小号大写灰标题。
- **每行**：行首 **flat outline 图标**（24-grid 描边，`SettingsIcons.tsx`）+ 标题 + 右侧「当前值 + ›」
  或开关。**多选项设置收成单行**（值 + `›`），点开 `PickerSheet`（底部弹层 + 选项 + 勾），
  不要把整组单选铺成长列表。布尔项用内联开关。
- **可达性**：行高 ≥52pt；危险项（移除服务器）红色 + trash 图标。
- 复用组件：`SettingsGroup` / `SettingsRow` / `PickerSheet`（`src/ui/SettingsRow.tsx`）。**菜单栏设置同一组织语言**（见 DESIGN.md）。

---

## 7. 推送与连接

- 路径：`gtmux serve → push relay（持 APNs key、无状态）→ APNs → 设备`。iOS 用原生
  APNs token，**不需要 Firebase**；token 经 `POST /api/push/register` 存在 Mac。
- `alert` 两类：`waiting`（任意态→waiting）/ `done`（working→idle）。前台改为应用内 banner。
- **点推送 → 深链直达该 pane 的 Detail**（读 payload 的 `pane`）。
- APNs 由 Apple 投递，**离开 VPN 也能收到**；只有拉实况 / focus 需要内网。
- 配对二维码 schema v1：`{ "v":1, "url":"https://host:port", "token":"<serve-token>", "name":"…" }`。

---

## 8. 状态与边界

| 场景 | 行为 |
|---|---|
| 0 agent | 空状态卡「没有在跑的 coding agent」，不报错 |
| 1 waiting | 落「需要你」，红方块·双竖线 + 淡红底 + 脉冲一次 |
| 15+ | 列表滚动；「只看等输入」过滤收窄 |
| 超长 task / CJK | 单行省略号截断，绝不换行/溢出 |
| 离线 / 重连 | offline 红点 / reconnecting；推送仍由 APNs 投递照收 |
| idle→waiting | 绿圆→红方块，一次脉冲 + 提醒 |

---

## 9. 路线图

- **MVP**：只读监控 + focus + push（本设计覆盖）。
- **P2**：终端输入 `POST /api/send`（send-keys，写权限门控）。
- **P3**：语音。
- **P4**：Android / HarmonyOS（RNOH，组件保持平台中立）。
