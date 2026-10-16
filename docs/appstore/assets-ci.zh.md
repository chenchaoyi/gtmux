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
