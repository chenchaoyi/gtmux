# 功能图标清单 · 2026-10-10

[English](ICON-PARITY-2026-10-10.md)

| 功能 | Mac 菜单栏 | iPhone / iPad | 网页 / 终端 | 处理结果 |
|---|---|---|---|---|
| 知识库 | HQ 快捷入口：SF `book` | HQ 菜单：`SIcon knowledge`，打开的书本 | 网页无知识库快捷入口；CLI 用文字 | 将 Mac 菱形改为书本 |
| 态势板 | SF `doc.plaintext`，报告行用名称 | HQ 行用名称 | 无对应阅读快捷入口 | 保留文档含义，不给文字行加装饰 |
| 用量 | HQ 报告行用名称 | HQ 菜单用柱状图 | CLI 用名称；网页无用量快捷入口 | 含义一致，不新增装饰图标 |
| 网络线路 | 偏好设置、配对页用地区名称 | 设置页：`SIcon globe` | CLI 用名称 | 手机改用地球；Mac 身份仍用服务器图标 |
| 所有窗格 | SF `rectangle.split.2x2` | `PanesIcon`，分栏窗口 | 网页分栏窗口 SVG | 将网页的文档字符改为窗口 |
| 新建会话 | 加号 | `NewSessionIcon`，终端外框加号 | CLI 命令；网页无创建入口 | 相同的创建含义 |
| 通用设置 | SF `gearshape` | `SettingsIcon`，齿轮 | 网页 `Aa` 仅设置字体和字号 | 范围不同，保留不同图标 |
| 通知 | 铃铛 | `SIcon bell` / `bellOff` | CLI 用名称 | 含义一致 |
| 语言 | 地球 | `SIcon globe` | 跟随浏览器语言，无选择器 | 有选择器的端保持一致 |
| gtmux 品牌 | 右上青色窗格，底部整宽窗格 | App、HQ 共用标志；widget 用 BrandIcon 资源 | 网页窗格标志；CLI 用文字 | 保留现有品牌结构 |
| Agent 身份 | 注册表、资源中的官方图标 | 手机与 iPad 共用 `AgentAvatar` | 网页图标接口 | 不增加新的 agent 图案 |

## 依据与验证

- Mac：`macapp/` 下的 `MenuView.swift`、`HQReader.swift`、`Preferences.swift`、`RemoteAccessControls.swift`、`Theme.swift`、`BrandMarkTests.swift`。
- 手机/iPad：共用的 `SettingsScreen.tsx`、`HQDisc.tsx`、`SettingsIcons.tsx`、`SettingsIcon.tsx`、`Icons.tsx`、`BrandMark.tsx`、`AgentAvatar.tsx`；widget 的 `GtmuxWidget.swift` / `BrandIcon.imageset`。
- 网页：`internal/server/web/index.html`、`app.js`、`style.css`；官方 agent 图案来源：`assets/agent-icons/SOURCES.md`。

本记录是源码清单与自动化检查，不代表 iPhone/iPad 真机布局或 VoiceOver 已验收。纯文字入口无需新增图标。各端可保留原生笔画和细节，但视觉含义相同。

PR 中记录验证结果：原生 SF Symbols 可用性、阅读器测试、手机线路与 HQ 快捷入口检查、网页中英文无障碍名称检查，以及仓库检查。
