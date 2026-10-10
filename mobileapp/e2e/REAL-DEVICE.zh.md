# iPhone 真机验收

[English](REAL-DEVICE.md) · [模拟器测试入口](README.md)

安装版本一致、启动成功、开发服务可用，分别记录；这三项都不能代替布局、交互或读屏验收。
每次记录 App 版本、设备型号、iOS/Xcode 版本、逐项结果、截图和未覆盖范围。

## 1. 检查一次，再选可用工具

先用 `df -h /private/tmp` 核对空间，再执行 `xcrun devicectl list devices`。
对选定设备读取 `xcrun devicectl device info details --device <设备>`，必要时再读
`xcrun devicectl device info lockState --device <设备>`。
不要后台轮询等待解锁。服务明确报告锁屏、未信任或开发者模式未启用时，说明需要用户做的具体操作。
`unlockedSinceBoot` 只表示开机后解锁过，不能证明现在没有锁屏。

| 工具 | 实测限制 | 处理 |
| --- | --- | --- |
| libimobiledevice 的 `idevicescreenshot` | 调用 `screenshotr`；iOS 26.6.2 上返回 `Invalid service`，同时 `ddiServicesAvailable:true` | 保留一次错误，不按其通用建议推断缺少开发镜像或未解锁；换用下列可用入口。 |
| Xcode Device Hub → View Screen | 这台 Mac 的 Xcode 27 提示屏幕共享要求 iOS 27+ | 记录界面的明确限制，不在 iOS 26 上反复重试。 |
| Appium/XCUITest + WebDriverAgent（WDA） | 同一台 iPhone、iOS 26.6.2、Xcode 27 上成功 | 用于截图、控件检查和限定范围的导航。WDA 是签名的轻量测试助手，无需重新构建 gtmux。 |

这些是 2026-10-10 的兼容性实测，不是永久的版本规则。工具或系统升级后，以新的实际错误为准。

## 2. 保留用户数据，隔离测试资源

使用独立的 Appium/WDA 端口、DerivedData 和证据目录。个人手机不能直接运行模拟器的 global setup：
它会回收进程并写入调试标志。不要清空配对、写 `gtmux-debug-flags.json`、自动批准确认弹窗、重新安装
App 或向工作中的窗格发消息。使用已配对 App 做只读导航；授权开关的修改在自有测试环境或 Demo 中验证，
不要为测试擅自开启真实会话的跟进、通知或知识记录。截图/XML 可能含对话和其他 App 的通知，保存在本地，
未经脱敏不提交仓库。

启动已安装的 Appium，使用空闲端口：

```sh
appium --port 14723 --log /private/tmp/<本任务目录>/appium.log --log-level info
```

向 `http://127.0.0.1:14723/session` 发 POST，结构为
`{"capabilities":{"alwaysMatch":<caps>}}`。使用明确选择的真机和已授权的签名团队。
已验证的能力配置见 [英文版 JSON](REAL-DEVICE.md#2-preserve-the-users-phone-and-other-workers)：
`noReset:true`、`autoLaunch:false`、独立助手 bundle ID、构建目录和 WDA 端口；不设置自动接受/取消弹窗。
助手启动允许有界等待；记录会话 ID、签名或启动错误，回执缺失时不要不断重新构建。
新增 provisioning 参数前先查已安装驱动的文档。webdriverio 测试需 Node 22（见 README）；临时验收也可用
直接 HTTP 客户端，避免该客户端依赖。

随后向 `POST /session/<id>/appium/settings` 发送
`{"settings":{"defaultActiveApplication":"com.gtmux.app"}}`，固定控件查询对象。
其他 App 的通知会令 WDA 查询转向 SpringBoard，即使 App 专属控件树仍显示 gtmux。
先固定查询目标并重读控件树，不把选择器失败当成产品缺陷，也不要盲点。

固定查询对象不能阻止系统通知截获触摸。手势前用 `mobile: activeAppInfo` 确认前台 App，
并查看当前截图；用户切换到其他 App 时停止设备操作。

`GET /session/<id>/screenshot` 返回 base64 PNG；`GET .../source` 返回 XML。
交互稳定后保存配套截图和控件树，查看画面再操作，优先使用 `testIds.ts` 的标识。
终端历史很长时 XML 会较慢，保持有界 HTTP 超时并记录未完成；缺属性或截断快照不代表控件不存在。

## 3. 验收与清理

- 顶部预览/全文：打开全文不扩大终端顶栏；正文独立滚动；关闭后回到会话。
- 列表：查看底部和全部折叠状态，核对分组数量及底部隐藏会话数；结束后恢复测试改变的折叠状态。
- 跟进设置：区分开启、关闭和禁用；切换时各行位置不跳。用自有测试环境验证保存，不擅自修改真实授权。
- HQ 浮窗：长按后分别进入用量和知识库，检查内容、关闭、返回；截图之外再核对标签和按钮角色。
- VoiceOver：实际焦点、播报和自定义操作单独验收。当前驱动的相关命令要求 iOS 27+；iOS 26 需用户在
  「设置 → 辅助功能 → VoiceOver」启用后人工检查。控件树能证明标签/角色，不能证明实际播报或焦点顺序。
- iPad、横屏、大字号、另一语言分别记录，不从一台竖屏 iPhone 推断通过。未装到设备的新改动明确记为待真机验收。

删除测试会话（`DELETE /session/<id>`），只停止本任务的 Appium，确认其 WDA 已停止后再移除本任务的助手
构建目录。证据与回执留待复查，汇报实际剩余空间；不回收其他会话的端口、工作区或构建产物。
