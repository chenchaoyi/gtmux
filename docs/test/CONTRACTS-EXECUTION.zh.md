# 客户端契约与隔离执行

本文补充[分层测试体系](TEST-PLAN.zh.md)。评审用例与源码入口仍收录在
[用例目录](CASES.zh.md)；静态数量不是运行通过数。

## 隔离 HTTP 契约（L2）

`tests/contracts/http-contract.json` 保存合成存储输入、HTTP 请求及真实 Go handler
的响应基准。`TestClientHTTPContracts` 在临时 HOME 中，通过注入存储执行生产鉴权路由。
普通 Go CI 比较已提交的响应；确需改变协议时，先评审，再用显式
`GTMUX_UPDATE_HTTP_CONTRACT=1` 更新。不能为了消除失败直接刷新基准。
HQ 读取使用隔离存储执行实际应用投影，只固定总览中的两个动态时间字段，不会启动真实 tmux daemon。

RN 的 `e2e/fake-serve/backend-contract.test.ts` 和 Web 的 `backend-contract.test.cjs`
各自根据相同的领域输入提供服务，再比较实际 HTTP 响应。两个假服务都不读取期望响应；
共用辅助代码只负责请求与断言。覆盖 Trace 原轮次、历史与过期游标、上下文主动请求及
访客过滤、条件读取、空列表、HQ 总览/详情与所有者权限、文档元数据及可选 home 的省略、
下载字节与安全响应头、未知请求字段和未鉴权访问。反例检查会拒绝错误类型、缺失字段或
响应头、权限及轮次变化。

原有可选真实服务形状检查继续独立存在；没有显式环境配置时仍跳过。新增隔离契约无需
用户服务、凭据、真实 pane 或原生构建。

## 挂载交互（L2）

新增页面用例挂载生产 Detail/Composer、HQ 总览/事项/对话/Composer，以及
AgentsProvider/ManageMacScreen。通过合成客户端和延迟 Promise，验证服务器/会话草稿
隔离、旧响应迟到、实际订阅通知、待决进入原事项及准备答复后发送。权威数据确认答复
已保存后，卡片退出待用户决定区域，转入 HQ 跟进，不把持续事项标为已完成；跟进中
仍可保留原问题文字。选择选项本身不会发送。新增回归同时发现并修复了 HQ 预填答复跨服务器/会话携带的问题：导航和预填现在沿用输入框的归属身份，不把旧问题带进新的对话。

## Android 能力分块（L4；调度策略为 L1）

默认 `npm run test:e2e:android` 在专属手机/平板模拟器中保留全部 31 条既有 UI 检查。
`GTMUX_ANDROID_E2E_BLOCKS=trace,hq,documents,permissions` 可选择对应能力；完整名称见
`e2e/android/journey.cjs`。开发子集 `GTMUX_ANDROID_E2E_ONLY=08` 会自动补齐前置 06/07。
空选择或未知名称会失败；CI 继续跑完整旅程。

每组由原有的 QA App 启动步骤进入，或使用显式启动/入口准备，不依赖上一组停留页面。
折叠历史、HTML 返回后进入 Markdown 等依赖均明确登记。准备或前置检查失败时，后续
标为 `blocked` 并关联首因；驱动断连只记录一个根故障，之后标为 `not-run`，均不算通过。
普通断言失败后，独立分组仍可执行。没有自动重试掩盖失败，只重置明确指定的专属模拟器
及 QA App。

`RESULTS.json` schema 2 保留 `results`，增加选择、能力/分组、`blocked_by`、动作/证据耗时
和阶段指标，包括安装 APK、建立驱动会话、启动 App、滚动寻找、读取控件树、取证及点击
等待。这些指标存在包含关系，例如取证中也读取控件树，不能相加当作总耗时。原断言、
成功截图/控件树和失败时的独立取证均保留。本轮不声称 Android 已提速。

## 浏览器进程复用与 CI 范围（L3 + G）

每个 Node 测试 worker 复用一个 Chromium 进程，每条用例仍新建 BrowserContext、页面、
追踪及合成 HTTP 服务，成功或失败都关闭。进程崩溃后只供后续用例重建，不重试当前
失败用例。实际浏览器用例检查 cookie/存储与服务状态隔离。
`GTMUX_WEB_FRESH_BROWSER=1` 保留独立进程对照方式；`GTMUX_WEB_TIMING=/绝对输出目录`
写入各 worker 耗时 JSONL。失败截图、追踪及控制台/标题检查保持。

只有 `mobileapp/src/**/*.test.ts(x)` 可能省去原生门禁，而且必须由 `ci.py` 核对不可变
提交中的完整 Metro 配置摘要，确认产品打包器禁止导入测试模块。这类变更仍跑完整 RN、
Web 及始终执行的策略门禁；证明缺失则保留原生检查。E2E/假服务、契约数据、构建配置、
依赖、混入产品代码及未知路径沿原有更广范围执行，不能仅凭测试文件名跳过产品构建。

模拟器或组件结果不代表真机触控、VoiceOver/TalkBack、硬键盘、原生文档渲染及 Safari
已验收。
