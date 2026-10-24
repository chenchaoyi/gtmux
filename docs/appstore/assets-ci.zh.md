# 在 Actions 中采集 App Store 素材

从 `main` 运行 **App Store screenshots (maintainers)**（`ios-store-assets.yml`），
指定正式发布的 `release_tag`。流程核对发行版与主分支 CI，不需要签名、分发或 ASC 凭据。
[English](assets-ci.md)

流程拆成三个独立任务：

1. **build**：只构建一次准确的发行版。公开附件 `store-simulator-<run>-<attempt>`
   包含未签名的模拟器 App，以及源码、工具链和内容哈希清单。
2. **capture**：直接安装成品，不安装 Pods、不重做原生构建。专用 iPhone 17 Pro Max、
   iPad Pro 13-inch 分别采集中英文。每完成一组就保存哈希与原图，失败时也上传
   `store-checkpoint-<run>-<attempt>`。只关闭和删除本轮创建的模拟器。
3. **render**：核对四组原图，用当前已审核的脚本和标题生成 22 张截图、两张创意图。
   widget 绘制读取发行版源码。`appstore-assets-<tag>-<run>-<attempt>` 包含原图以及分开记录的采集/排版来源。

```sh
# 全新采集
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z
# 复用已结束的失败或成功运行中的构建和有效截图组
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z -f reuse_run=RUN_ID
# 仅改标题/排版：不构建 App，不启动 Appium 或模拟器
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z \
  -f reuse_run=RUN_ID -f stage=render-only
```

只允许复用本仓库主分支上、由同一工作流手动触发且已结束的运行。
要复用构建，应选采集运行；仅排版运行会转存原图检查点，但没有构建附件。
源码、Xcode、架构及 App 哈希必须匹配；构建不匹配时明确停止，需重新构建。
采集输入变化会使已完成标记失效；已记录文件缺失或损坏时会报错，不能继续当作有效原图。
仅排版必须有四组完整且版本一致的原图。附件 **7 天**到期，必要附件缺失或过期时需重新采集；
已验收原图和清单应另作私有长期留存。部分检查点不能当作最终素材清单。
当前标题改动可以重新排版已验证的旧原图，保留原始来源且不改变发行版 App。

下载成品后逐张按原尺寸检查，将截图与创意图目录复制到 `mobileapp/fastlane/` 对应位置，
保留来源清单，按[准备与读回指引](preparation.zh.md)继续。
锁屏图仍由 widget 定义绘制，不能作为真实推送验收。本工作流不会修改 ASC 或提交审核，
也不能证明真机、VoiceOver、APNs 或实时活动验收通过。
公开产物不能包含真机 IPA、profile、凭据或真实会话数据。

运行器从 npm 注册发行版 lockfile 指定版本的 XCUITest 驱动。不要对已安装的
`node_modules` 目录使用 `--source=local`：Appium 会把它当作源码目录并执行
`prepare`，而 npm 发布包不含重新构建所需的 TypeScript 工程，会报 TS5083。

商店包另行准备。本机磁盘不足时，复用成品所需的签名和验证步骤见
[复用已验证的发行版 IPA](reuse-ipa.zh.md)。

## 采集用例与排障

App 从已发布的 tag 构建。已审核的 workflow 只提供两份采集用例、演示选择器、
模拟器能力配置和会话启动文件，不覆盖产品源码。清单分别记录 App 与用例的提交及用例文件哈希。
这样可以修复采集流程，而不改变已发布 App。单元测试会渲染真实配对页，核对演示选择器。

云端保持 simctl 已启动的模拟器以无窗口模式运行。未设置 `isHeadless` 时，Appium
会将它关闭，再尝试显示 Simulator 窗口，曾导致云端重启超时。
本地 e2e 默认仍显示窗口，需显式选择才使用无窗口模式；采集 helper 拒绝在共享本机执行。
结构化进度标明设备和语言阶段；失败产物显式包含白名单内隐藏的
`.e2e-artifacts` Appium 日志和演示 PNG。

当前可见的演示入口英文为 **View demo**，中文为
**查看演示**；读屏名称与可见操作一致。
审核说明应引用可见卡片文字；测试匹配本地化的 **View demo / 查看演示** 动作。
额外采集的 `00-demo-entry.png` 用于核实审核入口，只作为证据，不增加商店截图数量。

云端先在独立进程中编译 WebDriverAgent，再启动 Appium，使用 `usePrebuiltWDA` 复用成品。
WDA 15.0.0 不应使用 `prebuildWDA`：编译进程退出后，它保留了退出标记，
下一次启动会直接跳过服务就绪等待并拒绝会话。运行 38913607022 的日志确认，
编译耗时三分钟并成功，随后才出现此问题；[对应版本源码](https://github.com/appium/WebDriverAgent/blob/v15.0.0/lib/xcodebuild.ts)
也显示退出标记没有重置。依赖无论嵌套还是提升到上层，都从本轮 Appium 目录解析，
编译输出保留在 `wda-build-<device>.log`。
运行 38907980603 在就绪超时后反复终止冷启动的 xcodebuild；由于未开启 Xcode 输出，
当时无法进一步判断编译还是启动出了问题。现在仅云端采集开启 Xcode 日志，
设置 `wdaLaunchTimeout=240000`，单次连接总限时 15 分钟，取消自动重复连接，避免构建重叠。
普通本地 e2e 的默认行为不变。超时仍记为采集失败，不能视为模拟器、App 或截图通过。

云端多页面采集用例的总时限为五分钟，各控件仍保留较短的等待时限，普通本地测试
仍为两分钟。运行 38920162585 已拍完六个手机页面，随后被旧的两分钟总时限中断。
用量页直接通过可见的「完成」按钮关闭，不再等待不存在的详情返回按钮。
每张原图写入后都会记录名称和时间。

排版任务响应取消，不使用无条件的 `always()` 保持执行。检查点和排障证据会尽可能在清理阶段上传；
强制终止运行器可能打断上传，因此不能把缺失的检查点当作已保存进度。

排版成品只包含检查点记录并核验过的原图及发行版 widget 绘图，不会复制目录中未记录的旧文件。
