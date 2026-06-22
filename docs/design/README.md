# gtmux — 设计交接包（菜单栏 + 移动端）

放在仓库 `docs/design/` 下。CLI · 菜单栏 · 手机三块屏共用一套状态语言。

## 从哪里开始

**把 `HANDOVER.md` 整段贴给仓库里的 Claude Code** —— 它是落地**两块屏**的总入口
（索引 + 实现顺序 + 验收 + 已定/待决 + 持续约束）。然后把 `CLAUDE.snippet.md` 合并进根
`CLAUDE.md`，让后续会话持续遵守。需要看原型时打开 `mockup/` 下对应 `.dc.html`。

## 通用 / 入口

| 文件 | 用途 |
| --- | --- |
| `HANDOVER.md` | **总交接**（贴给 cc 的起步 Prompt）：覆盖菜单栏 + 移动端的阅读顺序、实现顺序、验收、已定/待决、约束。 |
| `CLAUDE.snippet.md` | 合并进根 `CLAUDE.md` 的片段（两块屏通用规范）。 |

## 菜单栏 app

| 文件 | 用途 |
| --- | --- |
| `DESIGN.md` | **菜单栏权威设计规范**。所有菜单栏 UI 以它为准。 |
| `mockup/gtmux-menubar.dc.html` | 可交互高保真原型（浏览器打开；运行时从 CDN 加载，需联网）。可切 0/1/5/15+、深浅色、中英、↑↓⏎、搜索。 |
| `mockup/support.js` | 原型运行时（与 .dc.html 同目录即可）。 |
| `mockup/preview-popover.png` | 静态参照：popover（深色、5 agent）。 |
| `mockup/preview-iconset.png` | 静态参照：状态栏字形概念 + token。 |
| `mockup/preview-firstrun.png` | 静态参照：空状态 + 首次运行权限卡。 |
| `mockup/preview-preferences.png` | 静态参照：偏好设置（含语言三态）。 |

## 移动端 app（第三块屏）

| 文件 | 用途 |
| --- | --- |
| `MOBILE.md` | **移动端权威设计补充**：App 图标 + Agent.icon 头像 + Radar/Detail 交互 + 推送 + 状态。配合 `mobileapp/SPEC.md`、`api/contract.md`。 |
| `mockup/gtmux-mobile.dc.html` | 可交互原型（需联网）。四屏全流程 + 推送 banner + App 图标 + Agent 图标槽 + 折叠 + 彩色终端 + composer/语音 + **iPad 分栏**。 |
| `mockup/image-slot.js` | 图标槽组件（与 mobile .dc.html 同目录）。 |

## 怎么用

1. 把 **`HANDOVER.md`** 整段贴给仓库里的 Claude Code 起步（它会按需读 `DESIGN.md` / `MOBILE.md` / 原型 / 代码）。
2. 把 `CLAUDE.snippet.md` 合并进根 `CLAUDE.md`，后续都遵守。
3. 看原型时打开 `mockup/` 下对应 `.dc.html`（需联网加载 React 运行时）。

> 移动端权威工程蓝图在 `mobileapp/SPEC.md` 与 `api/contract.md`；状态色权威在 `internal/menubar/icon.go`。
