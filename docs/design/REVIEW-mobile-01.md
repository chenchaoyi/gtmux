# gtmux 移动端 — 设计 Review #01（实拍 vs 设计规范）

> 基于 v0.9.3 实拍截图的设计走查。权威规范见 `docs/design/MOBILE.md` 与 `DESIGN.md`。
> 按严重度排序，repo 的 Claude Code 可照单改。P0 优先。

整体评价：实现**强且超出规范**——终端字体选择、xterm 渲染器、多 server、git diff、
tunnel「Open on computer」都是加分项。以下是需要修/打磨的点。

---

## 🔴 P0 必须修

### 1. Pairing 页「Cancel」与状态栏时钟重叠
模态页 `Add a server` 左上角 `‹ Cancel` 压在状态栏 `09:41` 上——自定义导航栏没有
respect 顶部 safe-area inset（Radar/Settings/Detail 都正确避让了，只有这个模态没有）。

- **改法**：Pairing（模态）的 header 包 `SafeAreaView`，或顶部留 `useSafeAreaInsets().top`，
  让 `Cancel` 落到状态栏**下方**。

---

## 🟠 P1 体验问题

### 2. 经典渲染器下终端几乎单色灰、对比度低
Detail 的输出大面积暗灰压近黑底，`+16 lines` / `ctrl+o to expand` 等 dim 行几乎读不清。
设计要求「像 Terminal.app 一样有色」。

- **改法**：(a) dim/comment 灰阶提一档（≈ `#8A9099`）保证暗行可读；
  (b) 考虑 **xterm 渲染器默认开启**（彩色 + CJK 宽度是核心体验，现为 beta 关闭）；
  (c) 经典渲染器至少把 ANSI 基础 8 色映射到 `theme.ts` 终端色板，而非整体降灰。

### 3. Detail 顶部「快捷操作条」右侧被裁切
键盘上方 `Paste · Continue · Stop · ⏎ · …` 最右按钮被屏幕边缘切掉半个。

- **改法**：这排改 `ScrollView horizontal`（或 `flex-wrap`），任何语言/按钮数都不被裁。
  这正是设计里 composer「控制键排」要求的横滑行为。

### 4. Radar 所有 agent 头像是同一个橙色 sunburst
本例数据恰好都是 Claude Code 所以可接受，但要确认**非 Claude agent 的回退路径**生效。

- **改法**：用混合 agent 类型（Codex/Gemini/aider）的数据集跑一遍，确认 `Agent.icon` 缺省时
  回退到**中性字标**（CC/Cx/G/Ai…，见 MOBILE.md §2），不会全部塌成同一图标。
  颜色只编码状态，不要给 agent 上色。

---

## 🟡 P2 细节打磨（可选）

### 5. Radar 主标题从 gtmux 品牌变成 server 名「debug」
为多 server 合理，但主屏失去 gtmux 品牌露出。

- **建议**：server 名旁/下补一个很小的 gtmux pane-grid mark，或把 wordmark 放进切换菜单。

### 6. 「Waiting only」在 0 waiting 时仍可点
点了会进空列表。

- **建议**：0 waiting 时该 pill 置灰或显计数 `Waiting 0`，避免点进空态。

### 7. Pairing 文案 serve + tunnel 并列 → 产品决策点
`Run gtmux serve (or gtmux tunnel)…`。若要把 serve/tunnel 合并为「一个连接、用户可选模式」，
Pairing 页正好加一个 segmented：**局域网 serve / 远程 tunnel（Pro 🔒）**。
（tunnel 付费解锁是独立的产品/设计议题，需要时单独出一版 Pairing + 付费墙设计。）

---

## ✅ 符合设计、做得好的

- 折叠分区：计数气泡 + Hide + 圆形箭头 + 分区分隔槽 —— 完全按「可发现折叠」落地。
- 状态徽章三重编码：working 青圆加载环 / idle 绿圆对勾 —— 一致。
- Detail 控制条 `Diff · A− · A+ · Wrap/Scroll · 全屏` + 全屏 Exit + git diff 视图 —— 齐全且超预期。
- Settings 信息架构（语言三态 / 终端字体 / server / 推送 / xterm）清晰；`Open on computer` tunnel 入口很赞。
- Servers 多 server 管理 + 绿点已连接 + Disconnect —— 干净。

---

## 修复优先级清单

| # | 严重度 | 一句话 |
|---|---|---|
| 1 | P0 | Pairing 模态 header 避让状态栏 safe-area |
| 2 | P1 | 终端经典渲染器提对比度 / xterm 默认开 / ANSI 上色 |
| 3 | P1 | composer 控制键排横向可滚动，不裁切 |
| 4 | P1 | 验证非 Claude agent 头像回退中性字标 |
| 5 | P2 | Radar 主屏补 gtmux 品牌锚点 |
| 6 | P2 | 0 waiting 时禁用/标注 Waiting only |
| 7 | P2 | Pairing serve/tunnel 合并 + tunnel 付费（产品决策） |
