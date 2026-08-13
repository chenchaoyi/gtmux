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

### 邻居 pane 条（tiered-pane-control）

Detail 顶部（header 下、segmented 上）一条**横向邻居 pane 条**：列出该 pane 所在 tmux
session 的**其他 pane**（`GET /api/panes` 过滤同 session），agent 行 ▸ + 名、普通行 › +
命令名。点一个 = 打开那个 pane 的 Detail(任意 pane 可看/可输入,普通 pane 以 `paneRowToAgent`
适配)。**无兄弟 pane 或全屏时隐藏**;guest 只看到被授权的 pane。桌面侧的"邻居"由 §16 的
pane 浏览器(按 session 分组)覆盖,手机侧则用这条 —— 都基于同一 `/api/panes` 契约。

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
- **短缓冲不黑屏**：capture 会保留 pane 网格的**尾部空行**（服务端为底锚光标的行数数学而保留，
  见 `internal/tmux` CapturePaneColor），渲染端必须把全空尾行裁掉再显示（`term.ts renderView`：
  光标行号先在**未裁剪**数组上算、裁剪永不切到光标所在行）——否则一个大而空的 pane（200×50 只有
  5 行内容）或 `clear` 之后，跟随底部的滚动视图停在空行区，整屏全黑。
- 等宽字体；离线时显示最后一帧。

### Terminal text selection（iOS 终端文本选中）

iOS 端选中/复制是**原生实现**（Android 保持 `<Text selectable>` 平铺 overlay 不动）。四次
失败尝试（RN selectable 只有菜单、UITextView overlay 错位+卡顿、全屏 select sheet 估算漂移、
只能向下拖）之后定案：**统一网格 + 自实现 UITextInput 只读子集**，让系统选中 UI（band +
双向手柄 + 放大镜 + Copy 菜单）直接画在我们自己的彩色渲染上，几何完全由我们提供 → 零错位。
设计与验证记录见 `openspec/changes/mobile-native-term-selection/`（设备验收 2026-08-08）。

怎么工作：

- **Stage 1 · 统一网格（JS，`term.ts`/`NativeTerm.tsx`）**：每个可视行显式同高
  `rowHeightFor(fs)`（1.6×，Menlo 与 PingFang CJK 同高）；逻辑行在 JS 里按**格子算术硬换行**
  （`charCells` CJK=2、选择符/ZWJ=0；`colsFor` 容量保守 −1，行永不被 RN Text 原生再折行）。
  于是行几何是纯算术 `row = ⌊y / rowH⌋`，且 char-wrap 与 tmux 自己的折行方式一致。
- **Stage 2 · 原生层（`mobileapp/ios/TermSelection/`）**：透明 `TermSelectionView`
  absoluteFill 盖在行栈上，实现 UITextInput **只读子集**（positions/ranges/`caretRect`/
  `selectionRects`/`closestPosition`…；行内 x↔char 用缓存 CTLine 的 Core Text advance —— 与
  RN Text 同一 shaping 引擎、同一字体，CJK fallback 的 advance 是量出来的不是假设 2×cell）。
  系统件：band+双手柄 = `UITextSelectionDisplayInteraction`（iOS 17+）、放大镜 =
  `UITextLoupeSession`、Copy 菜单 = `UIEditMenuInteraction`（en/zh）。手势全部自驱：激活长按挂在
  外层 scroll view 上，未激活时 overlay 对触摸隐形（`point(inside:)` false + box-none），链接
  点按/滚动不受影响；选中期间 JS 冻结快照（`onSelectionActive` → freeze/thaw）。

五个坑（改这块前先读，每个都真踩过）：

1. **有效字号必须处处折算 Dynamic Type**：`fs = fontSize × PixelRatio.getFontScale()`（代码取
   `useWindowDimensions().fontScale`），wrap cols、rowH、块行字体、overlay 的 CTLine 字体**全部**
   从这一个数派生，块行再设 `allowFontScaling={false}` 防二次缩放。RN Text 默认按 fontScale 缩放
   而网格算术不缩 → 字号档位 > Large 的真机上渲染栈超出 rows×rowH，底部 (scale−1)/scale 的屏幕
   长按全是死区（1.0-scale 的 sim 复现不了；`simctl ui content_size extra-large` 可复现）。
2. **不要挂 UITextInteraction**：它的内部手势会在 handle 拖动中每帧清空 `selectedTextRange`，
   拖动即断（sim 实证）。band/手柄/放大镜/菜单用上面三个公开件显式实现；系统 handle view 还要
   `isUserInteractionEnabled = false`，否则旋钮吞掉触摸、拖不动。
3. **跨 interop 传色用 hex 字符串**（`"#RRGGBB"` prop，原生侧自己解析）：`processColor` 打包
   ARGB 而新架构 interop 层按 RGBA 解码 → 蓝色选中带变成红色。
4. **overlay 文本必须与渲染同源**：`flattenGrid` 用同一次 wrap 同时产出行栈与 overlay 文本，
   不变式 overlay 行数 == 栈行数（unit-tested）；光标 splice 追加的补位空格取 RAW 行、绝不漏进
   Copy。行数一漂移，其下每一行的选中都错位。
5. **Swift 只消费 props、绝不自算**：rowHeight/fontSize/padTop/padLeft 全部来自 JS
   （`rowHeightFor`/`PAD` 是唯一数值源）；在原生侧用 UIFont metrics 自推行高，就是把
   UITextView 时代的错位事故重演一遍。

### 顶栏

- 返回 ‹ + 状态徽章 + primary/secondary。（手机侧 **Focus on Mac 已移除**（#85）—— 顶栏不再放该
  按钮，与 mockup 一致；对焦 Mac 仍可在菜单栏 app 侧完成。）

### Composer（输入 · Phase 2，写入需一次性授权）

输入主次分明、**突出 agent 相关管理输入**，自由输入作为扩展：

- **上下文快捷键（agent 化）**：waiting 时直接给 `1·Yes / 2·Always / 3·No`；其它状态给
  `继续 / ⏎ / 停止`。
- **控制键排**：`Tab ↑ ↓ ⏎ ⌫ Ctrl-C Esc`（横向可滚）。`↑/↓` 导航、`⏎` 提交、`⌫` 退格 ——
  这样交互式 TUI picker（Claude Code 的 AskUserQuestion，单选/多选）能在终端里被驱动。这类
  富 picker **不出一键 ApprovalCard**（裸数字驱动不了它），终端键排就是它的回复通道。
  （`␣` 空格键 2026-08-08 移除：实际没用，字面空格从输入框就能打；`⌫` 发 tmux `BSpace`，
  修 agent 输入行里的手滑。）
- **自由输入框 + 发送**：任意文本兜底。
- 全部走 `POST /api/send`（send-keys），**写权限门控**：未授权时 composer 置灰并标注
  `Phase 2 · 写入需一次性授权`。

### 语音输入

- 麦克风键唤起全屏聆听态：脉冲麦克风 + 波形 + 实时转写 + 取消 / 发送。
- 转写结果走与 composer 同一条 `POST /api/send`（受同样写权限门控）。

---

### 悬浮在终端内容之上的控件（全屏退出等）

浮在**任意终端输出**之上的控件，**不能向背景借对比度**。退出全屏的胶囊曾经用半透明填充
（`rgba(20,20,22,0.82)`）+ 0.14 alpha 的发丝边 —— 终端自己的字直接透上来，控件跟背景糊成一片。
铁律：**填充必须不透明**、边框是一条真的线、再加投影把它抬起来。这样同一个胶囊在密集文字上、
在空白面板上、在深色或浅色终端主题下都读得清。文字用**固定浅色**（这类面永远是深色，取
`pal.fg` 在浅色模式下会变成近黑 → 隐形，见 [[mobile-light-mode-dark-surface-trap]]）。

图标要表达**动作**，不是形状：退出全屏用 `✕`（关闭），不要用 `⤡`（对角缩放箭头 —— 读作
"变大变小"，不是"离开"），并且配全词（"退出全屏"），不留猜测空间。

### 全屏模式 = 阅读态

全屏的定位是**阅读空间最大化**：只留内容（终端/对话）——顶部 chrome **和底部
composer/键条**全部隐藏，退出即恢复（2026-08 用户定）。ApprovalCard / 发送失败条是
**异常态提醒**、不是 chrome，仍会浮现。终端全屏时，安全区背板必须用**终端自身的深色**
（`TERM_BG` / 终端主题背景），不随 app 主题 —— 浅色模式下 `pal.bg` 曾在永远深色的终端
上方画出一条亮带（dark-surface 陷阱的背板变体）；对话全屏背板仍随主题（对话面本身就随主题）。

**浮起的退出胶囊不能压住正文**：内容层在全屏时下移一个胶囊的高度（`fsTopInset`，约 46pt）。
它确实吃掉一点「最大化」的阅读空间 —— 但**看得见的空间胜过藏了一行字的空间**；竖屏还能忍，
横屏整个阅读区才 ~390pt 高，第一行被永久盖住（2026-08-09 用户反馈）。

**横屏的安全区是左右，不是上下**：`SafeAreaView` 只取 `top` 时，横屏下刘海/灵动岛跑到侧边，
终端和对话都会钻到它下面。取 `['top','left','right']`——竖屏下左右 inset 为 0，等于只影响横屏。

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
- 同一套 RN 组件按 `useWindowDimensions` 断点切换，**不另起一套**；导航层加 `regular / compact` 分流。
- **断点看画布，不只看宽度**（`ui/layout.ts` 的 `isSplitCanvas`：`width ≥ 768 且 height ≥ 600`）。
  现代 iPhone 横屏宽 852–932pt，早就越过宽度线，但只有 393–430pt 高 —— 只看宽会把 iPad 的
  「侧栏 + 详情」塞进一块比键盘高不了多少的画布上，正好是 §5 开篇那句话的反面：**iPad 不是放大的
  手机，手机横屏也不是缩小的 iPad**。600 这条线放过所有 iPad 方向（最矮 768pt），挡住所有手机。

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

## 8b. 更新内容弹窗（What's New）

更新后**只弹一次**，等价于 CLI 的 `gtmux whatsnew`；Settings → 关于里可随时再看
（只能看一次的更新说明等于看不到）。

**跨版本是核心场景**：跳过了三个版本的用户，必须看到那三个版本的说明，而不只是最新的一条。

- **两层结构，与 CLI 同构**：
  - 弹窗 = 摘要。按版本分组、新的在前、**截断到 8 条**（CLI 的 `changelogMax` 是 5；这里是
    用户主动在看的一张卡片，8 条刚好让常见的单版本发布（5–6 条）完整显示，折叠只在「你确实
    跳过了版本」时才出现）。折叠处显示「还有 N 条 —— 全部显示」，**原地展开**，不跳页。
  - Settings → 更新内容 = 全量，直接展开。
- **截断的三条规矩**（都写进了规范）：某个版本要么整版显示、要么整版折叠（「0.46.0 的 6 条里
  给你看 3 条」对读者毫无意义）；折叠一定是**后缀而不是空洞**（为了塞下更小的版本而跳过中间
  某个版本，等于告诉读者那个版本什么都没改）；**最新版本永远显示**，哪怕它一条就超了上限。
- **文案来源是逐版本归档** `mobileapp/release-notes/<version>.{en,zh}.txt`，由
  `scripts/gen-release-notes.sh` 编译成 `src/releaseNotes.ts`（新的在前）。**不能用 App Store
  元数据**当来源 —— 它每次发版被覆盖，只剩当前版本。`set-version.sh` 在文案确实变化时把当前
  store 文案归档到新版本号下，所以正常发版流程不需要额外步骤。`check-design.sh` 重新生成后逐
  字节比对，不一致就红。
- **语言**跟随语言三态（跟随系统 / EN / 中文）；某一语言缺失时回退到另一种 —— 与 CLI 在 tag 的
  `user:` / `user-zh:` 之间的回退规则相同。截断计数按**正在阅读的那种语言**算。
- **版本序按数字段比较**（0.10 在 0.9 之后），比 binary 更新的归档条目不显示。
- **首次安装不弹**：没见过旧版就无所谓「新」。静默记下版本，让**第一次更新**成为首次问候。
  归档里没有内容的版本同样不弹（CLI-only 的发布对手机用户可能确实无话可说）。
- **视觉**（§6 铁律）：**不分 NEW/ENHANCED/FIXED 那类栏目、无强调填充、无动效** —— 更新说明是
  读一遍就关掉的东西，包装它就是被禁止的营销腔。居中卡片而非全屏页，点遮罩或按钮都能关。条目
  是散文，**换行不省略**（§8 的 CJK 省略号规则针对列表行）。
- **品牌感来自产品自己的语汇，不是装饰**：卡头放 `BrandMark`（就是 app 图标那个 pane 网格），
  版本号一律用 `Menlo`（终端、分支 chip、HQ 数字用的同一种等宽 —— 版本是 token 不是散文），
  条目符号是**一个 pane 格子**（5pt 圆角方块）而不是排版圆点。就这些，不再多。
- **版本号不出现两次**：分组标题在时，卡头的版本 chip 让位；只有一个版本时反过来。两处都印
  不是层次，是噪音。
- **绝对不能把 `ScrollView` 包在 `Pressable` 里**。遮罩曾经是卡片的**父节点**（外层点击关闭 +
  内层空 `onPress` 拦截穿透），于是 touch 在 start 就被 JS responder 抢走，原生滚动拿不到手势 ——
  卡片滑不动/一顿一顿（2026-08-09 用户反馈）。正确结构：遮罩是卡片的**兄弟节点**（`absoluteFill`
  垫在后面），卡片是普通 `View`。有测试直接断言组件树里 `ScrollView` 没有 `Pressable` 祖先。

---

## 9. 路线图

- **MVP**：只读监控 + focus + push（本设计覆盖）。
- **P2**：终端输入 `POST /api/send`（send-keys，写权限门控）。
- **P3**：语音。
- **P4**：Android / HarmonyOS（RNOH，组件保持平台中立）。


---

## HQ 指挥页（HQScreen）· §17

点 `role:"supervisor"` 落到专属 HQScreen（非普通 Detail）。

**铁律：HQ 页面不再逐条罗列舰队。** 那张列表是雷达的，退一步就能看到；旧版的「舰队态势板」
只是把它缩小塞进聊天上方，于是它对「自己冗余」的答案变成一个折叠开关，折起来留下一条空横杠
（`hq-command-page` 就是修这个）。舰队**计数**留在状态条，**列表**归雷达。

页面只回答雷达答不了的三个问题，用**什么只有参谋长知道**来搭：

1. **状态条** — gtmux HQ + 连接点（+ DEMO 标）· 舰队计数 · 订阅窗口% · 资源告警。
2. **判断（⟣ 常驻两行内）** — 确定性结论（与 HQ 卡片 `fleetHeadline` 同一套规则，故卡片与页面
   永不打架；有人等你时转琥珀）+ 一行「态势板 · N前 ›」。点开 = **参谋长自己的作战态势板**
   （`GET /api/hq/board`，等宽只读全屏）。**新鲜度必须显示**——态势板是人维护的综合判断，
   它多久没更新，本身就是该信它几分的依据。板不存在时这一行整行不出现，只留判断句。
3. **对话分区必须显示"在跑"** —— HQ 收到指令后可能长时间不出任何字。旧版这里 `lines={[]}`，
   连 Detail 已有的 live 卡片都没有：你的指令回显完就是一片死寂，与 app 卡死无法区分。现在
   console 会拉 HQ pane 的实时屏幕，并且**没有输出时也挂一条"正在思考… 42s"**。
   **必须带时长**——"working" 一个词分不出"在想"和"挂了"，而只有后者值得你去打断。
   拿不到起始时间就只写"正在思考…"，**绝不编一个时长出来**。
4. **三个分区（分段切换，各占满整个 body，不做同屏挤压）**
   - **该你拍板** — 每个 `waiting` 会话一张决策卡：状态方块·w窗口号·会话名·agent·等待时长，
     **ask 是卡片正文**（旧版把它压成第三行 12px 小字——全产品决策密度最高的一个字段，排成了脚注），
     下面两个动作：`打开会话`（去该 worker 的 Detail，直接输入只住在那里）·`问参谋长`（让 HQ 给回复建议）。
     选中即成为 chips 的目标。**空态说人话**：「现在没有需要你拍板的事。」
   - **动态** — `GET /api/hq/events?severity=notable` 的事件账本（**历史**；雷达只有此刻），
     渲染成人话（`在等你·要授权` / `跑完一轮` / `这一轮崩了` / `收到指令`…），
     important 用琥珀点。空态：「最近没有值得一提的动静。」
   - **对话** — 与 HQ 的会话（ChatView）。
5. **命令台常驻** — chips + Composer 三个分区都在（你随时可能有话对参谋长说）。
   选中决策卡时 chips 变 `帮我回复`/`看它在干嘛`/`让它继续`。

**分段标签自带信号**：`该你拍板` 带琥珀数字角标，`动态` 有未读时带点——**你没在看的分区也要
自报家门**。进页面时：**有人在等 → 落在「该你拍板」**（你就是为这个才点进来的），否则落在「对话」。

**任何分区都不允许「光头 + 空白」**，空态必须是一句话。

**手机雷达里 HQ = 可拖动的浮窗圆盘（`HQDisc`）**，不再是最顶部的卡片。HQ 是**元层**（凌驾于舰队
之上），所以它**浮**在列表上、不随滚动移走，而不是当作「又一张卡」挤在顶部（那样太像一个 session）。
**可自由拖到屏幕任何位置**，落点持久化（`AsyncStorage`，跨启动记住）；轻点=进 HQ、拖动=移动（靠位移阈值
区分）。圆盘内=gtmux 品牌标 **+「HQ」字标**堆叠（只有 logo 指向性不够，字标点明是 HQ，logo 保留）。
圆盘放不下那句合成头条,故**情报头条移到 HQ 页**（无障碍标签仍念出当前状态）。**手机雷达（真实 + Demo）**
都用圆盘——Demo 是给新用户看真实雷达长什么样的，理应与真机一致（早先 Demo 保留 `HQCard`，结果 app 换成浮窗后
Demo 就落后了，用户会当成两套 UI）。**只有 iPad 侧栏（`SplitScreen`）仍用 `HQCard`**（浮窗不适合常驻侧栏）。

**圆盘状态模型（`HQDisc` 的 `discState`，按优先级取最高一个；铁律 色=状态）**——红=需要注意（决策或
**真·资源瓶颈**），角标区分是哪种：

| 状态 | 条件 | 环色 | 中心/角标 | 轻点 |
|---|---|---|---|---|
| 未启动 | 无 HQ session（仅 owner，且已连上） | 灰（整体置灰 62%） | `?` 角标 | 弹说明：HQ 是什么 + 如何在 Mac 启动（手机是远端，不能 spawn） |
| 请你拍板 | HQ 自身 `waiting` | 红 | `!` | 进 HQ 页 |
| 有人等你 | ≥1 worker `waiting` | 红 | 计数 | 进 HQ 页 |
| 资源瓶颈 | 机器 **red 档**真瓶颈（`/api/usage` `resource.machine.tier==='red'`：磁盘临界 / 内存 critical / 负载顶满；雷达慢轮询 25s） | 红 | `⚠` | 进 HQ 页 |
| HQ 运行中 | HQ 自身 `working` | 青 | — | 进 HQ 页 |
| 一切正常 | 其余 | 绿 | — | 进 HQ 页 |

优先级：请你拍板 > 有人等你 > 资源瓶颈 > 运行中 > 正常。**未启动**态即使没有 HQ 也渲染（灰盘 +
说明），但只在 `conn==='live'` 后显示，避免连接中闪现「未启动」。

**只有 red 档才染红盘（低噪铁律）**：软 **amber** 提示（如 37GB 空闲——低于 amber 线 50GB 但远未见底，
或内存 `warn`、负载 1.0~1.5×核）**不**变红。理由:红=「该你拍板/立刻处理」;一个后台磁盘提示若染成和
「session 等你输入」一样的红，用户会误以为 HQ 有事、点进去却什么都没有(实测踩过)。amber 只活在 HQ 页 /
用量视图,不占「一眼看板」的圆盘。`resource.machine.tier` 由 Go `WarnTier` 产出(`amber`/`red`,normal 时
省略),`disk_use_pct` 采**数据卷** `/System/Volumes/Data`(而非只读的 `/`,否则容量%严重偏低误导)。

**跨屏识别令牌统一**（menubar-hq-state-parity）：菜单栏的 HQ 卡片头像现在也是**同一枚圆形 HQ 徽章**
（brand + 「HQ」+ 状态环），用与本处 `discState` 完全相同的六态优先级(Swift 侧 `AgentStore.hqState`,
resource 同样只在 red 档才红、软 amber 不红)。差异仅在容器:手机是**可拖动浮窗**、菜单栏是**卡片头像**
(popover 是固定面板,不浮窗);且菜单栏 absent 态可直接 shell `gtmux hq`(手机是远端,只能弹说明)。改这边
的状态模型/配色时,记得同步菜单栏 `HQMedallion` 与 DESIGN §12。

三屏（菜单栏/手机/网页）同一心智：判断 + 该你拍板 + 对话，规模随屏放大。实现见 `HQDisc.tsx` /
`HQScreen.tsx` + 纯逻辑 `hqZones.ts`。


---

## 对齐实现补记（2026-07 · F 轮）
见 ITERATIONS-2026-06.md §F。要点：计费全部移出手机（唯一付费点=Mac 端 Direct 兑换码）；Servers 两轨分组（我的 MAC/访客连接）；Composer 静息键条 ⌨|Tab ↑ ↓ ⏎ ⌫ Ctrl-C Esc|常用语▾ 历史（用户可见文案 2026-08 起为「常用语 / Quick replies」，代码内部名保持 snippets），写死 1/2/3 移除、回应归 ApprovalCard（/api/options 1..N）；回车=换行、↑ 发送、⤢ 全屏撰写、附件暂存-发送时上传；通知快回=固定三键数字不带 Enter；设置=Moshi 分组+PickerSheet，访客隐藏 owner 项；iPad=SplitScreen 宽度≥768；HQ 手机雷达入口=可拖动浮窗圆盘（`HQDisc`，logo+「HQ」字标，6 态状态环：未启动灰/请你拍板红!/有人等你红计数/资源瓶颈红⚠/运行中青/正常绿，未启动点按弹启动说明；情报头条移 HQ 页；Demo 与真机雷达一致用 `HQDisc`，仅 iPad 侧栏仍 `HQCard`，见 hq-meta-layer）。

- **Demo 模式**（mockup §18）：全功能无 server 演示（App Review 路径）。铁律：明示样例（DEMO chip 全程）、永不混入真实（Servers 无条目）、每次进入重置、每步引导「配对你的 Mac」。剧本主线 = 30 秒核心循环：看到等你 → 点进 → 按 1 批准 → 测试跑完 → 雷达变绿挂 latest。优化项见 ITERATIONS §F7。
  - **Demo 必须与真机雷达同款，不是它的简化版**（2026-08-12 定，此前已因同一原因翻车两次）：
    Demo 是审核员**唯一**能看到的这个 app，也是新用户的第一印象。真机雷达有的**产品能力**，
    Demo 一律要有 —— 2026-08-12 的审计发现缺了三样：**舰队计数行**、**「只看等输入」筛选**、
    **「所有 pane」浏览器**（`demoClient` 连 `panes()` 都没有，整块 surface 在审核路径上不存在），
    而「所有 pane」正是商店描述里写着的能力。另外普通 pane 点开会落到「(no live screen)」死路，
    也已补上可信画面。
  - **Demo 专属**的东西保留：样例横幅、「配对你的 Mac」按钮、关闭按钮 —— 这些是演示脚手架，不是产品差异。
  - **两边都画的 chrome，抽成一个共享组件**，不要各写一份。§17 早就记过 Demo 因为自己留了一份
    `HQCard` 而落后于浮窗改版；这次的计数行/筛选也是同样的机制。现在 `ui/RadarSummary` 由真机与
    Demo 共用，`demoPanes()` 从 `sampleAgents()` **派生**而不是另写一张表 —— 派生的东西不会漂移。


## 18. 服务器模式（纯展示，不控制）

Mac 处于服务器模式（合盖不睡）时，手机要能**一眼看到**，但**不提供任何开关**。

- **呈现 = 连接状态点外的一圈细环**：与状态点同色（已连接绿 / 重连琥珀 / 离线红），
  仅在服务器模式开启时出现。同色是刻意的 —— 它读起来是**同一个指示器的另一种状态**，
  而不是又一个要解读的东西。VoiceOver 标签追加「服务器模式开启」。
- **不做 chip、不做列表行、不进雷达**：服务器模式是**机器**状态不是 agent 状态。
- **Servers 页（我的 Mac 列表）**：当前连接的那台，若处于服务器模式，其连接点外加**同一圈环**，
  副标题加「服务器模式 ·」。**只标当前连接的那台** —— 没连上的 Mac 问不到状态，替它编一个
  比不显示更糟。
- **「分享与设备」页给一行只读状态**（开启时才出现）：开启时长 · 电源/电量（写明"到 20% 自动
  恢复睡眠"）· 失效或守护缺失的告警。**没有任何按钮** —— 雷达那圈环是"一眼层"，这一行是
  "句子层"，两者都只负责告知。

**为什么不给关闭按钮**（2026-07-31 定，早先版本给过）：这个功能的**每一条管理路径都终结于
「在 Mac 上输一次管理员密码」**。给一个"关得掉但开不回来、还得跑回电脑前"的远程开关，
比不给更让人困惑。能力仍保留在 API 与 client 里（降权随处可发起是安全不变式），只是不做成 UI。

**状态与边界**：无服务器模式 → 没有环 · guest token → 连读都 403，完全看不见 ·
远程开启 → 服务端对任何客户端一律 403（需在 Mac 上授权）。
