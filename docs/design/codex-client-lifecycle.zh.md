# Codex Desktop 与 CLI 的会话生命周期

[English](codex-client-lifecycle.md) · **中文**

**方案待实施；调研日期 2026-10-12，核对版本 Codex 0.162.1。** 本文不是已上线功能说明。
现状见 [Desktop 跟进](desktop-follow.zh.md)、[对话阅读](desktop-conversation.zh.md) 和能力规格。
实施待办位于 `openspec/changes/codex-client-lifecycle/`。

## 1. 结论与证据

| 证据 | 可以确认 | 不能据此确认 |
|---|---|---|
| 活跃 tmux CLI 对话在 Desktop 提示被另一个应用打开 | 同一对话可以在 Desktop 看见，同时仍由 CLI 使用 | 截图不说明后台进程是谁，也不证明存在接管接口 |
| 已回收 HQ 任务仍在 Desktop 历史，输入框可用 | 回收 tmux 资源不等于删除对话 | 输入框可用不代表有回合运行，也不证明新 writer 已创建 |
| 任务先产生无 pane 的 SessionStart，后确认绑定 tmux，完成后回收；旧 native 记录未更新 | 临时记录被隐藏，pane 消失后重新出现 | 这是 gtmux 生命周期缺陷，与 Desktop 保留历史不同 |
| 本机 schema 将 originator 定义为创建来源 | 它不能可靠判断当前控制端 | cwd、标题、rollout source 也不能替代 |
| WriterLockCoordinator 拒绝同一 Codex home 的第二个进程 writer | 存在跨进程存储写入互斥 | 锁文件存在不等于被锁，也不说明前台是谁 |
| resume_running_thread 允许客户端加入已加载线程 | 共享后台重新加入与另一个进程取得 writer 不同 | 不能凭存储锁保证前台只有一个输入者 |
| 当前 CLI 使用 alternate screen；普通历史停止增长，当前屏幕 48 行 | 终端画面与持久对话历史不同 | 不能把旧普通缓冲区拼成完整终端历史 |

官方 [app-server 文档](https://learn.chatgpt.com/docs/app-server) 区分读取、运行状态、恢复、分叉、
取消订阅、归档和删除。[Desktop 命令](https://learn.chatgpt.com/docs/app/commands) 提供
`codex://threads/<thread-id>` 打开本地对话。这些不等于 gtmux 已有安全接管能力。
本次没有恢复真实对话、发送输入、分叉、取得锁、归档、删除或关闭真实客户端。

核对官方源码版本 `rust-v0.162.1`：

- `codex-rs/rollout/src/writer_lock.rs`、`writer_lock_tests.rs`：竞争、释放及残留的未锁文件。
- `codex-rs/thread-store/src/local/live_writer.rs`：创建/恢复取得 writer；关闭时完成写盘并移除运行记录。
- `codex-rs/app-server/src/request_processors/thread_processor.rs`：thread_resume_inner 与
  resume_running_thread；已有观察者时，冲突恢复参数不能随意重建线程。
- `codex-rs/app-server/tests/suite/v2/thread_resume.rs`：另一进程占用时拒绝，原进程关闭后恢复，
  覆盖 legacy 与 paginated 历史。
- `codex-rs/tui/src/app/tests/external_writer_fork_tests.rs`：分叉生成不同 ID，保留草稿，原 writer 不变。
- `codex-rs/tui/src/daemon_startup.rs`：共享 daemon 与内嵌后台的启动条件、兼容检查及例外。
  共用存储不代表共用后台。

只阅读测试，未运行 Rust 测试。本机静态 schema 导出成功：
`codex app-server generate-json-schema --experimental --out <私有目录>`。
真实 ID、提示词、日志和截图不进入公开仓库。

## 2. 拆开几种身份

对话唯一键采用 `(gtmux server 身份, Codex home 身份, thread.id)`。home 使用本机不透明标识，
不向访客暴露路径。后台身份属于运行连接；daemon 重启不生成第二份历史。
`thread.sessionId` 是会话树根，分叉可能共用它；去重必须用 thread.id。
gtmux 现有 session_id 经明确兼容层保持“已核实 thread ID”语义，不能悄悄换成树根。

| 对象 | 表示什么 | 不代表什么 |
|---|---|---|
| 对话 | 持久 ID、标题、父子/分叉关系、历史版本 | 客户端还活着 |
| 创建来源 | 最初 originator/source | 当前控制端或权限 |
| 客户端连接 | 后台、已知前台、精确进程身份、pane、观察时间 | 永久对话身份 |
| 存储 writer | 持有写入锁的后台 | 唯一前台或正在运行的任务 |
| 控制端 | 核实的输入通道与权限 | 根据创建来源猜出的客户端 |
| 回合 | turn ID、工作/等待/完成/失败 | writer 或进程是否存活 |
| HQ 任务 | 派发 ID、资源归属、角色、授权范围 | 对话永远属于 HQ |
| 跟进设置 | 观察、通知、知识采集许可 | 输入或接管许可 |

writer 空闲也可能存活；已加载线程可能只是缓存；保存的对话可能两者都没有。
缺证据显示“未确认”，不能显示“已完成”。canAcceptDirectInput 是后台能力，含子 agent 限制，
不证明 gtmux 有输入控制权。进程核对须区分前后台并防 PID 复用。

## 3. 应有的生命周期

| 场景 | 雷达与操作 | 历史与 HQ |
|---|---|---|
| CLI 在 tmux 运行，也在 Desktop 查看 | 一条对话，指向核实的 pane；跳转和输入沿用该 pane | Desktop 外部应用占用正常；查看不转移控制权 |
| tmux 外的终端 CLI | 有可信存活证据才计入；迁移另验控制权 | 有日志只证明可阅读，不能无限延长存活时间 |
| CLI 创建后由 Desktop 继续 | 有证据显示 Desktop，否则控制端未确认；不提供不安全迁移 | 新活动遵循 Desktop 许可；创建来源仍为 CLI |
| HQ 排查任务回收成功 | 结束任务资源连接，只清理失效临时 native 记录 | 保留对话，不自动归档/删除 |
| 用户之后在 Desktop 继续排查对话 | 新可信活动/连接可重新进入 Desktop 分组 | 不复活旧任务、HQ 角色、授权、推送或知识采集 |
| 两个真实连接冲突 | 同一对话明确显示冲突，pane 浏览器保留两个窗格 | 不静默选 owner，不按标题拼输出 |
| 分叉与原对话同名同树根 | 两条对话，注明关系 | 不按标题、cwd 或树根去重 |
| 后台断开/版本不支持 | 保留内容，提示观察不可用，禁用未确认操作 | 不伪造工作→完成，不发完成通知 |

回收成功以自有资源确已关闭为准。共享 daemon 中，前台退出不能证明后台 writer 或回合结束，
须分别记录，没有证据不声称 writer 释放。

## 4. 展示与操作设计

标题为主，一行简短副标题，沿用状态图标。如 `Codex · tmux %N`、`Codex · ChatGPT 桌面版`、
`Codex · 控制端未确认`。创建来源放详情，不当作当前客户端标签。纯历史不计入活跃数量。
HQ 标记只来自当前已核实 HQ 绑定。不默认将所有 Desktop 历史加入雷达。

| 操作 | 条件与行为 |
|---|---|
| 查看对话 | 核实 thread，限 owner/配对设备；不自动跟进、不产生输入副作用 |
| 跳转窗格 | 核实当前 pane，沿用 focus |
| 输入/批准 | 已授权 pane 通道，发送前再核实，不悄悄改投 |
| 在 Desktop 打开 | 本机 thread、URL handler、对应 home 已核实；仅 Mac 本机。打开不等于交接 |
| 跟进设置 | 观察/通知/知识分别授权；Desktop 或控制端未确认时不自动继承 CLI 监督范围 |
| 在 tmux 中继续 | 后续独立交接，不能用普通 spawn/第二个 resume 代替；未验证前不提供 |
| 分叉 | 后续明确的新对话操作，标明来源，不称为迁移 |

五端消费 Go core 证据与能力。菜单栏跳转本机 pane 或打开 Desktop。
手机/iPad 经配对 Mac 读取及请求既有授权 pane 操作，不在 iOS 打开 Mac URI。
Web 顺序和能力判断一致；访客仅访问授权窗格，不读任意持久对话。CLI 提供结构化证据/回执。
能力不存在就不显示操作，各 UI 不猜测。真机布局及 VoiceOver 仍需设备验收。

Chat 展示公开持久对话和中间回复。Terminal 展示真实输出：alternate 可能只有当前屏幕，
inline 从之后输出积累历史。在受限状态简短解释并提供 Chat，不能用 transcript 伪造 ANSI 历史。
已有客户端切模式需另行核实同 ID 重启方案并获用户同意。/tui 是下次启动设置；
本轮尚未证明 /raw 可安全退出 alternate screen。

## 5. Core 适配与证据

新增分版本 Go 底层模块，保持 cgo-free。优先使用已运行可信本地后台的核实能力。
不能为了填列表启动后台或恢复对话。核对版本/schema，限制时长、数据量、分页和取消。
只读通道仅允许元数据/历史，禁止 resume、turn、队列修改、配置、shell、归档和删除。
thread/read 不订阅；实时更新须用核实的只读观察或有界读取，不能拿 resume 冒充订阅。
用支持的 turns/items 分页，必要时回退现有有界 rollout 读取，并说明省略范围。

本轮 schema 无通用“当前前台 owner/转移控制权”接口。锁文件、loaded 列表、direct-input 能力、
resume 成功都填不上缺口。某个后台 notLoaded 也不证明另一进程无 writer。
不修改 Codex 锁或 SQLite 强行接管。控制权未确认时保留阅读、禁用接管。

按连接代次、turn ID 对齐证据，不只比时间。精确 tmux 绑定后的临时记录清理须条件更新，
保护后来的独立 native 活动。保存有界清理回执，支持旧记录核实及崩溃恢复。
继承 pane 环境不算 owner 证据。记录损坏/写盘失败须可诊断、可重试，不能静默成功。
reap 回执带核实的 thread、派发及已关闭资源身份，不记录消息正文。

诊断关联 server/home/backend、thread、连接代次、已核实 pane/进程、turn、dispatch，
以及来源/原因、前后状态、证据时间、结果/错误。串起绑定→清理→回收→Desktop 新活动，
不采集凭据、整套环境或提示词。通知按 thread/turn/结果去重；换客户端不是新一次完成。
发送时复核跟进和 server 推送许可；知识采集沿用授权时段与候选台账。

## 6. 分批实施与验收

| 批次 | 交付 | 验收条件 |
|---|---|---|
| A：生命周期 | 精确清理临时记录、reap 关联、有界核实旧案例 | 启动→绑定→完成→回收无幽灵；新 native 活动保留；缺证据不盲删 |
| B：身份/权限 | 拆开创建/当前控制，兼容字段/能力，统一标签菜单 | Desktop 续聊不继承 HQ/输入/知识许可；分叉/机器独立；旧端安全降级 |
| C：阅读 | 只读适配、有界历史/更新、诚实终端限制 | 查看不恢复、不占权、不输入、不改许可；迟到/错误响应不影响别的线程 |
| D：可选交接 | 同 thread 交接及现有 CLI 原位 inline 恢复 | 能证明独占控制才开放，否则明确不可用及原因 |

D 先隔离验证：自然空闲，无排队/未提交草稿，精确 thread/pane/进程；保留 model/effort/provider、
approval/sandbox/workspace、instructions 和非敏感启动设置。旧 owner 确认释放后才启动替代者，
绝不终止共享后台。恢复后比对有效设置、唯一控制端；inline 核实 alternate_on=0 和后续历史增长。
超时/冲突/部分失败保留恢复记录；回滚不启动副本、不扩大权限、不发送排队输入。
真实切换等操作可审阅后再由用户明确决定。

回归覆盖两类历史、两个后台进程、共享后台重新加入、分叉/树根、同标题同 cwd、不同机器/home、
继承 pane、乱序回合、PID 复用、绑定/清理写盘中断、重复回收、新 native 与 reap 竞争、
观察断开、旧 schema、分页省略、许可撤销和通知去重。新增测试须破坏实现确认能报红。
隔离测试不碰真实 Codex home/HQ。UI 验收含短标签、禁用操作、稳定计数、安全区、键盘及真机 VoiceOver。

## 7. 取舍与尚未核实项

先 A，再 B/C。D 独立调研并由用户审阅操作；公开协议还不足以承诺安全通用接管。
不默认归档回收任务的对话。未来自动归档须单独审阅对子线程与用户历史的影响。

未核实：Desktop 前台控制机制、已安装后台安全只读订阅、不同 home/旧版本兼容、
Desktop 专属工具和环境在 CLI 的恢复，以及真实跨端交接。截图/静态源码不代表这些已验收。
更新后 doctor 体验在独立开发分支的 `openspec/changes/post-update-environment-check/` 跟踪；
本方案不包含自动修复或已有客户端重启。
