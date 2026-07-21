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

### 终端渲染（窄屏适配）

- 数据：每 ~1.5s `GET /api/pane`。**`/api/pane` 用 `tmux capture-pane -e -p`**（带 ANSI SGR），
  保留颜色。
- **彩色输出**：RN 端用一个轻量 ANSI/SGR 解析器把转义映射到彩色 `<Text>` span，对标 macOS
  Terminal「Pro」深色：prompt `$` 绿、命令名青、commit 哈希黄、PASS/✓/`ok`/diff `+` 绿、
  FAIL/diff `-` 红、`Tool use:` 品红、盒线/选择器暗灰、`❯` 选中绿、正文 `#D6D6DA`。
  色板对齐 `theme.ts`。
- **窄屏 ↔ 宽窗技巧**：②**字号 A− / A+** 三档 + ④**回滚缓冲** + 右下 **↓ 跳到底部** FAB（**已实现**）。
  ①**换行 / 滚动**切换与③顶部 `cols × rows · live` 指示 **暂缓**：① iOS 上嵌套横向 `ScrollView`
  会白屏（NativeTerm 现固定按手机宽度软换行，见其注释）；③ 服务器 `/api/pane`/`agents` 目前不下发
  pane 真实列宽/行高，做出来只能是合成值 —— 需先加一个 pane 尺寸字段（契约变更）才有意义。
- 等宽字体；离线时显示最后一帧。

### 顶栏

- 返回 ‹ + 状态徽章 + primary/secondary。（手机侧 **Focus on Mac 已移除**（#85）—— 顶栏不再放该
  按钮，与 mockup 一致；对焦 Mac 仍可在菜单栏 app 侧完成。）

### Composer（输入 · Phase 2，写入需一次性授权）

输入主次分明、**突出 agent 相关管理输入**，自由输入作为扩展：

- **上下文快捷键（agent 化）**：waiting 时直接给 `1·Yes / 2·Always / 3·No`；其它状态给
  `继续 / ⏎ / 停止`。
- **控制键排**：`⏎ Ctrl-C Esc Tab ↑ ↓`。
- **自由输入框 + 发送**：任意文本兜底。
- 全部走 `POST /api/send`（send-keys），**写权限门控**：未授权时 composer 置灰并标注
  `Phase 2 · 写入需一次性授权`。

### 语音输入

- 麦克风键唤起全屏聆听态：脉冲麦克风 + 波形 + 实时转写 + 取消 / 发送。
- 转写结果走与 composer 同一条 `POST /api/send`（受同样写权限门控）。

---

## 5. iPad / 平板与自适应布局

iPad **不是「放大的手机」**——大画布用 **split-view（侧栏雷达 + 主区详情）**，一屏掌控全局。

### 布局

- **侧栏（左，宽 300–320pt，常驻）**：gtmux 标题 + 连接点 + 汇总；分组雷达列表（需要你→运行中→空闲），
  沿用 §3 的可折叠分区 + 分区分隔；**选中行高亮**（`rowSel` 底 + 左侧 `2.5px` 青色 accent 条）。
- **主区（右，占剩余）**：选中 pane 的头（状态徽章 + primary/secondary + 连接指示；`cols×rows·live`
  与 Focus on Mac 同 §4 —— 前者暂缓、后者已移除）；
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


---

## HQ 中控（HQScreen）· §17 mockup

点 `role:"supervisor"` 落到专属 HQScreen（非普通 Detail）：状态条（舰队计数 + 订阅窗口%）· 舰队态势板（`/api/digest`）· 命令台（ChatView + 快捷 chips，选中行时变 continue/inspect/reply-for-me）。雷达里 HQ 用 ⌂ 置顶青框。已实现于 `HQScreen.tsx`，此为视觉规范镜像。三屏（菜单栏/手机/网页）同一心智：态势 + 对话 + 派活，规模随屏放大。


---

## 对齐实现补记（2026-07 · F 轮）
见 ITERATIONS-2026-06.md §F。要点：计费全部移出手机（唯一付费点=Mac 端 Direct 兑换码）；Servers 两轨分组（我的 MAC/访客连接）；Composer 静息键条 ⌨|Tab ⏎ Ctrl-C Esc|快捷短语▾ 历史，写死 1/2/3 移除、回应归 ApprovalCard（/api/options 1..N）；回车=换行、↑ 发送、⤢ 全屏撰写、附件暂存-发送时上传；通知快回=固定三键数字不带 Enter；设置=Moshi 分组+PickerSheet，访客隐藏 owner 项；iPad=SplitScreen 宽度≥768；HQ 雷达入口=参谋长卡（无角标+舰队光点条）。

- **Demo 模式**（mockup §18）：全功能无 server 演示（App Review 路径）。铁律：明示样例（DEMO chip 全程）、永不混入真实（Servers 无条目）、每次进入重置、每步引导「配对你的 Mac」。剧本主线 = 30 秒核心循环：看到等你 → 点进 → 按 1 批准 → 测试跑完 → 雷达变绿挂 latest。优化项见 ITERATIONS §F7。
