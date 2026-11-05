# 测试体系

[English](TEST-PLAN.md) · [用例目录](CASES.zh.md) · [设计工作流](DESIGN-WORKFLOW.zh.md) · [设计跟随矩阵](DESIGN-TRACEABILITY.zh.md)

**测试分层按验证的边界划分。** Go、React Native（RN）、Swift、Cloudflare Worker
是实现技术或运行环境，不是并列的测试层次；同一种技术可以出现在多层。
构建、静态检查、截图和真实设备操作，各自能证明的事情不同。

## 测试分层

| 层次 | 要回答的问题与可观察结果 | gtmux 中的例子 | 环境与限制 |
| --- | --- | --- | --- |
| L1：单元／纯逻辑 | 独立的解析器、模型、策略，在正常、边界和错误输入下结果是否正确？ | Go 分类、历史分页、恢复策略；RN Trace 模型与格式化；Swift HTML/Markdown 预处理；Android 文件策略 | 合成数据，按需注入时钟/网络；不证明屏幕或原生手势正确 |
| L2：组件／契约 | 一个组件或接口边界是否正确消费数据、呈现状态并执行动作？ | RN 组件渲染与按钮回调；Go `httptest` HTTP 契约；Worker 请求响应和 APNs/FCM 载荷；Swift store 注入假客户端 | RN renderer 不是已安装的 App；路由测试不代表整个后台服务已启动 |
| L3：运行时集成 | 多个部分在真实运行时中能否配合？ | Chromium 加载真实 HTML/JS/CSS，连接合成 API；Cloudflare runtime；明确运行的原生 bridge 集成 | 浏览器假 API 不等于真实 Go 后端契约；Foundation 命令行断言属于 L1，不是 WKWebView 操作 |
| L4：应用操作流程 | 用户能否通过已编译 UI 与设备驱动完成流程？ | Android 手机/平板模拟器旅程；显式启用的 iOS Appium 自动回归 | 合成后台可以验证 App 操作，不证明生产推送、真实 agent 输入或全部系统权限 |
| M：人工／真机验收 | 实际排版、触控、焦点、无障碍播报和系统集成是否可用？ | iPhone/iPad、Safari、VoiceOver/TalkBack、硬键盘、旋转分屏、Ghostty 焦点 | 记录设备、源码/构建、步骤、结果与限制；截图或控件树不能代替操作/播报证据 |
| G：质量与交付门禁（横向约束） | 源码或产物是否满足规则？ | 类型、lint、race/漏洞检查、严格规格、中英双语/生成文档、源码封印、架构、签名/公证/验包 | 可保护任意层；编译、正则检查或签名通过不等于 UI 行为正确 |

优先在能发现问题的最低层验证；若真实故障出在集成或最终呈现，再补高层回归。
保留有意义的旧 provider 兼容与权限/身份回归。固定的金字塔比例或测试数量不能代替有效断言。

## 各产品端与技术栈属于哪里

| 实现／产品端 | 常见层次 | 实际入口与源码 |
| --- | --- | --- |
| Go 核心、CLI、HTTP、HQ/恢复 | L1 + L2；隔离进程/服务集成 L3；race/构建/策略 G | `go test -race ./...`；`internal/**`、`cmd/**`、`relay/**`；Linux 服务验收自建临时账号 |
| RN 共享 App（iPhone、iPad、Android 手机/平板） | 纯 TS 为 L1；React 组件为 L2；bridge 集成为 L3；实际 App 操作为 L4 | `mobileapp/src`、Jest；Android 旅程与显式 Appium。RN 是共享框架，不是 UI 和 Go 之间额外的一层测试 |
| iOS Swift／Android Kotlin、C++ | 逻辑 L1；明确执行的平台/bridge 集成 L3；App L4；构建/验包 G | Foundation 文档断言、Widget 目标检查、Android JUnit/lint/QA 验包、无签名 ARM64 App/扩展编译 |
| macOS 菜单栏／Widget | 模型 L1；store/组件 L2；真实窗口/系统操作 L3/M；通用架构构建 G | `macapp/Tests`、`shared/UsageWidgets`；实际焦点与视觉另验收 |
| 浏览器 | 模型 L1；真实 DOM/浏览器运行时 L3；显式真实主机验收 L4/M | `internal/server/web` 的 Node 测试、`tests/web` 的 Chromium |
| Worker（推送中继、隧道、下载） | handler/契约 L2；Worker runtime L3；另行授权后的部署 smoke；类型/策略 G | `relay-worker`、`tunnel-worker`、`download-worker`、`shared/worker-json.test.mts`。Worker 是边缘服务运行环境，不是测试层次 |
| CI、发布器、文档、测试工具 | 工具本身的 L1/L2；所执行规则为 G | `scripts/*_test.py`、`mobileapp/scripts`、Ruby/Shell/原生检查入口、`internal/docs` |

浏览器和 RN 复用同一份语义行为表
[`tests/contracts/client-behavior.json`](../../tests/contracts/client-behavior.json)，
分别在各自真实 renderer 上断言：五类 agent、原始第 151 轮、数量、默认折叠、单条调用、
HQ 发言身份、Markdown 和会话范围内的文件动作。只比较共享函数不算最终 UI 一致。
这些用例不虚构浏览器 HQ 事项页，也不冒充原生 WK/手势验收。

## CI 各阶段执行什么

权威入口为 [CI](../../.github/workflows/ci.yml)、[影响选择器](../../scripts/ci.py)
与以下六份 workflow。矩阵说明执行规则，不代表某次版本已通过。

| 阶段 | 执行的测试／检查 | 成功不代表什么 |
| --- | --- | --- |
| PR 到 main | 总是运行范围选择、quality、`ci-required`，以及下表中受影响的平台；设计历史检查要求评审提交早于第一次测试修改 | 不签名、不部署、不安装，也不隐含人工批准 |
| 实际 main push | 同一套保守选择与所选门禁；squash 后检查最终设计/目录快照；单独绑定实际合入 SHA | PR 通过不能替代此轮；squash 形成的单个提交无法保留此前开发提交的时序 |
| 手动 dispatch `ci` | 全部平台、quality 与汇总 | 不发布或安装 |
| tag／正式发行 `release.yml` | 校验不可变源码/流水线；Go 安全与完整 `make check`；CLI 交叉构建；通用架构 Mac 签名、公证、staple 验证；生产作业成功后发布；签名下载源与镜像检查 | CLI/Mac 发布不代表 iOS/Android 更新或实际装机 |
| iOS 测试交付 `ios-test-delivery.yml` | 精确 main/tag 及对应成功 main CI；按真实输入差异跳过未变 iOS 包；受保护签名/发布器预检、构建/归档/导出及产物校验；preview 与 release-test 模式分别处理 | 不提交 Store；IPA 上传或 Safari 链接不代表已安装/UI 通过 |
| Android 测试交付 `android-test-delivery.yml` | 精确 main 来源、原生和两尺寸模拟器成功；构建序号/输入；生产未签名构建与加密产物；后续由执行者按交付流程验签/发布 | QA APK 不等于正式签名包；上传不等于真机或 Play 上线 |
| Store 素材 `ios-store-assets.yml` | 校验来源；构建/复用合格模拟器宿主；补拍缺少的语言/设备组，保存检查点并渲染/验证素材 | 截图是素材生产，不自动代表功能/视觉批准或 Store 提交 |
| 镜像恢复 `release-mirror.yml` | 校验已发布版本、冻结的安装器/源码、签名源和信任锚；同步既有不可变产物 | 不重建产品；不能替代原始发行/安装证据 |

当前仓库没有定时运行全部 UI 的 nightly workflow。更长的探索和真机测试须另记，
不能暗示每个 PR 都会执行。

### 受影响的 CI 作业与命令

| 作业 | 检查内容与层次 | 选择条件／环境 |
| --- | --- | --- |
| `quality` | `make ci-test publisher-test`、actionlint、`check-design.sh --source-only`（含设计/目录与双语）、严格 OpenSpec、生成资源/文档、download-worker test/build、`go test ./internal/docs` | 每个 PR/main/manual；自有运行需要私有精确词策略，fork 明确缺少该覆盖 |
| `test` | gofmt/vet/staticcheck、隔离 tmux 前提、Go race/coverage、Linux 用户服务、漏洞协议与设计检查（L1/L2/L3 + G） | Go/core、受影响 Web/relay；影响未知则全跑 |
| `cross-build` | CLI 无 cgo 交叉编译 | 核心/共享影响；是构建门禁，不是运行测试 |
| `mobile` | npm ci、tsc、eslint、完整 Jest、交付工具与 creative-layout（L1/L2 + G） | RN 或下游影响；mock bridge 不证明原生体验 |
| `web` | 完整 Node 逻辑与 Chromium 实际渲染（L1/L3） | 浏览器资源、测试或移动端共享呈现变更 |
| `ios-native` | `check-native-documents.py` 执行生产 Swift HTML/Markdown 断言（L1）；Ruby Widget 边界；Release ARM64 App 与两个嵌入扩展无签名编译/架构检查（G） | iOS/RN/共享变更；不启动设备、不做 UI 自动化、签名或上传 |
| `android-native` | `assembleQa testQaUnitTest lintQa`、源码封印、包/ABI/16KiB 检查（L1 + G） | Android/RN/共享输入变更 |
| `android-emulator` | 手机/平板矩阵使用相同源码封印的 QA APK，执行实际 UI 旅程（L4） | 与 Android 构建一同选择；不使用本机共享模拟器/ADB |
| `menubar` | Swift 通用架构构建、UsageWidgets 与菜单栏测试、Widget 构建/设计（L1/L2 + G） | 菜单栏或共享/未知影响；实际视觉/焦点另验 |
| `relay-worker` | 类型、共享 JSON/APNs/FCM 载荷与 handler（L2 + G） | 推送源码/测试与下游消费端 |
| `tunnel-worker` | 类型、Node 契约、Worker runtime Vitest（L2/L3 + G） | 隧道/共享/未知影响 |
| `ci-required` | 失败关闭汇总，包含两个 Android 矩阵结果 | 每次运行；必需作业缺失、失败、取消或跳过均失败；未选跳过不算通过 |

混合修改取并集。未知路径、依赖、构建/流水线、无效/缺失 diff、符号链接变化均扩大到全跑。
重命名检查新旧路径。详见 [CI 规则](../ci.zh.md)，不得为得到绿色结果手动缩小范围。

## 各层用例与设计—实现规则

[`CASES.zh.md`](CASES.zh.md) 提供分层入口，[`CASES.csv`](CASES.csv) 列出每个已盘点的
声明、子测试或参数模板，包含稳定 ID、层次、技术栈、文件/行、描述与评审来源。
[`catalogue.json`](catalogue.json) 固定源码摘要。数量是**静态声明位置**，不是展开后的
参数实例、实际执行数或通过数。存量层次是结构盘点，触及时须语义复核，不补造历史批准。

新增、修改、移动或删除测试均遵循[设计工作流](DESIGN-WORKFLOW.zh.md)：
先写可观察行为，评审并提交带封印设计，再实现、登记具体测试入口。`make test-design`
检查结果快照，PR CI 另验提交顺序。在 CI 检查前保留设计与实现的独立提交，不提前本地 squash。
最终正常 GitHub squash 后，main 检查最终快照。

封印用于防误改漂移，不是身份认证或“断言有价值”的证明。评审仍须检查需求、失败判据、
mock 边界与范围，并验证刻意构造的反例能使门禁失败。

## 人工验收与汇报

保留[产品设计矩阵](DESIGN-TRACEABILITY.zh.md)。Mac 验证状态图标、浅/深/着色菜单栏、搜索、
设置、键盘及精确 pane 焦点；手机/平板验证配对、HQ/知识、输入、原生预览、长 Trace、
旋转分屏和无障碍；浏览器验证窄/宽屏、配对撤权、发送失败保留草稿、live、历史锚点和外部会话。

截图/探针分别在 `mobileapp/e2e/capture`、`diagnostics`，普通 RN 与 Appium 回归不收集它们。
显式 `e2e:manual` 要求命名工具开关；发送诊断还需在访问设备前验证精确 pane 与本机假服务身份。
截图工具不能采纳知识条目。见[手工工具](../../mobileapp/e2e/MANUAL-TOOLS.zh.md)。

报告分别说明设计评审、代码实现、实际测试、条件跳过、仅编译、模拟器操作、真机/无障碍和安装。
不得沿用旧报告的通过状态，也不得把截图写成触控或 VoiceOver 已通过。
