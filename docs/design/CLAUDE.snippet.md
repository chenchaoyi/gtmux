<!-- 把本段合并进仓库根 CLAUDE.md，确保后续会话遵守菜单栏 app 设计规范 -->

## 菜单栏 app 设计规范（必读）

实现/改动 macOS 菜单栏 app（`NSStatusItem + NSPopover + SwiftUI`）的任何 UI 时，**先读并遵守
`docs/design/DESIGN.md`**（权威设计规范），参照 `docs/design/mockup/`。要点：

- **状态语言三重编码**（色+形+字形），全表面统一：waiting=红方块·双竖线 / working=青圆·静态加载环 /
  idle=绿圆·✓ / running=灰圆·点。颜色**只**表达状态，状态色用 `internal/menubar/icon.go` 的权威值。
- **层级**：waiting 响、idle 静。分区顺序 needs-you→working→idle→running。
- **agent 身份**用中性单字标，官方图标走 `agents.json` 的 `icon` 字段；**不在代码里绘制第三方商标**。
- **双语** en/zh（跟随 `GTMUX_LANG`），CJK 不换行用省略号；偏好语言三态、即时生效。
- **动效最小**：只允许 idle→waiting 一次脉冲；加载环不旋转；空闲零动画。
- **视觉克制**：无彩虹渐变、无彩色发光阴影；文案平实、禁止营销腔（尤其首次运行权限卡）。
- **支持原生终端**（无 tmux）：行与跳转按 `source: tmux|native` 泛化（DESIGN §7）。
- 与 DESIGN.md 冲突的改动，先提出、不擅自偏离。
