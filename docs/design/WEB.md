# Web 浏览器镜像 — 工作台版（WEB.md）

> 浏览器镜像的权威设计补充。可视参照 `mockup/gtmux-web.dc.html`（§01–§04）。
> 实现入口 `internal/server/web/`（`index.html` / `app.js` / `style.css`）。底层全部复用现有
> 契约（`/api/agents · /api/pane · /api/transcript · /api/diff · /api/icon · SSE`），**无新后端**。

## 定位

浏览器屏大、有真键鼠 —— **不照搬手机单列**，做**桌面工作台**。左侧 session/window/pane 目录，
把任意 pane 拖到画板自由排列、缩放，像 tmux 那样自己组合视图。**仍是只读镜像。**

## 红线：只读

真 tmux 能 split/kill/spawn，镜像**不假装**。「编排窗口与 pane」= 用户**自己的观察布局**，
不改动真实 tmux 树。输入仍回手机/Mac。

## 1. 顶栏

gtmux logo · **布局预设**下拉（Frontend trio…）· **贴齐网格**开关 · **自动浮出 waiting** 开关 ·
连接指示（server 名 + 状态点，不用 “live”）· 外观（Aa：字体/字号，沿用现有 settings）。

## 2. 左侧目录（session/window/pane 树）

- 按状态分组 needs-you→working→idle；window 可展开到 pane；顶部搜索过滤。
- **拖 pane → 画板**；双击 → 全屏。
- **可收起**：头部 ⇤ 收入；收起后画板左缘留带 waiting 计数的小标签（⇥）可重开。
- **可调宽**：右缘 col-resize 拖柄，记住宽度（localStorage）。窄屏（<900px）自动收起。

## 3. 自由画板

- 每块 tile = 一个 pane 的**实时 xterm 镜像**（复用现有 `app.js` 的 xterm 写入 + 滚动锁定）。
- tile：拖标题移动、拖右下角缩放、可叠放/平铺；可选**贴齐网格**对齐。
- tile 头部：avatar + 角标状态徽章 · 名称 · `终端 / 对话 / diff` 切换 · ⤢ 全屏 · × 关闭。
- **waiting tile** 红边 + 轻脉冲。
- 多 pane = 多个并发 `/api/pane?id` 挂载；`diff` 用 `/api/diff?id`；`对话` 用 `/api/transcript?id`。
- **缩放某块 tile，其余自适应回流**（非自由叠放模式下用网格/弹性布局）；**单击 tile 即最大化聚焦**，再点/Esc 复原。

## 4. 全屏聚焦（单 pane 细读，mockup §02）

双击 tile / ⤢ / 单击最大化 → 一块 pane 占满，给最大阅读面积 + 完整工具条（终端/对话/diff、A−/A+、换行/滚动、复制可见屏/回滚缓冲、跳到最新）。Esc 回画板。

## 5. 对话模式 · 宽屏版（mockup §03）

与移动端 `ChatView` 同源（`/api/transcript`：prompt → 中间步骤折叠 → agent 回复），为宽屏重排：
- **左侧轮次目录**：列出每个 turn，`j`/`k` 跳转、当前轮高亮（大屏独有的全局导航）。
- **居中对话列**（~680px 易读宽）：用户气泡靠右 + 人类头像；agent 气泡靠左 + 官方图标；**气泡悬停浮出「复制 / 引用」**（桌面鼠标特性）。
- **审批卡**：waiting 时整行大按钮 `1/2/3`（真实 label），点一下即 `/api/send`，与菜单栏/通知同源。
- 折叠步骤；底部多行 composer（⏎ 发送、⌥⏎/⤓ 换行）。对话面始终深色。

## 6. 你的头像 · agent 时代的人类（mockup §03 附）

人类在对话里的头像。默认 **人形电池**（人在电池里供电——你以为在用它，其实在喂它），统一青色渐变底，与品牌一致、与 agent 头像区分（agent 用官方图标/方形，人用渐变圆）。设置里可换其余款（飞升 / 指挥家 / 拍板人 / 队长 / 休息中 / 橡皮图章 / 遛狗反转 / 仓鼠轮），或上传照片 / 选 emoji / 用首字母。颜色只是品牌色，不编码身份。三屏统一替换现有 `UserAvatar`。

## 7. 建议新增能力

- **保存布局/预设**：命名布局（哪些 pane、位置、大小）存 localStorage，顶栏切换；重开链接即恢复。
- **自动浮出 waiting**（可选）：SSE `alert kind:"waiting"` 时该 pane 自动上画板并脉冲 → 画板即雷达。
- **专注模式**：双击 tile / ⤢ → 全屏单 pane（细读/滚历史/看 diff）；Esc 回画板（等于现有单列视图）。

## 8. 键盘（桌面优先）

`⌘K` 命令面板调 pane · `1–9` 聚焦第 N 块 · `f` 全屏 · `Esc` 退出全屏 · `g` 贴齐网格 ·
`[ ]` 切布局预设 · `/` 搜目录。

## 9. 状态 / 落地

- 状态语言与三屏一致（色+形+字形）；离线 tile 置灰、不清屏。
- 实现：`web/app.js` 加一个 **board 布局引擎**（绝对定位 + 拖拽/缩放 + localStorage 持久化），
  tile 复用现有 xterm 逻辑；响应式回退：窄屏自动回到现有单列 radar→pane。
- 渐进式：先并发多 pane + 拖拽缩放 + 收起/调宽，再加预设/自动浮出/专注。
