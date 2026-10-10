# gtmux 的五种形态：每次迭代都要过一遍

gtmux 是一个产品、五种形态，共用同一个 Go 核心（`internal/`，单一数据源）和同一套状态语言
（waiting 红 / working 青 / idle 绿 / running 灰，色 = 状态）。2026-09-12 起的规矩：**任何改动用户可见行为
的变更，提案里必须逐条说明它对五种形态各是什么**：做了、不适用（写明为什么）、或者留给哪个后续变更。
`scripts/check-design.sh` 检查在途提案里有这一节且五个名字都出现；缺一个就红。

| 形态 | 在哪 | 是什么 | 权威设计 |
|---|---|---|---|
| 终端 | `cmd/gtmux` + `internal/`；`gtmux attach` 走 `GET /api/attach` | CLI 本身，以及远程 attach：把远端 tmux pane 的 PTY 桥到本地终端（本人或已配对设备；分享链接会被拒绝） | `docs/cli.md`、`docs/design/remote-attach-research.md` |
| 菜单栏 | `macapp/` | 原生 Swift，`agents --json` 的纯消费者；通知的点击目标 | `docs/design/DESIGN.md` |
| 手机 | `mobileapp/`，compact 壳 | iPhone：雷达 → 详情 → HQ 的堆叠导航；推送；终端输入 | `docs/design/MOBILE.md` |
| iPad | `mobileapp/`，regular 壳 | 同一个 app 的侧栏 + 主区形态；硬件键盘、指针、多任务窗口 | `docs/design/MOBILE.md` §5，change `ipad-universal-app` |
| Web | serve 的共享页 / 配对页 | 浏览器镜像；主人或获准的访客可在授权 pane 输入 | `docs/design/WEB.md` |

## 为什么要写成规矩

iPad 的分栏 2026-07 就做了一半，首次上架时延后，之后两个月里雷达的每次改动都只落在手机上：
浮窗 HQ 圆盘、出错分区、访客横幅、长按菜单。没有人决定跳过 iPad，只是没有一步要求回答
「iPad 上这个怎么样」，不在清单上的形态就这样悄悄偏掉了。Demo 模式也曾两次落后于真实雷达，原因一样。

## 怎么防偏移：先靠结构，再靠清单

防偏移靠两样：上面那一节提案清单挡遗忘，下面四条结构规则挡复制：

1. 一份实现，多种呈现。同一件事在两种形态里出现，就是同一段代码加一个 prop；出现第二份代码就是缺陷。
   手机与 iPad：`RadarPanel` 两种形态、`DetailView` / `HQView` / `PaneBrowserView` 带 `layout`；
   `shellDrift.test.ts` 读源码守着。菜单栏与手机共享的是数据契约（`agents --json`、`/api/digest`），
   不共享代码，所以 `DESIGN.md` 和 `MOBILE.md` 各自有一节对照状态语言。
2. 开东西的动作不认识形态。「打开某个 pane / HQ / 所有 pane」是工作区状态（`WorkspaceContext`），
   壳决定翻译成导航还是主区切换；推送深链、键盘、按钮都只改状态。
3. Demo 就是真壳套假数据。演示模式渲染的是同一套壳（compact / regular），差的只有 client；
   它不允许有自己的雷达。
4. 形态间的边界写在提案里。一个改动只影响一种形态是常态（例如菜单栏的弹窗），但「不适用」要说出来，
   让评审能反对。

## 提案里那一节长什么样

```
## Surfaces

- 终端 (terminal / attach)：不适用 —— 这是 UI 排布的改动，CLI 没有对应物。
- 菜单栏 (menubar)：不适用 —— 菜单栏没有 HQ 对话区。
- 手机 (phone)：不变，compact 壳一字不动。
- iPad：本变更的主体。
- Web：不适用 —— 共享页不显示 HQ。
```

五个名字（终端 / 菜单栏 / 手机 / iPad / Web，或英文 terminal / menubar / phone / iPad / web）都得出现；
门禁只查名字在不在、判断对不对留给评审。

## 对应交互检查（2026-10-10）

本轮检查以下共用交互，不代表所有页面均完成真机验收。原生布局与权限边界按平台保留。

| 交互 | 统一的含义与顺序 | 证据与结果 |
|---|---|---|
| agent 详情模式 | 对话 → 终端；默认终端；普通 pane 没有对话模式 | `web/index.html`、`web/app.js` 聚焦与 tile、`DetailScreen.tsx`；修复 Web 顺序和普通 pane 对模式偏好的覆盖 |
| 等待、工作、空闲、普通进程 | 需要你、运行中、空闲、待命，使用同一核心状态与状态色 | `MenuView.swift`、`CommandPalette.swift`、手机 i18n、Web LABEL；修复 Mac 等待分组叫法和 Web 状态读屏名称 |
| 分组展开与折叠 | 折叠全部 / 展开全部 | Web `app.js`、手机 `PaneBrowserView` 与 `ChatView`；修复 Web 用词 |
| 知识库与线路入口 | 知识库用书，线路用地球 | `HQReaderTab.symbol`、手机 `SIcon` 与 `HQDisc`；已一致，无需改图标 |
| 配对设备身份 | 自定义名称原样显示；客户端、地址、连接时间依次展示；缺失信息明确说明 | `PairStore.swift`、`ManageMacScreen.tsx`、`/api/devices`；修复 Browser 大小写、信息与图标优先级、名称清理 |
| 只读与输入权限 | 核心执行自有设备与共享权限，只读界面不暗示可以发送 | Web 输入区域、手机 DetailView、API 契约；保留各平台原生控件 |

菜单栏直接打开本地 pane，不嵌入对话与终端视图。Web 的 Diff、工作台 tile 和浏览器本地
布局存储是其独有能力。配对管理使用 Mac 主凭据，手机与 iPad 的设备列表只读。
这些属于有意保留的平台差异。

回归检查覆盖中英文 tile 顺序、普通 pane 能力与模式偏好保留、自定义名称、配对与重连
客户端信息，以及权限边界。上述检查不包含 iPhone/iPad 真机布局与 VoiceOver 实际语音验收。
