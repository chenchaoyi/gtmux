# 服务器模式 —— 合盖运行调研（2026-07-30，07-31 更新）

`server-mode` change（`openspec/changes/server-mode/`）的可行性调研：
gtmux 能不能让 MacBook 合上盖子继续干活，让 `serve` + `tunnel` + 手机一直有应答，
而且永远不会把机器留在一个再也睡不着的状态？

下面严格区分两类断言，因为它们的分量不同：

- **[MEASURED]** —— 本轮在开发机（Apple M4 Pro，macOS **26.5.2** / build 25F84）上实测。可复现。
- **[SOURCED]** —— 来自文档或可信的第三方。够用来做设计，不够用来直接发布。

两者都不是的，标为**待定** —— 见 §7。其中一项是阻塞项。

## 1. 结论

**已在目标硬件上证实（2026-07-31）。阻塞性测试通过：合盖的 M4 Pro 开着 `disablesleep`，
整个测试期间持续提供服务，内核确认零次睡眠。**方案成立，有十年之久的先例，而且现在是实测过的，
不只是推理。

- gtmux 需要的机制存在，macOS 26.5.2 仍然认它 **[MEASURED]**，并且与这个品类的领头产品
  （Mac App Store 上的 Amphetamine）多年来用的是同一个机制 **[SOURCED]**。
- 更便宜的、无特权的路径走不通，不是 gtmux 的局限，而是 macOS 的架构事实：合盖睡眠是一条独立的、
  硬件触发的路径，电源断言（power assertion）不参与 —— **2026-07-31 在本机确认**：一个持有中的
  `caffeinate` 断言没有扛过合盖（§2.0）。**[MEASURED]**
- 推荐的授权路径（§4）正是 Amphetamine 的做法，而且它是从沙盒化的 App Store 二进制里做到的
  **[SOURCED]** —— 所以并非什么另类选择。
- **A1 —— 合盖运行：已确认 [MEASURED]**（§7）。开着 `disablesleep` 合盖约 4 分钟：内核 Sleep/Wake
  计数不变（58 → 58），记录器间隔 6 s（即没有中断），`/api/health` 50/50 × 200。对照同机的 0.0
  对照组 —— 133 s 内就睡了（`pmset -g log`：*"Entering Sleep state due to 'Clamshell Sleep'"*）——
  这个机制做的正是它声称的事。
- **A2 —— 只限交流电的作用域：已推翻 [MEASURED]**（§7）。`pmset -c` 被接受，但值落在 plist 的
  `SystemPowerSettings` 顶层，也就是**全局的，含电池**。拔电源时没有内核强制的恢复，所以守卫的
  电源轮询是*唯一*防线，不是备份。这让守卫比初稿设想的更承重。
- **A2b —— 电池运行：已确认 [MEASURED]**（§5.1）。拔掉电源、合盖，45 个采样里 42 个在电池上：
  零次睡眠，全程提供服务，重新插电也没有失效。「拿着它在会议室之间走」这个用例成立。
  这**与现有产品自己的文档矛盾**，那份文档说电源不能拔 —— 以实测为准。
- **A3 —— 怎么读状态：已解答，先错了两次**（§3.1）。`ioreg` 的 `IOPMrootDomain.SleepDisabled`
  是唯一正确的回读；`pmset` 从不显示它，plist 有滞后。两个错误答案都作为真实 bug 复现过，
  其中一个把一台睡不着的 Mac 报成「会正常睡眠」。

有一项发现实质性地加固了设计（§5）：在 Apple Silicon 上，**插拔电源是合盖运行已知的脆弱时刻**，
不是理论上的边角。Amphetamine 正是为此出过一类 bug，随后又加了面向用户的失败提醒。

## 2. 为什么无特权的路径走不通

macOS 有两条相互独立的睡眠路径，只有一条听断言的：

| 路径 | 触发 | 用户态能压住吗？ |
|---|---|---|
| 空闲睡眠 | 不活动超过 `sleep` 计时器 | **能** —— IOKit 电源断言（`caffeinate`） |
| 合盖（clamshell）睡眠 | 盖子传感器 | **不能** —— 断言不参与 |

- 合盖「不是普通的不活动 —— 它是一条独立的、硬件触发的睡眠路径」，所以一个忙碌的进程*和*一个
  空闲睡眠断言可以同时存在，而机器在盖子合上的一瞬间就睡 **[SOURCED]**。
- `caffeinate -s`（「阻止系统睡眠」）在自己的 man page 里就有明确限定：*"This assertion is valid
  only when system is running on AC power."* **[MEASURED —— `man caffeinate`]**。它对盖子只字未提，
  它发起的断言类型（`PreventSystemSleep`）是空闲路径的概念。
- 本机上活跃的断言类型全是空闲/显示路径的：`PreventUserIdleSystemSleep`、
  `PreventUserIdleDisplaySleep`、`NoDisplaySleepAssertion`、`SystemIsActive`、
  `PreventSystemSleep` **[MEASURED —— `pmset -g assertions`]**。没有一个提到盖子。
- *真正*有效的：`pmset disablesleep 1` 设置一个内核标志（`SleepDisabled`），`IOPMrootDomain`
  把它当作对睡眠的否决，并且**跨过合盖事件仍然有效** **[SOURCED]**。这与断言不在同一层，
  这也是为什么需要 root。

### 0.0 —— 在目标硬件上实测（2026-07-31）

上面的断言不再只是有出处。在本机（M4 Pro，macOS 26.5.2，**无外接显示器**）运行：
持有 `caffeinate -dis`，合盖约 3 分钟，再打开。

| 信号 | 结果 |
|---|---|
| 内核 `Sleep/Wakes since boot` | **57 → 58** —— 内核记了一次睡眠 |
| 记录器日志间隔 | **136 s** —— 整个合盖期间用户态被冻结 |
| `/api/health` 采样 | 11/11 × HTTP 200 —— `serve` 前后都健康，所以间隔是睡眠，不是服务死了 |

两个独立信号一致，第三个排除了显而易见的混淆因素。**持有中的 `caffeinate` 断言在 Apple Silicon
上扛不过合盖。**所以 root 在这里不是设计偏好 —— 它是唯一剩下的机制，和 §2 预测的一样。
`[MEASURED]`

**对 gtmux 的后果：**设计里的双档模型不是填充。`awake` 档（断言）老老实实地扛不过合盖，
任何暗示它能的 UI 都是在撒谎。只有 `clamshell` 档（root）做到用户想要的事。

另有独立证据表明这是真实、活生生的痛点，而非假设：「手机上用 Claude Code」的生态已经发布了
若干变通办法，每一个都是空闲路径断言或者手改节能设置 —— 也就是说没有一个真正解决盖子的问题
**[SOURCED]**。见 §6。

## 3. 关于 `disablesleep` 什么是真的（有风险的部分）

**它没有文档。**macOS 26.5.2 的 `man pmset` 里**任何地方**都没有 `disablesleep` **[MEASURED]** ——
`SETTINGS` 里没有，整页都没有 —— 广泛使用的第三方 pmset 参考里也没有 **[SOURCED]**。
gtmux 建在它之上的一切，都压在一个 Apple 从未承诺过的接口上。这是真实风险（§7/R5），不是脚注。

**macOS 26.5.2 仍然认它 [MEASURED]。**不带 root 探测，所以什么都没改：

```
$ pmset -c disablesleep 1          → "'pmset' must be run as root..."   (parsed OK)
$ pmset -a disablesleep 1          → "'pmset' must be run as root..."   (parsed OK)
$ pmset -a totallybogussetting 1   → "Usage: pmset <options>"           (rejected)
```

参数解析发生在权限检查之前，所以能走到「must be run as root」就是设置名有效的正面证据，
而假名字的对照证明这个探测有区分度。它同时说明 **`-c` 在语法上被接受** —— 但被接受不等于
作用域真被*遵守*（§7/A2）。

**实现陷阱 [MEASURED]：**那次失败返回的**退出码是 0**。`pmset` 打印「must be run as root」然后以
0 退出。所以调用方**绝不能**从退出码推断成功 —— 要把设置读回来核实。漏掉这点会造成这个功能里最糟
的 bug：gtmux 以为睡眠已禁用而实际没有，或者以为已恢复睡眠而实际没恢复。

**它跨重启持久 [SOURCED + 与 MEASURED 一致]。**pmset 设置存在系统级 plist 里，重启后仍在。
正是这个性质让遗留下来的 `1` 危险，也是设计里存在开机对账的原因。

**路径更正 [MEASURED]。**第三方参考（以及我们自己设计的早期草稿）给的是
`/Library/Preferences/SystemConfiguration/com.apple.PowerManagement.plist`。
macOS 26.5.2 上这个文件**不存在**；真正的文件是
`/Library/Preferences/com.apple.PowerManagement.plist`（root:wheel，所有人可读）。

### 3.1 到底怎么读状态 —— A3，走了弯路才解答 [MEASURED 2026-07-31]

三个候选来源；在找到正确的那个之前，**其中两个在验证脚本里产生了真实、可复现的 bug**。
这张表对实现是规范性的：

| 来源 | 裁定 |
|---|---|
| `pmset -g` / `-g custom` / `-g live` | ❌ **两种状态下都从不打印 `disablesleep`。**脚本用了它，在 `SleepDisabled` 为 true 时告诉操作者「睡眠已恢复，Mac 会正常睡眠」—— 正是这个功能要避免的最坏失败，被意外复现了。 |
| plist → `SystemPowerSettings.SleepDisabled` | ⚠️ 真实且所有人可读，但**滞后于写入**：`pmset -a disablesleep 0` 成功一秒后它仍读到 `true`，脚本因此报「恢复失败」，而恢复其实已经生效。它描述的也是*持久化*的设置，不是当前的内核行为。 |
| **`ioreg -r -c IOPMrootDomain -d 1` → `"SleepDisabled" = Yes\|No`** | ✅ **内核的实时状态。即时、无需特权、权威。** |

两个值得明说的陷阱：

- **关是 `false`，不是缺失。**从未设置过的机器**没有** `SleepDisabled` 键；设置过又恢复的机器
  持有 `SleepDisabled => false`。两者都表示「关」，所以「键是否存在」是错误的判据 —— 要解析值。
- **两个实时来源回答的是不同的问题，都需要。**`ioreg` = 「*此刻*睡眠是否被禁用」（每个界面显示的）。
  plist = 「*重启之后*还会不会被禁用」（开机对账和 `doctor` 关心的，因为设置是持久的）。
  谁也替代不了谁。

**关闭态对随手一看的人是不可见的 [MEASURED]。**默认的 `pmset` 输出里没有任何东西暗示这个设置，
所以某人机器上一个遗留的启用态，除非你知道去 `ioreg` 或 plist 里找，否则根本发现不了。
这是本 change 里 `doctor` 检查最有力的论据 —— 没有它，这种失败模式在构造上就是静默的。

## 4. 授权路径的先例

**Amphetamine**（Mac App Store，这个细分领域最有名的工具）用 `pmset disablesleep` 实现
Closed-Display Mode，通过 `do shell script "pmset disablesleep 1" with administrator privileges`
提权 **[SOURCED]** —— 与本 change 推荐的路径相同（`design.md` §1 的选项 A/E）。它通过 App Store
分发，所以是在 app 内处理管理员认证，不改 sudoers，也没有 Gatekeeper 的摩擦 **[SOURCED]**。

这是很强的先例：这个机制加这条提权路径，在一个装机量很大、经 Apple 审核的 app 里挺过了多年的
macOS 版本更迭。

**要诚实记录的反面压力。**Apple 自己对这条提权路径态度冷淡：`do shell script … with administrator
privileges` 被视为等同于 `sudo`，面向管理员而非作为 app API；底层的
`AuthorizationExecuteWithPrivileges` 已被正式弃用多年却仍在工作 —— 「不应在广泛分发的产品中使用」
**[SOURCED]**。现代替代品 `SMAppService`（macOS 13+）把守护进程放在 **app bundle 内部**，
所以删掉 `.app` 它就随之移除 **[SOURCED]**。

这让 change 对特权 helper 路线的否决更尖锐 —— 但没有反转它：

- `SMAppService` *在构造上就是 bundle 作用域的*。gtmux 的 CLI 通过 Homebrew 和 tarball 发布，
  根本没有 `.app`，所以 helper 路线会让 app 用户有这个功能、brew 用户什么都没有。这个割裂就是
  否决理由，现在是确认过的，不是假设。
- 它还会引入设计核心不对称性所禁止的常驻 root RPC：一个存在的目的就是*被请求提权*、
  在没人在机器前时也可达的接口。
- 弃用风险的缓解是一条回退阶梯，不是重写：如果 AppleScript 路径哪天被关掉，回退到窄范围的
  `sudoers` 片段（`design.md` §1 选项 C），或者引导用户手动跑一次 `sudo`。两者都保持不对称性；
  都不需要 helper。

## 5. Apple Silicon：电源切换是脆弱时刻

这是改变了设计、而不只是确认了设计的发现。

- Amphetamine 5.3 之前，Apple Silicon 上的 Closed-Display Mode **在 Mac 接上或断开外部电源时
  会失效**（充电器，或带供电的显示器）**[SOURCED]**。
- 修复之后，厂商仍警告 Apple Silicon 笔记本在电源切换时 Closed-Display Mode「可能不按预期工作」
  **[SOURCED]**。
- Amphetamine 5.2 为*失败的*合盖会话加了面向用户的提醒，外加可选的自动终止 **[SOURCED]** ——
  也就是说，现有产品得出的结论是：对这类失败的正确回应是**告诉用户**，而不是装作成功了。
- 而具体到拔电源这个方向，厂商直白地说，因为 OS 层面的限制，电源线拔掉后它**无法**让 Mac 在
  合盖状态下保持唤醒 **[SOURCED]**。

### 5.1 2026-07-31 实测：拔电源的说法在这里不成立

上面的厂商文档说 OS 限制导致电源一拔合盖运行就停。**在 macOS 26.5.2 / M4 Pro 上这是假的**，
而这个测试正是因为一项用户需求依赖它才做的（拿着合盖的笔记本用电池在会议室之间走）：

| 信号 | 结果 |
|---|---|
| 开始时服务器模式已启用 | **是** —— 由脚本记录，不是假定 |
| 电池上的采样数 | **45 个里 42 个** —— 大部分时间电源是拔掉的 |
| 内核 Sleep/Wakes | **60 → 60** —— 零次睡眠 |
| 记录器间隔 | **6 s** —— 没有中断 |
| `/api/health` | **45/45 × 200** —— 全程提供服务 |

拔掉电源、合上盖子、走动：它一直在工作。之后重新插电也没有失效，所以**电源来回的两个方向都通过了**。
这与 A2 一致（设置是全局存储的，不分电源配置），与厂商的说明不一致 —— 那可能描述的是更老的 OS，
或者是外接显示器的 clamshell 模式而非 `disablesleep` 路径。以实测为准；引用保留，
让分歧可见而不是悄悄丢掉。`[MEASURED]`

三个后果，都已并入 change：

1. **「拔电源 ⇒ 结束会话」作为策略被否决。**它既不是系统行为（§5.1 —— 电源拔掉时机器**没有**睡），
   也不可取：拔电源是核心用例的正常一环。护栏改为按**剩余电量**键控 —— 到阈值警告、到底线恢复睡眠 ——
   这才是真正能预测伤害的量。见设计 §2.3。
2. **重新插电是一个测试用例，不只是拔电源。**验证「拔掉 → 睡眠」不够；插回去的路径也必须验证，
   因为那才是有文档记载的 Apple Silicon 故障。已加入手动检查清单（`tasks.md` 0.2b）。
3. **失败/失效的状态必须可见。**现有产品为此需要一个提醒；我们也需要。设计里带注意色的常驻指示器，
   加上携带机器可读原因的退出通告，覆盖了这一点 —— 前提是指示器也能表达「这个悄悄不工作了」，
   而不只是「开」和「关」。

## 6. 竞争格局 —— 这个领域没有人解决它

远程控制 agent 的工具完全不处理盖子的问题 **[SOURCED]**：

- **VibeTunnel** 通过本地 web UI 暴露 agent 终端 —— 一个*可达性*答案，假定机器保持唤醒。
- **Happy** 是 Claude Code / Codex 的移动客户端 —— 同样的假设。
- **Claude Code 自己的 Remote Control** 从另一台设备接续本地会话 —— 同样的假设。
- 流传的「Claude Code 跑着的时候让 Mac 别睡」的社区变通办法，是 `caffeinate dims claude`
  （空闲路径 —— 扛不过合盖，§2），或者手动把节能计时器设成「永不」然后记得改回去。
- *确实*解决了盖子问题的工具（Amphetamine、KeepingYouAwake、Caffeinated，以及专注 clamshell 的
  小工具）是通用的保持唤醒 app，对 agent、会话、远程指令一无所知。

所以这个空白是真实且具体的：**远程 agent 工具假定 Mac 醒着；保持唤醒的工具不知道 agent 是什么。**
gtmux 是唯一有条件把「让机器持续服务」和「有 agent 正在回合中、另一头有一部手机」绑在一起的产品 ——
包括通用工具在结构上做不到的那部分：活干完了就结束这个状态，并在操作者的手机上告诉他已经结束。

安全文案还值得注意一点：独立来源警告无风扇的 MacBook Air 靠键盘区域散热，所以长时间合盖高负载
最多会损失约 50% 的持续性能 **[SOURCED]**。这比设计里笼统的「合盖散热更差」强得多，
支持在授权卡片里专门点名 Air 的情况。

## 7. 待定项与风险

**已关闭 —— 2026-07-31 在 M4 Pro / macOS 26.5.2 上实测，无外接显示器**

- **A1 —— 合盖真的能持续服务吗？能。**开着 `disablesleep`：内核 Sleep/Wake 58 → 58（没睡），
  记录器间隔 6 s，合盖约 4 分钟内 `/api/health` 50/50 × 200。对照组（0.0，同机，仅断言）：133 s 后睡了。
  **功能的阻塞性前提成立；可按设计实现。**
- **A2 —— `-c` 的作用域被遵守吗？没有。**写入成功，但落在 `SystemPowerSettings`（全局），
  不在 `AC Power` 下。⇒ G3 删除，G4（守卫电源轮询）升为唯一防线，G1（电池上拒绝）更要紧。
  见设计 §1/§2。
- **A3 —— 回读：只用 `ioreg`。**完整表格和两个陷阱见 §3.1。

- **A2b —— 电池上能撑住吗？能 [MEASURED 2026-07-31]。**已启用，45 个采样里 42 个电源拔掉，合盖：
  零次睡眠，无中断，45/45 × 200；重新插电没有失效。⇒ **会议室之间的用例可以交付。**护栏从
  「拔电源 ⇒ 退出」改为电量底线（§5.1，设计 §2.3）。与 §5 的厂商文档矛盾 —— 以实测为准，引用保留。

**仍待定**

- **R7 —— 电池快耗尽时 launchd 还会调度守卫吗？**电量底线假定守护进程一直拿到它 30 秒一次的
  tick，直到 20%。macOS 在低电量模式和接近耗尽时会节流后台活动，我没有测过 `StartInterval`
  能否扛住。如果被节流，救援是*晚到*而不是缺席 —— 底线设在 20% 部分就是为了留这个余量，
  而且这个设置本身就是让机器醒着的东西，所以不存在「需要守卫而机器已关」的状态。
  值得在真实放电中测一次；记下来而不是假定，因为「我答应过要记这一条」正是那种不然就会蒸发的事。

- **G13（失效态检测）保留位置，但失去了原来的理由。**它的动机是 Apple Silicon 电源切换的故障，
  而那在这里**没有**复现。保留它 —— 便宜，而且设置仍可能被 MDM 配置、OS 更新或别的工具改掉 ——
  但别再把它描述成已知的、活生生的失败路径。
- **A4 —— 管理员对话框的 Touch ID。**只是 UX。
- **R5（新）—— `disablesleep` 没有文档（§3）**，AppleScript 提权路径已弃用但仍工作（§4）。
  缓解：§4 的回退阶梯，加上 `doctor` 在机制失效时把实情告诉用户（回读纪律让它可检测而非静默）。
- **R6（新）—— 无登录会话的重启。**`gtmux serve` 作为 **LaunchAgent**（每用户）运行，所以重启后
  要等到有用户会话才启动。开着 FileVault、没有自动登录时，直到有人打开盖子才会有人登录 ——
  所以心跳永远不会恢复，守卫正确地恢复了睡眠。**这是对的故障安全**（一台够不着的机器就该睡），
  但它意味着*服务器模式在 FileVault 锁定的 Mac 上扛不过重启*，文档必须写明，而不是暗示常驻。
  gtmux 不得通过动 FileVault 或自动登录来「修」它 —— 这是明确的非目标。

## 8. 来源

- [Why caffeinate does not work with the lid closed — clamshell.dev](https://clamshell.dev/guides/why-caffeinate-does-not-work-lid-closed)
- [Amphetamine & Closed-Display Mode — Toothpicks Support](https://iffy.freshdesk.com/support/solutions/articles/48001077199-amphetamine-closed-display-mode)
- [About Failed Closed-Display Mode Sessions — Toothpicks Support](https://iffy.freshdesk.com/support/solutions/articles/48001180528-about-failed-closed-display-mode-sessions)
- [Amphetamine: keep your MacBook awake in clamshell mode — TechPP](https://techpp.com/2021/06/18/macbook-clamshell-mode-keep-awake-amphetamine/)
- [How to use a MacBook with the lid closed — Macworld](https://www.macworld.com/article/673295/how-to-use-macbook-with-lid-closed-stop-closed-mac-sleeping.html)
- [How to keep your Mac awake, even when the lid is closed — 9to5Mac](https://9to5mac.com/2026/06/12/how-to-keep-your-mac-awake-even-when-your-macbook-lid-is-closed/)
- [How to Keep Your MacBook Running 24/7 for AI Agents (Even With the Lid Closed) — BlitzMetrics](https://blitzmetrics.com/how-to-keep-your-macbook-running-24-7-for-ai-agents-even-with-the-lid-closed/)
- [Running VoiceMode with MacBook Lid Closed](https://glama.ai/mcp/servers/@mbailey/voicemode/blob/61135913f46bd6d8612aa7d401c5c051e87b84e1/docs/guides/macbook-portable.md)
- [Keep Your Mac Awake While Claude Code Runs Locally — andrewbaker.ninja](https://andrewbaker.ninja/2026/04/11/keep-your-mac-awake-while-claude-code-works/)
- [Using VibeTunnel to control Claude Code instances remotely](https://www.andreagrandi.it/posts/using-vibetunnel-to-control-claude-code-instances-remotely/)
- [Happy — Remote Control for Claude Code & Codex](https://happy.engineering/)
- [Claude Code Remote Control — official docs](https://code.claude.com/docs/en/remote-control)
- [pmset reference — ss64](https://ss64.com/mac/pmset.html)
- [Power Management in detail: using pmset — The Eclectic Light Company](https://eclecticlight.co/2017/01/20/power-management-in-detail-using-pmset/)
- [pmset — disablesleep, Taylor Price](https://drpebcak.svbtle.com/pmset-disablesleep)
- [One-time privilege escalation — Apple Developer Forums](https://forums.developer.apple.com/forums/thread/768765)
- [How to perform actions as root from GUI apps on macOS? — Apple Developer Forums](https://developer.apple.com/forums/thread/773025)
- [Demystifying root on macOS, Part 3 — scriptingosx](https://scriptingosx.com/2018/04/demystifying-root-on-macos-part-3-root-and-scripting/)
- 本机：`man pmset`、`man caffeinate`、`pmset -g` / `-g custom` / `-g ps` / `-g therm` /
  `-g assertions`、`sw_vers`、`csrutil status`、非 root 的 `disablesleep` 探测、
  `/Library/Preferences/com.apple.PowerManagement.plist`。
