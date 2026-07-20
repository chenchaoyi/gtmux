# gtmux 设计交接包

放在仓库 `docs/design/`。CLI · 菜单栏 · 手机 · Web 四块屏,共用一套状态语言(色 + 形 + 字形)。

## 怎么用(给仓库 Claude Code)

1. 把本文件夹的内容**叠加覆盖**进仓库 `docs/design/`(只覆盖同名文件,**不要删除**仓库自有的
   `DECISIONS-FOR-CCY.md` / `SECURITY.md` / `multi-agent-multi-terminal.md` / `multiplexer-research.md` /
   `remote-access-tunnel.md` / `RESEARCH-prior-art-2026-06.md`)。
2. 仓库根开 Claude Code,整段粘贴 `HANDOFF.md` 里「给 CC 的 Prompt」。
3. 让它按 `ITERATIONS-2026-06.md` 的清单逐项落地,每项跟 mockup 对应 `§` 截图比对。

## 文件

| 文件 | 用途 |
| --- | --- |
| `HANDOFF.md` | **从这里开始**。给 CC 的整段 Prompt + 落地顺序 + 文件速查。 |
| `DESIGN.md` | 菜单栏权威规范。 |
| `MOBILE.md` | 移动端权威规范(App 图标 / Agent.icon / 交互 / 推送 / 状态)。 |
| `WEB.md` | Web 浏览器镜像权威规范(工作台 / 只读红线 / 对话模式 / 头像 / 键盘)。 |
| `ITERATIONS-2026-06.md` | 本轮所有变更清单(现状→改动→落地点)。 |
| `REVIEW-mobile-01.md` | 实拍走查(P0/P1/P2)。 |
| `CLAUDE.snippet.md` | 合并进仓库根 `CLAUDE.md`,让后续会话持续遵守。 |
| `mockup/gtmux-menubar.dc.html` | 菜单栏可交互原型(终版,§00–§11)。 |
| `mockup/gtmux-mobile.dc.html` | 移动端可交互原型(§01–§16)。 |
| `mockup/gtmux-web.dc.html` | Web 浏览器镜像设计(独立)。 |
| `mockup/{support.js, image-slot.js}` | 原型运行时(与 .dc.html 同目录)。 |
| `mockup/preview-*.png` | 菜单栏静态参照。 |

> 原型用浏览器打开,需联网加载运行时。移动端权威工程蓝图仍是 `mobileapp/SPEC.md` 与 `api/contract.md`。
