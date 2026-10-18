# App Store 准备与恢复流程

以下命令从 `mobileapp/` 运行。最后预览和 Submit 由操作者完成；本流程不会调用提交脚本。
[English](preparation.md)

## 1. 在耗时操作前核对输入

确定正式发行版与准确源码，汇总从**商店当前上架版本**以来的用户可见变化。
先审中英文文案，以及 `shot-captions.json`、`creative-captions.json` 的页面选择和标题。
在当前进程配置已有的 `ASC_KEY_ID`、`ASC_ISSUER_ID`、`ASC_KEY_PATH`，确认私钥文件存在。
不要依赖另一台机器的 shell 配置或上一轮任务的临时目录。缺少配置时，脚本在访问 API 前报错。
密钥、profile 和带鉴权的下载链接不能进入 Git 或公开附件。

## 2. 上传前构建并验证商店包

正常路径是 `bundle exec fastlane release`：保留 `build/GtmuxMobile.xcarchive`，
导出 IPA，并在上传**之前**运行 `scripts/store_package.py`。
主 App 和两个扩展的版本、build、签名、商店 profile 及归档符号都必须通过核验。
在私有副本上移除签名后比较可执行内容；资源和 Info 字段保持一致，仅允许所选 build 号变化。
原始分发 IPA 的 DER 权限保持一致，仅允许商店分发增加 `beta-reports-active=true`。
开发签名的 Xcode 归档另允许正常导出转换：关闭调试权限、将已启用的 APNs 切为 production，其余权限保持一致。
见 [Apple 分发权限核验](https://developer.apple.com/library/archive/qa/qa1798/_index.html)。
新增钥匙串或 App 分组、调试权限、APNs 环境变化都会阻止上传。

磁盘不足而采用备用路径时，从已验证的原始发行版 IPA 开始，按
[重签指引](reuse-ipa.zh.md)处理，再调用带校验的上传命令：

```sh
bundle exec fastlane upload ipa:/private/path/store.ipa \
  baseline:/private/path/original.ipa version:X.Y.Z build:N
```

校验成功后才生成 `build/store-package.json`。将回执与对应 IPA 一起保存；
最终状态检查会重新比较包的 SHA-256 和候选版本。原始 IPA 不能补回缺失的归档符号，
状态文件会保留这项限制。签名通过或 Apple 处理为 VALID 都不能证明配对保留或真机验收完成。

将完整归档和 dSYM 加密备份到其他机器；符号 UUID 必须与包匹配，见
[Apple 指引](https://developer.apple.com/documentation/xcode/adding-identifiable-symbol-names-to-a-crash-report)。
已有云端测试分发的加密恢复附件现在包含归档的 Products、Info 和 dSYM，
排除嵌入 profile、签名配置与符号链接，仍使用操作者已有的接收密钥加密。
在附件 **14 天**到期前下载、解密并核实。恢复的是经过筛选的归档，再导出时需另行提供已授权的 profile。
这项改动没有配置新的云端商店上传流程。

## 3. 复用云端截图进度

见 [Actions 截图](assets-ci.zh.md)。模拟器构建、已完成的设备/语言组、排版成品分别保存。
后续阶段失败不必重新构建；排版使用当前已审核的脚本与标题，清单保留已发布 App、
widget 源码和原始采集的来源记录。

仍需逐张按原尺寸审图。采集会等待已知演示内容出现，不能代替所有布局、VoiceOver 或真实推送验收。

## 4. 放置素材并独立读回

按[上架手册](submit.md)完成文字上传和准确 build 关联，再运行：

```sh
mkdir -p build
bundle exec ruby scripts/asc-preparation.rb assets \
  --version X.Y.Z --build N --ipa /private/path/store.ipa \
  --package-receipt build/store-package.json \
  --state build/store-state.json --journal build/store-progress.jsonl
```

`assets` 复用按内容识别的图片，放置截图及页头/搜索素材，随后独立核对：已关联的 VALID build、
版本文案、四组截图和四个创意图位置，并比较回执与真实 IPA。
缺图、仅靠旧版文件名/大小匹配、位置尚未 ACTIVE，都不能算就绪。
`status` 执行相同读回，不修改素材位置：

```sh
bundle exec ruby scripts/asc-preparation.rb status \
  --version X.Y.Z --build N --ipa /private/path/store.ipa \
  --package-receipt build/store-package.json --state build/store-state.json
```

每次检查都会替换私有状态，网络失败不会留下仍显示就绪的旧结论。
只有各项通过才显示 `ready_for_user_review`。该检查覆盖包/build、版本文案、截图和创意图；
协议、年龄分级/隐私表单、审核联系人、应用名称/副标题和 ASC 网页预览仍按上架手册核对。
不要只读取旧回执判断当前是否可以提交，应重新运行 `status`。

## 处理进度与恢复

进度即时输出，显示图片 ID 和处理状态；可选日志只允许当前用户读写（0600），不记录 API token 或签名上传地址。
长时间等待时，会在到期前用同一密钥刷新内存中的请求 JWT，不新增凭据或权限范围。
每张图默认最多等待 900 秒，可通过 `ASC_PROCESSING_WAIT_SECONDS=1..1800` 调整。
等待到期返回 **75（仍在处理）**，用相同命令和输入重试即可继续等待并复用已上传图片。
同组的新图全部可用之前，保留原位置。没有提交完整数据的预约可以重建；仍在处理的图片不会因为超时而重复上传。
未知状态或处理失败都不能视为就绪；恢复操作不会自动提交审核。

## 辅助功能信息

按[辅助功能核验流程](accessibility.zh.md)检查当前已上架版本。Accessibility URL 可留空，按设备类别仅发布已验证的能力；最终 App Submit 仍由操作者执行。
