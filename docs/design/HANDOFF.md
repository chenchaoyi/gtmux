# 交接给仓库 Claude Code — 整体实现（FINAL · 2026-06）

## 你（用户）怎么操作

1. 用本 `docs/design/` 文件夹**整体替换**旧目录（无需挑选覆盖）。
   > 注：旧目录里你自有的 `multi-agent-multi-terminal.md` / `remote-access-tunnel.md` 若仍需保留，replace 前先备份。
2. 仓库根开 Claude Code，整段粘贴下面「给 CC 的 Prompt」。

---

## 给 CC 的 Prompt（整段粘贴）

请按设计做一次整体实现。先读，以它们为准：

- `docs/design/ITERATIONS-2026-06.md` —— 本轮变更权威清单（菜单栏 A1–A5、移动端 B1–B4、状态语义 C）。
- `docs/design/DESIGN.md`（菜单栏）、`docs/design/MOBILE.md`（移动端）—— 既有规范。
- `docs/design/REVIEW-mobile-01.md` —— 实拍走查待修项。
- 可视参照（浏览器打开，需联网；每个 section 顶部有编号 §）：
  `docs/design/mockup/gtmux-menubar.dc.html`、`gtmux-mobile.dc.html`、`gtmux-web.dc.html`。

**贯穿铁律**（先自查再动任何表面）：
- **「等你输入」= 仅 `waiting`（红）；`working`（蓝）永不等输入。** 结构化 `1/2/3` 回应只挂 waiting。
- 状态三重编码（色 + 形 + 字形）全表面一致；颜色只编码状态、不编码 agent 身份（身份用官方图标 / 中性字标）。
- 中英 + CJK 不换行；连接指示用 **server 名 + 状态点**（已连接绿 / 重连琥珀 / 离线红），不用 “live” 字样。

按顺序实施，每完成一项跟 mockup 对应 § 截图比对：

**菜单栏 app（SwiftUI）**
1. 状态项 = **pane 网格品牌图标**，点亮格随最紧急状态（红方块·双竖线 / 青·加载环 / 绿·对勾）+ 计数；无底色，深/浅/着色栏自适应（§02）。
2. 分区可折叠（Hide/Show + 箭头，记住每类）（§01 行为）。
3. waiting 行就地展开 `1/2/3` 回应（§09）。
4. 快速切换器：needs-you 优先、头像+角标、行内序号、选中 waiting 就地回应（§04）。
5. macOS 系统通知带 1/2/3 动作 + 文字回复，去重 + 自动撤回（§10）。
6. Pair 配对面板（带 logo 的二维码 + 本地/远程）（§11）。

**移动端 app（RN）**
7. Terminal 页（§09–§11 三部曲）：ANSI 彩色 + **滚动锁定**（向上滚不被新输出拽走，↓最新浮标带「新」点）+ **复制**（终端选择 / 代码块复制）+ 双模式审批卡 + **多行输入**（回车换行、↑ 发送、全屏撰写）+ 粘贴/附件 + 图片标注。
8. 通知 / Live Activity / 灵动岛 **不进 app 直接回复**（1/2/3 + 真实 label 分行）（§12–§14）。
9. 设置页按信息架构补全（§15）。
10. **启动 splash**（`tmux × agent` slogan）/ **离线态**（红横幅 + 缓存置灰，不清屏）/ **进 pane 加载态**（骨架 + 转圈 + 慢/失败兜底）（§16）。
11. iPad split-view + 竖屏抽屉 + 指针/Pencil（§06）。

**Web 浏览器镜像**（独立）
12. 一次性可过期链接 + 居中只读雷达 + xterm.js pane 镜像 + 键盘导航（`gtmux-web.dc.html`）。

每项落地后，若与 mockup 有出入，**先报差异再改**，不要擅自偏离。

---

## 落地点速查

- 菜单栏弹层/切换器/通知/Pair/状态项 → macOS app SwiftUI。
- prompt 解析（`❯ N. label`）→ 抽成共享逻辑，菜单栏 + 通知 + 移动端复用。
- 终端渲染/滚动锁定/复制/加载态 → `mobileapp/src/screens/DetailScreen.tsx`。
- 多行输入/粘贴/附件 → `mobileapp/src/ui/Composer.tsx`；图片标注 → `ImageMarkup.tsx`。
- 双模式/审批卡 → DetailScreen + 新增 Chat 视图。
- 设置 → `SettingsScreen.tsx`；splash → 原生 LaunchScreen + RN。
- 配对/Web → `api/contract.md`、`internal/server/`、`PairingScreen.tsx`。
