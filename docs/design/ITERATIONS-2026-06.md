# gtmux 设计迭代汇总 — 2026-06

> 本轮设计迭代的**权威变更清单**。菜单栏对应 `gtmux 菜单栏设计.dc.html`(= `mockup/gtmux-menubar.dc.html`),
> 移动端对应 `gtmux 移动端设计.dc.html`(= `mockup/gtmux-mobile.dc.html`)。每条含「现状 → 改动 → 落地点」。
> 贯穿铁律见末尾「状态语义」。

---

## A. 菜单栏 app(DESIGN.md 的增补)

### A1 · 弹层内直接回应(就地批准) — §09
- 现状:弹层只能**跳转**到 waiting 的 agent。
- 改动:waiting 行**就地展开**,把 Claude 的真实选项做成整行大按钮(`1 Yes` / `2 Yes, and don't ask again` /
  `3 No, tell Claude what to do`,数字键 + 原始 label,**分行**)。点一下即发送,不必切到终端。
- 交互:默认折叠保持紧凑;选中/hover 出「⤺ 回应」,点它或 `→` 展开;展开后 `1/2/3` 直接发、`⏎` 跳转、
  `←`/`⎋` 收起;发送后行乐观更新为 working,不关弹层。仅 waiting 且能解析出选项时出现,否则回退「跳转去回应」。
- 落地点:popover SwiftUI 的 waiting row;prompt 解析复用移动端逻辑;发送走 `gtmux send <pane> "1"`(写,沿用自动化授权)。

### A2 · macOS 系统通知(带动作) — §10
- 现状:菜单栏从未 spec 系统通知。
- 改动:补 **macOS 通知**两类——`waiting` 带 `1/2/3` 动作按钮 + 行内文字回复(点按钮即 `send`,无需开 app);
  `done` 紧凑无动作、点击打开弹层并选中该行。
- 规则:`UNUserNotificationCenter` + `UNNotificationAction`(承载 1/2/3)+ `UNTextInputNotificationAction`(自定义回复);
  waiting 设 time-sensitive 穿透专注模式,done 普通级;通知去重(同 pane 状态没变不重复),解决后自动撤回;
  偏好新增「动作按钮」「勿扰时段」。
- 落地点:新增通知模块;与 A1/移动端共用同一套 prompt 解析 + 文案。

### A3 · 快速切换器(独立命令面板)迭代 — §04 迭代版
- 现状(实拍 v0.10.0):① 以 COMPLETED 打头;② 图标全是同一红 Claude 图标 + 绿勾,状态/身份不分;
  ③ 底栏写 `⌘1–9 direct` 但行上无序号;④ 选中 waiting 不能就地回应。
- 改动:① 排序翻正 **needs-you → working → completed**;② 改 **头像 + 角标状态徽章**(身份与状态分离);
  ③ 行左加 **1–9 序号**(呼应 `⌘1–9`);④ 选中 waiting **就地展开 1/2/3 + ⏎ jump**(与 A1/A2 同源)。
  保留实拍优点:主行用 task,副行补 `session · agent · pane`。底栏加 `1/2/3 回应`。
- 落地点:命令面板的分区排序、行视图、键盘处理。

### A4 · 弹层分区可折叠
- 现状:弹层分区(NEEDS YOU / WORKING / IDLE)不能折叠,但移动端雷达能。
- 改动:每个分区头变可点折叠条——右侧 **Hide/Show(收起/展开)文字 + 圆形旋转箭头**(展开 ▼、折叠 ▶),
  折叠后**计数常驻**,下面分区上移;**记住每类折叠态**;深浅色 + 中英。
- 落地点:popover section header → 可点 + 持久化折叠状态。

### A5 · Pair(让手机连这台 Mac) — §11
- 现状(实拍 v0.10.0):底栏多了 ▦ Pair,但未在设计里。
- 改动:底栏正式成 **5 项**:Overview · Watch · Restore · New · **Pair**(二维码网格图标,排最右)。
  点 Pair 弹**配对面板**:顶部「本地网络 / 远程·Pro」切换 + **二维码** + 可复制地址/token。
  二维码 schema v1 `{v,url,token,name}` 与移动端 PairingScreen 解析对齐(扫码 → 校验 v===1 → health → Keychain → 雷达)。
  二维码用**品牌版**:中心嵌 pane-grid logo(含青色格)+ 白色 quiet-zone 衬垫;纠错级别用 **H** 以容忍中心遮挡。
  本地=免费(`gtmux serve`),远程=Pro(隧道),与移动端 Servers/计费一致。
- 落地点:popover footer 第 5 个按钮 + 配对 sheet(渲染 QR + serve token)。

---

## B. 移动端 app(MOBILE.md 的增补)

### B1 · Terminal 输入重构(对标 Moshi) — §14
- 详情页加 **「对话 ↔ 终端」双模式**(顶部 segmented,记住每个 pane 的选择)。
- **对话模式 + 审批卡(核心)**:把会话解析成原生聊天(agent 气泡 → 工具卡 → **审批卡**)。审批卡把真实选项做成
  **整行大按钮**(1/2/3),点一下即 `/api/send`,与菜单栏/通知同源。
- 工具条重排、图标语义化:修饰键(可切图标/文字)· D-pad · 历史 · 片段 · 附件 · 语音 · 发送;终端模式保留键盘键
  (单击显隐键盘 / 双击 Enter / 长按语音)+ A−/A+、换行/滚动、回滚缓冲、全屏。

### B2 · 设置页详细设计 — §15
- iOS 分组列表,按频率:账户/Pro → 通知(等你/完成 + 声音 + 勿扰时段)→ 外观(主题/语言/App 图标)→
  终端(默认模式/字体/字号/修饰键显示/xterm)→ 语音(引擎)→ server(管理 + 在电脑打开)→ 安全(Face ID)→ 关于。
- 交互:分组 Form;开关即时;二级页 `›` 单选打勾;破坏性项红色置底;Pro 项未订阅显锁 → 付费墙。

### B3 · 长回复输入 · 粘贴 · 图片标注 — §16
- **核心修复**:输入框单行 `returnKeyType="send"` + `onSubmitEditing` → 一回车就发、无法换行。
  改 **multiline + returnKeyType="default"**、**去掉 onSubmitEditing 发送**;回车=换行,发送只走 **↑ 按钮**。
- 输入框**自增高 1→6 行**,超出内滚;`⤢` 进**全屏撰写**(等宽、大区域、⌘⏎ 发送)。
  快捷回复不变(waiting 的 1/2/3、控制键仍即时发)。设置给「回车直接发送」开关(默认关)。
- **粘贴识图**(已具备):剪贴板是图 → 标注编辑器;是文 → 插入。**＋ 附件**:相册/拍照/文件 → 上传 Mac → 回填路径。
- **图片标注扩展**(`ImageMarkup.tsx` 现仅红画笔):加 画笔/箭头/方框/打码/裁剪/撤销 + 5 色板;Done 拍平 PNG → 上传 → 回填路径。
- 落地顺序:① 多行输入(最高优先)→ ② 全屏撰写 → ③ 标注扩展。①② 只动 `Composer.tsx`;③ 动 `ImageMarkup.tsx`。

### B4 · 通知 / Live Activity / 灵动岛 直接回应(承前)
- 回应选项**不写死「是/总是/否」**,而是 `1/2/3` + **从 Claude 答复原样取出的真实 label**,**分行**排列。
- 三处入口(通知展开 / Live Activity / 灵动岛)+ 应用内 banner,全用同一套,深链直达该 pane。

---

## C. 状态语义铁律(写入 DESIGN.md §1,三屏共用)

> **「等你输入」= 仅 `waiting`(红)。`working`(蓝)永远不会等输入。**

- 结构化 `1/2/3` 回应**只挂在 waiting**:弹层就地回应(A1)、macOS 通知(A2)、切换器(A3)、移动端审批卡(B1)
  四处都以 `status === 'waiting'` 门控;working 行只有加载环 + ›,不出任何「需要你回答」UI。
- 摘要计数:`waiting`→等输入、`working`→运行中,从不混。
- 移动端终端/对话模式下,即便 pane 是 working,输入框仍在(你**主动**发/打断属于「可发」,中性,不标红、不催);
  只有 waiting 才升级成醒目审批卡。

---

## D. 第二轮补充（web 拆分 / 终端体验 / 启动·离线 / 品牌）

### D1 · 菜单栏状态项改「品牌版」
点亮格 = gtmux pane 网格的右上格，随最紧急状态点亮（红方块·双竖线 / 青·加载环 / 绿·对勾）+ 计数；无底色（修掉旧的蓝色实底块 + 孤立对勾），深/浅/着色栏自适应。状态项 = app 图标同源，更有品牌感。落地点：菜单栏 SwiftUI 状态项绘制。

### D2 · 菜单栏「终版」清爽化
去掉所有探索/待选项：品牌只留 pane 网格；状态项只留品牌版；行密度只留两行式；切换器只留最终版。对应 mockup/gtmux-menubar.dc.html（§00–§11，无 option 网格）。

### D3 · Web 浏览器镜像拆为独立文档
从移动端移出，单独成 mockup/gtmux-web.dc.html 并扩充：一次性可过期链接的生成/撤销、居中只读雷达、xterm.js pane 镜像、桌面键盘导航（j/k·Enter·Esc·/）、view-only 安全。移动端章节相应移除并重排。

### D4 · Terminal 页三合一（§09–§11 三部曲）
原分散的「深度设计 / Moshi 双模式 / 长回复输入」聚合为相邻三节：① 渲染与交互 ② 双模式与审批卡 ③ 长回复·粘贴·标注。读起来是一个连贯整体。

### D5 · 终端滚动锁定 / 跟随最新
向上滚进历史后，新输出不再把用户拽到底部；跟随仅在贴底时生效（tail -f 式）。离开底部浮出「↓ 最新」浮标，冻结期间有新内容时浮标加「新」圆点；点它平滑回底并恢复跟随。阈值 24px。落地点：DetailScreen.tsx，刷新前记录 atBottom，仅 true 才 scrollToEnd。

### D6 · 终端 / 对话内容可复制
终端长按选择 + 拷贝/全选 + 顶栏「复制可见屏/回滚缓冲」（去 ANSI）；对话长按气泡拷贝、代码块右上角常驻「复制」按钮。复制是只读操作，不受写权限门控。落地点：终端原生文本选择 + Clipboard.setString。

### D7 · 长回复多行输入（核心修复）
输入框单行 returnKeyType=send → 一回车就发、无法换行。改 multiline + returnKeyType=default、去掉 onSubmitEditing 发送，回车=换行、发送走 ↑ 按钮；自增高 1→6 行；⤢ 全屏撰写；快捷回复（1/2/3、控制键）仍即时发；设置给「回车直接发送」开关（默认关）。落地点：Composer.tsx。

### D8 · 启动 splash / 离线态 / 进 pane 加载态（移动端 §16）
- splash：pane 网格标记 + 字标 + slogan「tmux × agent · Your agents across tmux, at a glance.」+ 底部转圈；原生 LaunchScreen 秒出 → RN 同款；恢复 token 失败直接进配对页。
- 离线：三态连接指示（已连接绿 / 重连琥珀呼吸 / 离线红横幅含原因+重试+上次更新）；不清屏，缓存置灰；推送仍由 APNs 投递、离线照收。
- 进 pane 加载：立即进页（头部+composer 在位），终端区放骨架 + 居中转圈「正在拉取屏幕…」，首帧到达淡入替换；>3s 升级文案、失败给「重试+原因」；数据到达前 composer 禁用态。

### D9 · 连接指示去 live
全表面把「● live」改为 server 名 + 状态点（如 home-mac + 绿点柔光晕）；异常态才补「重连中/离线」文字。落地点：radar/detail/iPad/web 头部连接指示。

### D10 · iPad 扩充
原横屏 split-view 基础上补：竖屏/分屏变窄 → 侧栏收成 ☰ 抽屉、详情占满、再窄回退手机式单列；指针/Apple Pencil（trackpad hover 高亮 + 右键菜单 + Pencil 在 pane 镜像上随手圈画转标注）。

> 注：交接总入口已统一为 HANDOFF.md（旧 HANDOVER*.md 已废弃，可删）。