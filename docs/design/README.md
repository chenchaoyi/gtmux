# gtmux 菜单栏 app — 设计交接包

放在仓库 `docs/design/` 下。

## 文件

| 文件 | 用途 |
| --- | --- |
| `DESIGN.md` | **权威设计规范**。所有菜单栏 app UI 以它为准。 |
| `HANDOVER.md` | 交接给 Claude Code 的起步 Prompt（含实现顺序、约束、验收、待决项）。 |
| `CLAUDE.snippet.md` | 合并进仓库根 `CLAUDE.md` 的片段，让后续会话持续遵守规范。 |
| `mockup/gtmux-menubar.dc.html` | 可交互高保真原型（浏览器打开；运行时从 CDN 加载，需联网）。可切换 0/1/5/15+ 场景、深浅色、中英、↑↓⏎ 导航、搜索。 |
| `mockup/support.js` | 原型运行时（与 .dc.html 同目录即可）。 |
| `mockup/preview-popover.png` | 静态参照：popover（深色、5 agent）。 |
| `mockup/preview-iconset.png` | 静态参照：状态栏字形概念 + token。 |
| `mockup/preview-firstrun.png` | 静态参照：空状态 + 首次运行权限卡。 |
| `mockup/preview-preferences.png` | 静态参照：偏好设置（含语言三态）。 |

## 怎么用

1. 读 `DESIGN.md`。
2. 把 `HANDOVER.md` 整段贴给仓库里的 Claude Code 起步。
3. 把 `CLAUDE.snippet.md` 合并进根 `CLAUDE.md`，后续都遵守。

> 原型用浏览器打开 `mockup/gtmux-menubar.dc.html` 即可（需联网加载 React 运行时）；
> 不便联网时看 `mockup/preview-*.png`。
