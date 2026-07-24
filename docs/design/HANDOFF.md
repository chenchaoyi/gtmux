# 交接给仓库 Claude Code — 设计对齐与迭代（2026-07-18）

## 你（用户）怎么操作

1. 把本 `docs/design/` **叠加覆盖**进仓库的 `docs/design/`（同名文件覆盖）。
   > 不要清空目录：仓库自有的 `multi-agent-multi-terminal.md`、`remote-access-tunnel.md`、`DECISIONS-*` 等不在本包里，保留。
   > 例外：仓库的 `MOBILE.md` 若含比本包更新的实现记录（NativeTerm/chat 等小节），取并集而非直接覆盖。
2. 把 `CLAUDE.snippet.md` 合并进仓库根 `CLAUDE.md`（替换其中旧的设计段落）。
3. 仓库根启动 Claude Code，整段粘贴下面的 Prompt。

---

## 粘贴给 CC 的 Prompt

你要为 gtmux 做一轮**设计对齐与迭代**，覆盖三个表面：菜单栏（`macapp/`）、手机（`mobileapp/`）、Web（`internal/server/web/`）。设计权威在 `docs/design/`：

先读（顺序）：
1. `docs/design/DESIGN.md`（菜单栏权威规范，§12/§13/§14 为本轮重点）
2. `docs/design/MOBILE.md` + `docs/design/WEB.md`
3. `docs/design/ITERATIONS-2026-06.md` 的 **§E / §F**（最新两轮变更清单，权威）
4. 浏览器打开 `docs/design/mockup/gtmux-menubar.dc.html`、`gtmux-mobile.dc.html`、`gtmux-web.dc.html` 对照像素细节（离线看不了就按文字规范）。

### P0 · 重点迭代（按序执行，每项一个 commit）

**1. 偏好设置整窗重做**（menubar mockup §13）
- 分组表单：通用 / 状态栏 / 通知 / **远程访问** / **我的设备·配对** / **分享** / 软件更新。
- 远程访问：`关闭 | 局域网 | 任意网络` 分段 + 地址副题 + 隧道后端 `标准 | 直连`（直连=**兑换码解锁**，走**你自己的 VPS+域名** self-tunnel）+「当前已连接」实时名单（空则整块隐藏）。切「任意网络」先弹长期敞口确认。
- 配对 sheet：顶部**访问状态条**（模式+后端+地址+切换）；远程访问未开时先走**前置步**（选 局域网/任意网络，任意网络下细选 标准/直连、未解锁置灰；按钮只「**开启**」，配对码回主页生成）；主页 = 一次性码（5 分钟）**三种媒介**：扫 QR / 浏览器 `url/#c=码` / `gtmux attach`。⚙︎ 菜单「配对设备…」与空态 CTA 直达，不经偏好。
- 分享：逐 session 勾「**可见 / 输入**」（输入⊆可见，可见未勾则输入置灰）；创建后翻**交付页**（与配对同构一码三媒介，`#g=` guest token 只显一次）；已有链接行可展开编辑 scope、可吊销；「允许协作者输入」总开关。
- 文案统一「可见/输入」；图标扁平（几何形+等宽 chip），**无 emoji**。

**2. HQ 中控 · 参谋长卡 v2**（menubar §12，已选定方案）
- 卡片 = 「👁 CHIEF OF STAFF · 参谋长 · 统观全局」角色横幅 + 1px 描边面板 + **品牌网格头像（无状态角标）** + ~~舰队光点条~~ **情报头条副题**（**hq-meta-layer 已反转此处**：光点条那排匿名色点与列表/计数重复、连"谁在等"都答不了，已删；改为从舰队合成的一句参谋长结论）；HQ 自身等你 → **整卡琥珀**。
- 未运行 = 虚线幽灵条「中控未运行 · 点击启动」（shell `gtmux hq`）；搜索与真空态隐藏。
- 点卡 = **跳到中控 pane**（不开面板）。摘要计数**不含** HQ；状态栏图标计数**含** HQ。
- 手机雷达 HQ 入口同构（mobile §17）。**手机 HQ 页已随 `hq-command-page` 重构**：不再有「舰队态势板」，改为 判断 / 该你拍板 / 动态 / 对话 四区（态势板=雷达答不了的东西，不再复列舰队）。Web 宽屏指挥台三栏（态势 `/api/digest` / 对话 / 派活台账 `/api/tasks`，不对 guest 开放）（web §07）**尚未实现**，其态势列设计待与 mobile 一并复审。

**3. 底栏 v3**（menubar §14）
- 永久一行：左「＋ 新建会话」（图标+文字同行）· 右 状态内联（绿点+设备数、「输入」chip，**仅为真时**）+ 版本号（dim mono 常显）+ ⚙︎ 菜单（偏好设置… ⌘, / 配对设备… / 检查更新 / 退出 ⌘Q）。
- 情境行「↩ 恢复上次的工作现场 · N 会话 M 窗口」：**仅重启后**（有快照且无在跑会话）出现，一键 `gtmux restore`。
- 旧「接回/新建/配对」三格与旧连接条删除。

**4. 状态栏图标同步**（menubar §02）
done 态不带计数；计数=waiting 数否则 working 数（`BadgeText`）且**含 HQ**；三档显示（点+数字/仅圆点/空闲时隐藏）入口在偏好·状态栏。

### 移动端 F 轮（ITERATIONS §F，对照现有实现查漏补缺）
- F1 计费全部移出手机：无付费墙；添加 server = 扫码主路径（`#c=`我的 Mac / `#g=`访客）；Servers 两轨分组（我的 MAC / 访客连接）、移除=清 Keychain+撤推送 token。
- F2 Composer：静息键条 `⌨ | Tab ⏎ Ctrl-C Esc | 快捷短语▾ 历史`；写死 1/2/3 移除，waiting 回应由 **ApprovalCard**（`/api/options` 真实选项 1..N）承担；回车=换行、↑ 发送、⤢ 全屏撰写；附件先暂存后发送（上传带 %、失败重试、图片先过标注器）。
- F3 通知：category 固定三键 1·Yes/2·Always/3·No，后台 `/api/send` **数字不带 Enter**；点按深链（payload 带 server 名先切服务器）；角标=waiting 数。
- F4 设置页：Moshi 分组 + PickerSheet（行显当前值+›）；连接/终端/通知/通用/关于；**访客隐藏 owner 专属项**。
- F5 iPad：SplitScreen 宽度≥768、侧栏 320 复用 SectionList、原地换主区、推送深链=选中行。
- F6 HQ 雷达入口 = 参谋长卡（同 P0.2）。
- F7 Demo 模式优化（mobile §18 / ITERATIONS §F7）：入口升级为 DEMO 徽章次级卡；批准后**状态弧** waiting→working→idle(latest) 在雷达可见；HQ 参谋长卡入选 demo（canned digest + 预设对话）。边界铁律不动：DEMO chip 全程、Servers 无条目、退出重置、零网络。

### Web
- tile 头部明示 `⌨ 可输入`（青，composer+数字 chip）/ `👁 只读`（灰，无输入框+「未授权」一行）；guest 权限=**逐链接 scope**（输入⊆可见）+ 总开关，服务端强制；owner/guest 顶栏身份不同。
- 状态徽章补**字形**（红方块双竖线 / 青加载环 / 绿✓ / 灰点）——色+形+字形三重编码。

### 红线（不可违背）
- 颜色只表达状态（`#EF4444/#06B6D4/#22C55E/#8E8E93`）；绝不只靠颜色。
- `1/2/3` 结构化回应只在 waiting 出现，选项文案取 agent 真实 prompt。
- 权限服务端强制；HQ 只建议不代拍板；双语 en/zh、CJK 不换行。
- 与 mockup/规范有出入：**先报差异再改**，不要擅自偏离；每完成一项对照 mockup 自查并输出验收清单。

