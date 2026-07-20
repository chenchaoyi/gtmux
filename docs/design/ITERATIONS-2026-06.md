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

---

## E. 第三轮（HQ 中控 / 远程配置 / 网页输入 — 2026-07）

### E1 · HQ 中控（DESIGN §12 · WEB §07 · MOBILE §17）
`gtmux hq`（`role:"supervisor"`）不该长得像普通 session。菜单栏用**参谋长卡片**区分：**👁 CHIEF OF STAFF · 参谋长**角色横幅 + **1px 描边面板**（agent 行都没有）；点它=**跳到中控 pane**（与跳 worker 一致），不在弹层另开面板。真正的"指挥台"（态势板+命令台）在**手机 HQScreen（已实现）与网页宽屏版**。
**v2（参谋长卡去 session 化）**：删状态角标（HQ 恒在监视，无 idle/working 之分）、删 agent 名与时长；名字旁挂**舰队光点条**（每 worker 一枚色+形 pip）；副题只说值得知道的事；HQ 自身等你拍板→整卡琥珀描边+副题红字。方案已选定：舰队光点条（menubar mockup §12）。
- **指挥所三区**：状态条（舰队计数 + 订阅窗口 % + 资源告警）· 舰队态势板（`/api/digest`，waiting 的 ask 琥珀高亮、ctx% 右对齐）· 命令台（与 HQ 对话 + 快捷命令 简报/谁在等我/该我拍板/派活）。
- **网页宽屏**为三栏：左态势 / 中对话 / 右派活台账（`/api/tasks`，spawn/reap）。窄屏折叠。
- **红线**：命令台只对 HQ 说话，HQ 再驱动舰队（`send/spawn`）；HQ 只建议、不擅自替别的 agent 拍板；长按舰队行跳该 worker 的 Detail 直接回。落地点：菜单栏 popover + HQ panel、`HQScreen.tsx`（已实现，此为规范镜像）、`web/`。

### E2 · 偏好：Anywhere + Sharing 两标签页（DESIGN §13）
把「连不连得上」（网络层）与「谁能干什么」（权限层）拆成两个正交标签，各配人话解读：
- **远程访问 = 通用底座**（对 Pair 与 Share 同时生效）：LAN 免费；Standard 隧道免费（稳定托管址、配一次）；**Direct 隧道 = 兑换码解锁（付费）**，你自己的 VPS + 域名（self-tunnel），给屏蔽 Cloudflare 的网络。+ 保持常开 + 安全提示（URL+token 当密码）。
- **身份层**：**Pair** = 自己（全权 = 菜单栏全部能力，含 HQ）；**Share** = 协作者（逐 session「查看/输入」，输入隐含查看，可吊销，不含 HQ 与偏好）。文案统一「可见/输入」；图标扁平化（几何形/等宽 chip，去 emoji），§14 底栏内联指示同步扁平。
- **偏好整窗（对齐 Preferences.swift）**：分组表单（通用/状态栏/通知/远程访问/我的设备·配对/分享/软件更新）；远程访问=关闭|局域网|任意网络分段+隧道后端+当前已连接（空则隐藏）；切任意网络先弹长期敞口确认。**配对 sheet**=一码三媒介（QR / url/#c=码 / gtmux attach，5 分钟一次性），顶部访问状态条（模式+隧道后端+地址+切换），远程访问未开时前置一步「开启」只开门（任意网络可细选 标准/直连，直连未解锁置灰；配对码由 sheet 主页生成；不经偏好折返）；**分享 sheet**=命名+逐 session 可见/输入一步建链，逐链接 scope 可展开编辑；创建后交付页=一码三媒介（QR / url/#g=码 / gtmux attach，与配对同构），token 只显一次。

### E3 · 底栏 v3（按频次分层，DESIGN §14）
永久底栏只剩一行：＋新建会话（左）+ 状态内联（仅为真时：绿点+设备数、「输入」chip）+ 版本号（常显 dim mono）+ ⚙︎ 菜单（偏好设置…/配对设备…/检查更新/退出 ⌘Q）。情境行「↩ 恢复上次的工作现场」仅在重启后（有快照且无在跑会话）出现，一键恢复全部会话与窗口（gtmux restore）。频次归宿：新建=常驻；恢复现场=情境行；配对/偏好/更新/退出=⚙︎ 菜单（配对另在空态 CTA）；远程/分享=状态非动作。按钮一律图标+文字同行。HQ 仍在顶部参谋长卡（§12）。

### E4 · 网页输入 + 权限透出（WEB §08）
网页从只读升级为可输入（`POST /api/send` / `attach`）。每个 tile 头部**明示** ⌨可敲（青，有 composer + 1/2/3）/ 👁只读（灰，无输入区 + 一行「未授权」说明，不留空文本框）。owner 全 pane 可敲；guest 仅 host `--type` 白名单 + 总开关。owner/guest 顶栏身份不同，HQ 指挥台不对 guest 开放。服务端强制、撤销即时。

### E5 · 派活台账 + 用量/额度/资源透出
- **spawn/tasks/reap**：HQ 指挥台右栏「派活台账」——每个 dispatch 的 working/done/gone，done 给 reap（安全门：worktree 干净 + 分支已并）。
- **usage/limits/resource**：状态条与舰队板显 ctx%（⚠ 近上限琥珀）、订阅窗口 %（wk/Fable）、磁盘/内存告警；对齐 `/api/usage`、`/api/digest` 的 `usage_warn`。

### E6 · §02 状态栏图标同步
done 态不带计数（计数只给 待输入/运行中，对齐 BadgeText）；计数**含 HQ**（参谋长等你也算等你）；三档显示（点+数字/仅圆点/空闲时隐藏）入口在偏好·状态栏。

> 状态语义铁律不变：`1/2/3` 结构化回应只挂 waiting；HQ 只建议不代拍板。


---

## F. 移动端对齐轮（2026-07）— 对齐仓库实现

### F1 · 计费全部移出手机（§07/§08 重做）
多 server / 多设备**不收费**，手机端无付费墙；旧「多 server 计费」「Tunnel 订阅 ¥」方案取消。唯一付费点在 Mac 端（Direct 隧道兑换码）。§07 = 添加 server（扫码主路径，#c= 配对码=我的 Mac / #g= 分享链接=访客；手动 host+token 兜底；无 server 自动弹出 + 只读 demo）。§08 = Servers 连接页（对齐 ServersScreen：我的 MAC / 访客连接两轨分组永不混排、绿点=当前、点行切换按 URL 重挂载、✕ 移除=清 Keychain+让该 Mac 丢弃本机推送 token、＋添加、断开连接）。访问方式（LAN/Anywhere/隧道后端）全在 Mac 端配，手机只连 URL。

### F2 · Composer 对齐（§9/§10/§11 补记）
FloatingKeys 方向键盘退役。静息键条 = ⌨ | Tab ⏎ Ctrl-C Esc | 快捷短语▾ 历史；**写死 1/2/3 从键条移除**，waiting 回应全由 ApprovalCard（/api/options 真实选项 1..N chips）承担；Tab(接受)+⏎(提交)相邻。默认回车=换行、↑ 发送（设置可开「回车直接发送」）；1→6 行自动增高 → ⤢ 全屏撰写（⌘⏎ 发送）。附件先暂存后发送（缩略图条、发送时逐个上传带 %、失败保留重试、图片先过标注器、粘贴识别图片）；AttachSheet=照片/拍照/文件/粘贴；快捷短语=picker、历史=modal。

### F3 · 通知直接回复落地（§14 补记）
iOS category AGENT_WAITING 固定三键 1·Yes/2·Always/3·No（系统限制静态）→ 后台 /api/send **数字不带 Enter**；正文点按=深链（payload 带 server 名先切服务器）；动态文案留给 app 内 ApprovalCard；通知内动态按钮/LA 内嵌回复列 future；角标=waiting 数（静默推送维持）。

### F4 · 设置页重做（§15）
Moshi 式分组卡 + PickerSheet（行显当前值+›，底表单选）：连接（server→Servers / 在电脑上打开=铸码分享浏览器链接 / 管理这台 Mac=远程管 share / 移除）· 终端（外观 / 默认模式 终端|对话 / 回车直接发送）· 通知（总开关 + 等你回应/已完成）· 通用（语言）· 关于（版本）。访客隐藏 通知/在电脑上打开/管理这台 Mac。旧「账户/Pro」分区随计费取消删除。

### F5 · iPad 对齐（§06 补记）
SplitScreen：宽度≥768 触发（非仅 iPad 硬件）；侧栏 320pt 复用 SectionList、折叠状态与手机同键；server chip(品牌标+名+⇄)；点行原地换主区、native 行不可选；推送深链=宽屏选中该行；离线/横幅同手机。

### F6 · HQ 入口对齐菜单栏 v2（§17）
雷达 HQ 入口改为**参谋长卡**：👁 CHIEF OF STAFF 角色横幅 + 描边卡 + 品牌网格头像（**无状态角标**）+ **舰队光点条** + 琥珀副题；HQ 自身等你=整卡琥珀。点它进 HQScreen（不变）。


### F7 · Demo 模式（§18，对齐 DemoScreen.tsx + 优化提案）
已实现：真雷达+真 Detail 套假 client（demoClient/demoData，零网络）；「See a demo」入口；DEMO chip 雷达+Detail 全程跟随；Servers 永无 demo 条目；退出即重置；hero %7 带 1/2/3 真实选项 → ApprovalCard；canned 回复恒以「配对你的 Mac」引导收尾。App Review 以 demo mode 代替演示账号。
优化提案（待实现）：① 入口从 dim 链接升级为 DEMO 徽章次级卡（副题「样例数据 · 无需任何服务器」）；② **状态弧**：批准 %7 后在雷达上走完 waiting→working(~5s)→idle+latest，让审核员亲眼看到核心循环；③ HQ 参谋长卡入选 demo（canned digest + 一轮预设对话）。CTA 恒为青底「配对你的 Mac」；底部 CTA 不遮列表末行。
