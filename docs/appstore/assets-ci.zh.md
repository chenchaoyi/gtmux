# 在 Actions 中采集 App Store 素材

本机磁盘不足时，从 `main` 运行 **App Store screenshots (maintainers)**
（`ios-store-assets.yml`），指定已正式发布的 `release_tag`。
流程核对发行版及其主分支 CI，再从准确的源码提交构建模拟器 App。
不需要签名、分发服务或 App Store 凭据。

流程复用现有演示采集与排版脚本，在专用 iPhone 17 Pro Max 和 iPad Pro 13-inch
模拟器上分别采集中英文，只删除本轮创建的模拟器。产物包括 22 张商店截图、
两张创意图、演示原图，以及记录源码提交和图片哈希的清单。锁屏图仍由脚本绘制，
不能当作真实推送测试。

```sh
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z
```

私下下载 `appstore-assets-<tag>-<run>` 产物，逐张按原尺寸检查。
将 `screenshots/<locale>/`、`creative/<locale>/` 复制到
`mobileapp/fastlane/` 对应目录，并将清单留在发布证据中。
按[上架流程](submit.md)放置并读回四组截图，以及中英文的页头和搜索结果四个位置。
本流程不向 App Store Connect 上传或提交审核；这些图片也不代表已完成真机、
VoiceOver、APNs 或实时活动验收。

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

当前可见的演示入口英文为 **No server handy? See a demo**，中文为
**尚未连接？看看演示**；英文读屏名称以 **Not connected?** 开头。
审核说明应引用可见卡片文字；测试匹配本地化的 **See a demo / 看看演示** 动作。
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
