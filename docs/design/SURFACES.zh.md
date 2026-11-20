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
| agent 详情模式 | 对话 / 终端是模式，轨迹 / Diff 是独立工具；默认终端；普通 pane 没有 agent 对话与轨迹模式 | `web/index.html`、`web/app.js` 聚焦与 tile、`DetailScreen.tsx`；修复 Web 顺序和普通 pane 对模式偏好的覆盖 |
| 等待、工作、空闲、普通进程 | 需要你、运行中、空闲、待命，使用同一核心状态与状态色 | `MenuView.swift`、`CommandPalette.swift`、手机 i18n、Web LABEL；修复 Mac 等待分组叫法和 Web 状态读屏名称 |
| 分组展开与折叠 | 折叠全部 / 展开全部 | Web `app.js`、手机 `PaneBrowserView` 与 `ChatView`；修复 Web 用词 |
| 知识库与线路入口 | 知识库用书，线路用地球 | `HQResourcePage.symbol`、手机 `SIcon` 与 `HQDisc`；已一致，无需改图标 |
| 配对设备身份 | 自定义名称原样显示；客户端、地址、连接时间依次展示；缺失信息明确说明 | `PairStore.swift`、`ManageMacScreen.tsx`、`/api/devices`；修复 Browser 大小写、信息与图标优先级、名称清理 |
| 只读与输入权限 | 核心执行自有设备与共享权限，只读界面不暗示可以发送 | Web 输入区域、手机 DetailView、API 契约；保留各平台原生控件 |

菜单栏直接打开本地 pane，不嵌入对话与终端视图。Web 的 Diff、工作台 tile 和浏览器本地
布局存储是其独有能力。配对管理使用 Mac 主凭据，手机与 iPad 的设备列表只读。
这些属于有意保留的平台差异。

回归检查覆盖中英文 tile 顺序、普通 pane 能力与模式偏好保留、自定义名称、配对与重连
客户端信息，以及权限边界。上述检查不包含 iPhone/iPad 真机布局与 VoiceOver 实际语音验收。

## 主机平台能力

共享核心支持 macOS 和 Linux，远程服务器不一定是 Mac。连接、配对和错误提示使用“服务器”；真实 Apple 设备名和 macOS 专属控制仍保留 Mac／macOS。桌面跳转、桌面通知、菜单栏和合盖控制以 `GET /api/host.capabilities` 为准，不从 agent 或终端名推断。明确为 false 时移除对应操作，缺字段时保留旧版呈现。Linux 用户可在 App、网页、终端管理会话，Mac 菜单栏仍操作本机。

服务管理、主机信息、资源采样和桌面交互分别由平台模块实现；注册、访问规则、pane 身份和 HQ 行为留在共享核心。FreeBSD 仅为后续扩展目标，尚未纳入支持平台。

## Android 手机和平板

Android 复用手机与常规平板这两种界面形态，不另建业务模型或复制 UI。原生适配与交付约定见 `mobileapp/ANDROID.zh.md`。会话/HQ 状态、Chat、Terminal、Trace、Diff、代码排版、主题、配对和访客范围仍使用同一套 React Native/Go 实现。Kotlin 适配文件与系统分享、键盘/触感、可选 FCM，以及不含凭据的上次用量桌面组件。文件阅读样式由同一份源文件生成给 UIKit/WebKit 和 Android WebView，解析器与传输保持各平台原生适配。

Android 验收独立记账：捆绑 JS 的 QA 构建、自有模拟器真实操作、手机/平板窗口变化、返回键/输入法、权限、TalkBack、真机、真实 FCM 和商店上架分别保留证据。缺少 Google 配置或服务时应明确提示，同时允许正常配对使用。不能将 iOS 云备份或 Live Activity 的说明直接套给 Android。

## 品牌加载一致性

App（LoadingMark/BrandLoader）、Web（brandLoaderEl）与菜单栏
（BrandProgressMark）共用同一动作含义：100单位图形内的三个区块固定为
(8,8,40,40)、(52,8,40,40)、(8,52,84,40)，圆角9。品牌青色#06B6D4
按右上→下方→左上循环，周期1.8秒；每阶段停留390毫秒、渐变210毫秒，
只改变高亮透明度。减弱动态效果时固定右上高亮；加载提示只读一次，装饰块
不进入无障碍树。原生组件在非活动或卸载时停止，浏览器在读取或页面退役后
移除加载组件。系统进度控件、静态身份图与Agent工作圆环保留各自语义。

以后修改loading必须核对这三份实现及手机/iPad共用组件，并说明终端/attach
是否有对应展示；在双主题、窄屏及宽屏比较实际渲染阶段。源码检查或构建成功
不能代替视觉核对，无障碍标签也不等于读屏语音验收。

## 组件验收

共用规则的唯一定义见 [DESIGN 现行规则](DESIGN.zh.md#现行共用规则2026-11-17)。
每次变更都必须按下表记录实际结果、证据路径和未测项；列出平台名称不算完成验收。
状态维度至少包含正常、加载、空态、失败/重试、禁用；适用时另含 queued/保存与等待/工作/空闲。
亮暗主题都检查，动态组件另检查减少动态效果。此表是责任与入口，不代表尚未执行的检查已通过。

| 组件 / 规则 | Phone / iPad | Web 精确/粗指针 | Mac | 终端 |
| --- | --- | --- | --- | --- |
| 辅助文字 TEXT-READ | theme/SettingsRow/AgentRow；计算与组件 + 原生大字布局 | 实际 CSS 与截图；亮/暗、选中 | Theme 与实际材质/壁纸 | 系统终端调色板；不套用 App token |
| 输入/附件 TOUCH-44、UPLOAD-PHASE | Composer；独立编辑/移除与实际字节阶段 | composer；桌面/390px、XHR 阶段与重试 | 原生截图发送流程单列 | CLI 进度走文本/实际字节 |
| 加载 MOTION-LOAD | LoadingMark；手机/平板共享，前后台与减少动态效果 | brandload；页面/inline/send、减少动态效果 | GtmuxLoadingMark；关闭窗口后停止；系统控件例外 | 文本状态，无动画 logo |
| HQ / 会话 ROLE-HQ、A11Y-CONTEXT | HQ 独立；开关/同名会话语义；实际读屏另验 | 两种布局保持 HQ 分组；键盘与标签 | 下格状态符号灰度；菜单栏材质另验 | 语义状态与位置 |
| 文件 FILE-SCOPE | 真实 scope、不可用/权限/网络；新链接恢复 | 同源生成校验/文案 | 本地文档与系统预览边界 | add 成功后仍需接收端 scope 读取验证 |

每轮交付在项目报告中用相同规则 ID 登记：源码绑定、测试结果、截图、设备/实际朗读缺口。
