<!-- 把本段合并进仓库根 CLAUDE.md，确保后续会话遵守 gtmux 设计规范（菜单栏 + 移动端） -->

## gtmux 设计规范（必读）

gtmux 是一个产品、三块屏（CLI · 菜单栏 · 手机），共用同一套状态语言。改动**菜单栏 app**
（`NSStatusItem + NSPopover + SwiftUI`）先读 `docs/design/DESIGN.md`；改动**移动端 app**
（bare React Native）先读 `docs/design/MOBILE.md`；落地总入口见 `docs/design/HANDOFF.md`，
可视参照在 `docs/design/mockup/`。要点：

- **「等你输入」= 仅 `waiting`（红）；`working`（蓝）永不等输入。** 结构化 `1/2/3` 回应只挂 waiting。

- **状态语言三重编码**（色+形+字形），全表面统一：waiting=红方块·双竖线 / working=青圆·静态加载环 /
  idle=绿圆·✓ / running=灰圆·点。颜色**只**表达状态，状态色用 `internal/menubar/icon.go` 的权威值。
- **层级**：waiting 响、idle 静。分区顺序 needs-you→working→idle→running。
- **agent 身份**用中性单字标，官方图标走 `agents.json` 的 `icon` 字段；**不在代码里绘制第三方商标**。
- **双语** en/zh（跟随 `GTMUX_LANG` / 设备语言），CJK 不换行用省略号；语言三态（跟随系统/EN/中文）即时生效。
- **动效最小**：只允许 idle→waiting 一次脉冲；加载环不旋转；空闲零动画。
- **视觉克制**：无彩虹渐变、无彩色发光阴影；文案平实、禁止营销腔（尤其权限/首次运行卡）。
- **支持原生终端**（无 tmux）：行与跳转按 `source: tmux|native` 泛化（DESIGN §7 / MOBILE §2）。
- **连接指示**用 server 名 + 状态点（已连接绿 / 重连琥珀 / 离线红），不用 “live” 字样；离线不清屏、留缓存置灰。
- **移动端**：远程只读 MVP（监控 + focus + 推送）；终端输入是 Phase 2，UI 就位但写入门控置灰。
- **命名**统一小写 `gtmux`。与 `DESIGN.md` / `MOBILE.md` 冲突的改动，先提出、不擅自偏离；改了设计就同步更新规范。
