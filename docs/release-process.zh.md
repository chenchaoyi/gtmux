# 发版准备与耗时分析

本文是维护流程，不是新的产品发版命令。继续按 PR、最新候选 CI、合入、实际 main CI、
正式附注标签的顺序执行。CLI、Mac 和选定的移动端交付使用同一份已审源码。签名、公证、
私有安装入口检查、实际安装分别保留回执。

## 在第一轮发版 PR 前完成准备

公开任何任务提交前，先检查实际作者和提交者身份：

```sh
python3 mobileapp/scripts/release-preflight.py --identity-only
```

检查只接受项目公开身份 `gtmux@ccy.dev` 或 GitHub no-reply 身份，不打印配置中的地址。
若仓库继承了工作邮箱，可用 `GIT_AUTHOR_NAME`、`GIT_AUTHOR_EMAIL`、
`GIT_COMMITTER_NAME`、`GIT_COMMITTER_EMAIL` 为本次操作选择公开身份，不暗改共享或
全局 Git 配置。平台预检也包含这一项；它防止新增暴露，不会擦除已经公开的提交元数据。

1. 对照上一正式版本和用户要求核范围。iOS 使用 `mobileapp/scripts/delivery-scope.py`，
   Android 独立核对实际输入变化。只有 CLI/Mac 改动时不重建手机包。
2. 为选定的平台写双语说明。Apple 元数据与 `mobileapp/release-notes/android.json`
   是独立历史，不能自动互相复制。
3. 直接填写计划版本，不再为准备源码创建临时标签：

   ```sh
   bash mobileapp/scripts/set-version.sh --version X.Y.Z
   python3 mobileapp/scripts/release-preflight.py --version X.Y.Z --platform ios --platform android
   ```

   只选择实际交付的平台；CLI/Mac 专属版本使用 `--platform none`。预检读取版本、原生
   marketing version、双语说明、Apple 生成历史及源码指纹；不修改文件、不创建标签，
   也不替代设计检查、CI、签名和交付范围判断。
4. 填版本后再跑正常检查，将说明和生成文件一起提交，再开 PR。创建正式附注标签前，
   重新核远端版本占用和活动流水线。

## 构建一次，分别验证

Android CI 只构建、lint 和测试一次 QA APK。手机和平板任务下载同一流水线的包，核对
精确源码和 SHA256 后，各自执行完整用例。两边都必须通过，证据分别保留，一边失败
不会取消另一边。仅重跑失败任务时，使用原成功构建的输出，不按新 attempt 猜产物名。
Android 交付门禁兼容旧版完整的合并检查和新版三个完整检查，旧的已审源码仍可使用。

Mac 签名、公证和手机签名要求保持。Mac 构建与 CLI 检查、构建并行；独立上传任务等待
两边成功，核对构建任务传下来的 ZIP、源码、ref、run、原构建 attempt 和 SHA256 后，
才上传 App、更新 cask。官网发行镜像再等上传任务成功。仅重跑失败上传时，使用原成功
构建的产物 ID 和 attempt，不借用别次构建的 ZIP。

Mac 公证工具先确认 Apple 返回的 JSON 状态为 `Accepted`，再下载票据。已实测的
CloudKit 连接中断（`stapler` 返回68且包含 `NSURLErrorDomain Code=-1005`）只重试
同一个包的票据查询，最多三次，间隔两秒、五秒；不重试签名或公证提交，永久错误
立即停止。最后仍必须通过票据验证，避免为短暂下载故障重建整个 App，也不会放行
被拒绝或未附票据的包。

获准推送正式附注标签后，立即同时启动实际需要的 iOS 和 Android 交付。iOS 可先对
精确且 main 检查通过的源码构建，原有受保护环境审核保持；上传安装包前再次核正式
发行、远端附注标签、最新 main CI 及同源码最新 CLI 任务成功。若 CLI 尚未完成，
只读等待最多十五分钟；失败不上传，包和诊断继续加密保存。此改造本身不授权新标签、
发版、手机出包或 Store 提交。
无签名 iOS CI 与受保护交付统一采用标准 ARM `macos-15` 镜像、显式 Xcode26.3，避免
`macos-latest` 别名随镜像迁移变化；预检拒绝其它 CPU 架构。CI 明确编译 ARM64 模拟器目标，并核实际 App 和两个
嵌入扩展的可执行文件，不使用签名密钥。候选和 main 都要真实编译通过。标准 Intel
试验超过四十分钟，没有达到提速目标，未作为默认配置或正式恢复入口交付。不能为了省时间跳过最新 main CI、保护规则、缓存信任或签名校验。

## 预期收益与剩余耗时

171 的 CLI 用时 6分07秒，之后 Mac 用时 7分54秒，镜像 39秒，合计 14分47秒。
并行后桌面路径预计减少约六分钟，再扣除产物传递和上传任务启动开销。iOS 在标签
推送后 6分32秒启动，用时 15分43秒；提前构建可以覆盖其中大部分初始等待。两项
收益互相重叠，不能相加宣称整体快十二分钟。这是调度估算，不是新签名发行的实测。

171 的 iOS 归档、导出仍用 10分28秒，Android 正式构建用 7分33秒，手机和平板的
完整自动化、runner 容量和环境审核也仍有实际成本。候选和实际 main 的检查保持；
不能把 QA APK 当正式包，也不能跨源码、版本或描述文件复用签名归档。编译缓存要
先明确输入及信任边界，单次缓存命中较快不足以证明等价。

## 统一取回加密结果

先在任务目录创建空的私有目录（`umask 077; mkdir ...`）。当工作流和交付源码为同一
精确提交，且任务成功时执行：

```sh
python3 mobileapp/scripts/fetch-delivery.py --platform ios \
  --run RUN_ID --attempt ATTEMPT --artifact ARTIFACT_ID \
  --source FULL_COMMIT --output PRIVATE_RESULT_DIR
```

Android 使用 `--platform android`。认证来自既有 `GH_TOKEN` 或 `gh auth token`，
不要将凭据写进参数。命令核最新精确 main CI、工作流/run/attempt、完整成功任务、
产物身份和有效期、官方长度/SHA256，以及 ZIP 中唯一的普通 age 加密文件；只输出
加密数据与 `VERIFIED.json`，不解密、不签名、不上传、不安装、不发链接、不重试写操作。
有意构建不同于工作流 checkout 的历史提交时，需要单独核来源；工具不会猜这种关联。

仅对暂时性的 GET 错误及 HTTP 429/502/503/504 最多重试三次。正常 TLS 证书和主机名
检查始终开启。若当前证据表明地址族有问题，可用 `--ipv4` 仅修改该进程的解析。
GitHub 凭据不会转发到产物存储地址，不改系统代理、DNS、TLS 或认证配置。

同一目录再次执行会重新核元数据和缓存字节，避免重复下载大文件。残缺或损坏的缓存
会失败并保留，检查后另选私有目录；不要把解密结果混入下载目录。解密、包签名、权限、
源码、原生符号和私有发布回执，仍按原平台文档分别验证。

## 先分清耗时，再调整流程

分别记录准备返工、PR/main 最长检查路径、runner 排队、受保护环境审核、构建/导出/
公证、下载重试、安装、证据和回收。runner ID 为零、没有任何步骤的任务没有在编译。
API 中的 job started_at 也可能包含审核等待，必须对照审核和真正取得 runner 的时间。

169 这版的实测基线：

| 阶段 | 耗时 | 原因 |
| --- | --- | --- |
| 最终 main CI | 约 31 分钟 | Android 构建 8分19秒，手机 9分58秒、平板 10分51秒串行 |
| 补版本说明后的 CI | 候选约 27 分钟、main 约 31 分钟 | 首次遗漏 Android 独立说明，新的提前预检可拦住这类遗漏 |
| CLI 正式发行 | 6分19秒 | 包含完整发版检查，继续保留 |
| 取得 runner 后的 Mac 签名任务 | 6分17秒 | 此前一次没有取得 runner，等了 15分02秒 |
| 取得 runner 后的 iOS 任务 | 15分56秒 | 此前一次没有取得 runner，审核与排队另计 |
| Android 生产构建 | 9分12秒 | 独立交付任务，不是模拟器测试时间 |

模拟器并行后可去掉一次串行等待，这个样本约十分钟；实际收益还要扣除产物转移、
依赖安装和排队开销。以新候选和实际 main 的实测为准，不承诺解决云端容量或间歇 TLS。

任务未取得 runner 时，先看原始提示，确认上一轮结束后只重跑失败项。参考 GitHub
[失败任务重跑文档](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/re-run-workflows-and-jobs)。
保留成功产物；重跑沿用原 ref 和 SHA。以后若迁移 runner，应对照官方
[runner 清单](https://github.com/actions/runner-images)，另做原生构建和签名链路验证。

下载运输副本和编译中间产物留在任务存储，只将独有恢复材料和简明证据按原保留规则
归档。流程改动不会授权删除旧的受保护档案。

浏览器改动先运行 `node --test internal/server/web/*.test.cjs`，再运行 `npm test --prefix tests/web`。VM 逻辑测试环境必须加载与真实页面相同的共享生成模型；浏览器渲染通过不代表测试环境的依赖完整。`make check` 是核心门禁，不包含这两层独立浏览器检查。

首次打开 PR 前，同时核对当前用户指南与设计文档中的相关操作。只更新设计依据会让官网继续显示旧说明，也容易在实现已经就绪后追加提交、重跑整轮 CI。

## Mac runner 的可用性

Mac CI 与签名构建使用标准 `macos-15-intel` 镜像，并显式选择已安装的 Xcode26.3，避免
镜像默认旧工具链造成差异。CI 仍为 arm64、x86_64 编译 App
及 Widget 宿主／扩展，在 runner 上运行全部原生测试，并核设计和资源。正式产物仍是
双架构包，签名和公证门禁保留。iOS 保持 ARM，固定标准 `macos-15` 与 Xcode26.3，
另经完整原生编译与实际交付验收，不迁到 Intel。
这一选择针对 ARM 作业尚未取得 runner 就失败的实际问题；不承诺排队时长，也不替代
下一次正式发版的实测耗时。
