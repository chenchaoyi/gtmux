# 复用已验证的发行版成品准备商店包

常规流程是 `fastlane release`：归档、导出 App Store IPA 并上传。
应保留归档及匹配的 dSYM。本机磁盘不足时，维护者可以复用已独立验证、
来自准确发行版源码的 Release IPA。Ad Hoc IPA 不能直接上传商店，
必须先改为 App Store 签名。重新签名不能从 IPA 补回归档或缺失的 dSYM。

在私有副本上操作。复用有效的 Apple Distribution 身份，以及主 App、
Widget 和通知扩展各自的 App Store 描述文件。选择 ASC 中尚未使用的下一构建号，
三个 bundle 一起更新，保留公开版本号和 bundle ID。
现有身份有效且私钥可用时，无需新建证书。

使用 [fastlane 的 `resign`](https://docs.fastlane.tools/actions/resign/)，
并设置 `use_app_entitlements: true`。默认模式直接复制描述文件里的权限，
会带入通用 Keychain 分组，不能视为与 Xcode 正常导出等价。

上传前独立核对：

- 三个 bundle 签名有效、版本和构建号一致；描述文件未过期，属于 App Store 类型，
  不带注册设备清单或企业分发标记。
- 读取实际签名中的 DER 权限，与原始成品比较。本应用仅允许新增
  `beta-reports-active`，Keychain/App Group、应用与团队标识及其他运行权限均应保持原值。
  `get-task-allow` 保持 false，主 App 的 APNs 环境保持 production。
- JavaScript bundle 哈希、原生可执行文件 UUID 与原始成品一致；支持 iPhone 和 iPad，
  iPad 不要求全屏。

全部通过后，才使用已有 `fastlane upload` 上传。Apple 返回 `VALID` 不代表权限连续性或
原有配对已经验收。用 `asc-attach-build.rb` 关联准确的构建，再读回确认。
被替代的上传保持未选中，不要误提交。其余文字和素材按[上架流程](submit.md)处理；
用户要求自行 Submit 时，最终提交步骤留给用户。

## 上传前强制校验

上传命令现在必须提供原始 IPA/归档及明确的候选版本：

```sh
bundle exec fastlane upload ipa:/private/path/store.ipa \
  baseline:/private/path/original.ipa version:X.Y.Z build:N
```

命令在网络上传前运行 `scripts/store_package.py`，保存 `build/store-package.json`。
正常的 `fastlane release` 使用明确保留的 `build/GtmuxMobile.xcarchive` 作基线，
并要求原生 dSYM 匹配。见[准备与独立读回](preparation.zh.md)。
原始 IPA 备用路径缺失的符号仍需明确说明，不能视为已补齐。
