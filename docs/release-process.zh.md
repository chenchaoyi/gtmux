# 发版准备与耗时分析

本文是维护流程，不是新的产品发版命令。继续按 PR、最新候选 CI、合入、实际 main CI、
正式附注标签的顺序执行。CLI、Mac 和选定的移动端交付使用同一份已审源码。签名、公证、
私有安装入口检查、实际安装分别保留回执。

## 在第一轮发版 PR 前完成准备

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

Mac CLI/App 发布、iOS 受保护签名与 Android 生产签名维持现状。不能为了省时间跳过
最新 main CI，也不能只凭一次排队超时就换 runner 架构、保护规则、缓存信任或签名身份。

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
